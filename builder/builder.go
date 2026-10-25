// Package builder turns a validated config.Config into an .msi package: it
// derives every table row, numbers the sequences, packs the cabinet, writes
// the database through msidb and verifies the result by reading it back.
//
// The rules come from docs/spec/MSISPEC.md and the Windows results in
// docs/spec/MSISPEC-RESULTS.md; docs/design/architecture.md describes the
// pipeline.
package builder

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/msidb"
	"github.com/jkroepke/go-msi/schema"
	"github.com/jkroepke/go-msi/sequence"
)

// Options control a build.
type Options struct {
	BuildTime  time.Time
	ToolName   string
	CFBVersion int
	SkipVerify bool
}

// Result is a finished build.
type Result struct {
	Package     []byte
	Database    *msidb.Database
	Diagnostics config.Diagnostics
	ProductCode string
	PackageCode string
	// Files lists every payload file in File.Sequence order.
	Files []*fileInfo
}

// build holds the state of one build.
type build struct {
	secure      map[string]bool
	cfg         *config.Config
	numbered    map[string][]sequence.Numbered
	tables      map[string]*msidb.Table
	dirs        map[string]*dirInfo
	schedules   map[string][]sequence.Action
	components  map[string]*componentInfo
	hidden      map[string]bool
	fileByKey   map[string]*fileInfo
	properties  map[string]string
	upgradeCode string
	productCode string
	packageCode string
	files       []*fileInfo
	actions     []*actionInfo
	dirOrder    []*dirInfo
	diags       config.Diagnostics
	opts        Options
	version     config.Version
	is64        bool
}

type dirInfo struct {
	id        string
	parent    string
	long      string // long name, "" for standard folders and TARGETDIR
	canonical string // canonical path for GUIDs and sorting
	standard  bool
	used      bool
}

type componentInfo struct {
	guid       any
	keyPath    any
	cfg        *config.Component
	dir        *dirInfo
	keyFile    *fileInfo
	index      int
	attributes int
	is64       bool
}

// fileInfo is a payload file.
type fileInfo struct {
	Version   any
	Language  any
	Component *componentInfo
	cfg       *config.File
	Hash      *[4]int32
	path      string
	FileName  string
	LongName  string
	Key       string
	Data      []byte
	Attrs     int
	Sequence  int
	isPE      bool
}

type actionInfo struct {
	source any
	target any
	name   string
	typ    int
}

// Build builds the package described by cfg. Validation diagnostics of the
// config are included; the returned error lists every error-level finding.
func Build(cfg *config.Config, opts Options) (*Result, error) {
	diags := config.Validate(cfg)
	if err := diags.Err(); err != nil {
		return &Result{Diagnostics: diags}, err //nolint:wrapcheck // the diagnostics carry their own paths
	}

	if opts.BuildTime.IsZero() {
		opts.BuildTime = defaultBuildTime()
	}

	opts.BuildTime = opts.BuildTime.UTC().Truncate(time.Second)

	if opts.ToolName == "" {
		opts.ToolName = "go-msi"
	}

	bld := &build{
		cfg:        cfg,
		opts:       opts,
		diags:      diags,
		tables:     make(map[string]*msidb.Table),
		dirs:       make(map[string]*dirInfo),
		components: make(map[string]*componentInfo),
		fileByKey:  make(map[string]*fileInfo),
		properties: make(map[string]string),
		secure:     make(map[string]bool),
		hidden:     make(map[string]bool),
		schedules:  make(map[string][]sequence.Action),
		numbered:   make(map[string][]sequence.Numbered),
		is64:       cfg.Package.Platform.Is64Bit(),
	}

	res := &Result{}

	err := bld.run(res)
	res.Diagnostics = bld.diags

	if err != nil {
		return res, err
	}

	if err := bld.diags.Err(); err != nil {
		return res, err //nolint:wrapcheck // the diagnostics carry their own paths
	}

	return res, nil
}

func (b *build) run(res *Result) error {
	steps := []func() error{
		b.identity,
		b.directories,
		b.writeComponents,
		b.inputs,
		b.sequenceFiles,
		b.features,
		b.writeProperties,
		b.upgrade,
		b.launchConditions,
		b.customActions,
		b.firewall,
		b.streams,
		b.customTables,
		b.ui,
		b.sequences,
	}

	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}

		if err := b.diags.Err(); err != nil {
			return err //nolint:wrapcheck // the diagnostics carry their own paths
		}
	}

	pkg, database, err := b.finish()
	if err != nil {
		return err
	}

	res.Package = pkg
	res.Database = database
	res.ProductCode = b.productCode
	res.PackageCode = b.packageCode
	res.Files = b.files

	return nil
}

func defaultBuildTime() time.Time {
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(n, 0).UTC()
		}
	}

	return time.Now().UTC()
}

// table returns the named standard table, creating it empty on first use.
func (b *build) table(name string) *msidb.Table {
	tbl, ok := b.tables[name]
	if !ok {
		tbl = schema.New(name)
		b.tables[name] = tbl
	}

	return tbl
}

func (b *build) hasRows(name string) bool {
	t, ok := b.tables[name]

	return ok && len(t.Rows) > 0
}

func (b *build) errorf(path, format string, args ...any) {
	b.diags = append(b.diags, config.Diagnostic{Severity: config.SeverityError, Path: path, Message: fmt.Sprintf(format, args...)})
}

func (b *build) warnf(path, format string, args ...any) {
	b.diags = append(b.diags, config.Diagnostic{Severity: config.SeverityWarning, Path: path, Message: fmt.Sprintf(format, args...)})
}

// nullable returns nil for "" so that optional strings become NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}

	return s
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}
