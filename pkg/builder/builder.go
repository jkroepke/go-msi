// Package builder turns a validated config.Config into an .msi package: it
// derives every table row, numbers the sequences, packs the cabinet, writes
// the database through msidb and verifies the result by reading it back.
//
// The rules come from docs/spec/MSISPEC.md and the Windows results in
// docs/spec/MSISPEC-RESULTS.md; docs/design/architecture.md describes the
// pipeline.
package builder

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/jkroepke/go-msi/internal/schema"
	"github.com/jkroepke/go-msi/internal/sequence"
	"github.com/jkroepke/go-msi/pkg/config"
	"github.com/jkroepke/go-msi/pkg/msidb"
)

// Options control a build.
type Options struct {
	BuildTime time.Time
	ToolName  string
	// Output is the path the package is written to. The cabinet and the
	// package are written to temporary files beside it and the payload
	// files stream through, so the build needs memory for the tables only,
	// not for the files. The package replaces a file at Output only once it
	// is verified; a failed build leaves nothing behind. When Output is
	// empty the package is built in memory and returned in Result.Package.
	// After a build to a file, Result.Database lists the cabinet by name
	// and size only and cannot be marshalled again.
	Output     string
	CFBVersion int
	SkipVerify bool
}

// Result is a finished build.
type Result struct {
	// Package holds the bytes when Options.Output was empty.
	Package     []byte
	Database    *msidb.Database
	Diagnostics config.Diagnostics
	ProductCode string
	PackageCode string
	// Files lists every payload file in File.Sequence order.
	Files []File
	// Size is the package length in bytes.
	Size int64
}

// File describes one payload file of a finished build.
type File struct {
	// Key is the File table primary key.
	Key string
	// Name is the long file name as installed.
	Name string
	// Source is the source path as written in the config.
	Source string
	// Component is the ID of the component that owns the file.
	Component string
	// Version is the File.Version value, or "" for an unversioned file.
	Version string
	// Language is the File.Language value, or "" when none is recorded.
	Language string
	// Size is the file size in bytes.
	Size int64
	// Sequence is the File.Sequence value, which orders the files in the
	// cabinet.
	Sequence int
}

// build holds the state of one build.
type build struct { //nolint:govet // grouped by build step
	secure     map[string]bool
	cfg        *config.Config
	numbered   map[string][]sequence.Numbered
	tables     map[string]*msidb.Table
	ensured    map[string]bool // tables written even without rows (spec 11.10)
	dirs       map[string]*dirInfo
	schedules  map[string][]sequence.Action
	components map[string]*componentInfo
	hidden     map[string]bool
	// shortcutCarrier maps a component to the generated component that
	// holds the shortcuts to its files.
	shortcutCarrier map[string]string
	// shortcutNames are the short names taken per shortcut directory.
	shortcutNames map[string]map[string]bool
	fileByKey     map[string]*fileInfo
	properties    map[string]string
	upgradeCode   string
	productCode   string
	packageCode   string
	files         []*fileInfo
	actions       []*actionInfo
	dirOrder      []*dirInfo
	diags         config.Diagnostics
	opts          Options
	version       config.Version
	is64          bool
	// summaryWarned holds the config paths already reported as made to fit
	// the summary code page.
	summaryWarned map[string]bool
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
	// open reads the payload again, for the cabinet.
	open     func() (io.ReadCloser, error)
	FileName string
	LongName string
	Key      string
	Attrs    int
	Sequence int
	// Size is the file length; digest its SHA-256.
	Size   int64
	digest [sha256.Size]byte
	isPE   bool
}

type actionInfo struct {
	source any
	target any
	name   string
	typ    int
	// fromUI marks an action that a control runs with DoAction.
	fromUI bool
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
		cfg:             cfg,
		opts:            opts,
		diags:           diags,
		tables:          make(map[string]*msidb.Table),
		ensured:         make(map[string]bool),
		dirs:            make(map[string]*dirInfo),
		components:      make(map[string]*componentInfo),
		fileByKey:       make(map[string]*fileInfo),
		properties:      make(map[string]string),
		secure:          make(map[string]bool),
		hidden:          make(map[string]bool),
		shortcutCarrier: make(map[string]string),
		shortcutNames:   make(map[string]map[string]bool),
		schedules:       make(map[string][]sequence.Action),
		numbered:        make(map[string][]sequence.Numbered),
		is64:            cfg.Package.Platform.Is64Bit(),
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
		b.shortcuts,
		b.features,
		b.writeProperties,
		b.upgrade,
		b.launchConditions,
		b.rememberInstallDir,
		b.customActions,
		b.firewall,
		b.permissions,
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

	database, err := b.finish(res)
	if err != nil {
		return err
	}

	res.Database = database
	res.ProductCode = b.productCode
	res.PackageCode = b.packageCode
	res.Files = make([]File, 0, len(b.files))

	for _, file := range b.files {
		res.Files = append(res.Files, File{
			Key:       file.Key,
			Name:      file.LongName,
			Source:    file.cfg.Source,
			Component: file.Component.cfg.ID,
			Version:   stringCell(file.Version),
			Language:  stringCell(file.Language),
			Size:      file.Size,
			Sequence:  file.Sequence,
		})
	}

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

// ensure creates the named standard table and keeps it in the package even
// when it gets no rows; other empty tables are left out.
func (b *build) ensure(name string) {
	b.ensured[name] = true
	b.table(name)
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

// stringCell returns a string cell, or "" for a NULL cell.
func stringCell(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	return ""
}
