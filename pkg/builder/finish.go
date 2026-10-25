package builder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jkroepke/go-msi/internal/schema"
	"github.com/jkroepke/go-msi/internal/spool"
	"github.com/jkroepke/go-msi/pkg/cab"
	"github.com/jkroepke/go-msi/pkg/cfb"
	"github.com/jkroepke/go-msi/pkg/config"
	"github.com/jkroepke/go-msi/pkg/msidb"
	"github.com/jkroepke/go-msi/pkg/summary"
)

// placeholderCode stands in for an automatic package code while the
// database is marshalled the first time; it has the length of a real GUID so
// that replacing it changes no offsets.
const placeholderCode = "{00000000-0000-0000-0000-000000000000}"

// readInput reads a source path relative to the YAML file.
func (b *build) readInput(path string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.cfg.BaseDir, filepath.FromSlash(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return data, nil
}

// streams fills the Icon and Binary tables from the icons and binaries
// sections.
func (b *build) streams() error {
	for _, section := range []struct {
		src   map[string]string
		table string
	}{{table: "Icon", src: b.cfg.Icons}, {table: "Binary", src: b.cfg.Binaries}} {
		for _, name := range sortedStringKeys(section.src) {
			data, err := b.readInput(section.src[name])
			if err != nil {
				b.errorf(strings.ToLower(section.table)+"s."+name, "%v", err)

				continue
			}

			if len(data) == 0 {
				b.errorf(strings.ToLower(section.table)+"s."+name, "%s is empty", section.src[name])

				continue
			}

			b.table(section.table).Add(name, data)
		}
	}

	return nil
}

// customTables copies the tables section into the database. A string in a
// binary column names a file to embed.
func (b *build) customTables() error { //nolint:cyclop // one pass over the format; splitting would scatter it
	for i, custom := range b.cfg.Tables {
		tablePath := fmt.Sprintf("tables[%d]", i)

		if _, exists := b.tables[custom.Name]; exists {
			b.errorf(tablePath+".name", "table %s is written by the builder", custom.Name)

			continue
		}

		if custom.Name == "LockPermissions" && b.hasRows(lockTableName) {
			b.errorf(tablePath+".name", "LockPermissions and MsiLockPermissionsEx cannot both exist (ICE104, error 1941); use permissions")

			continue
		}

		tbl := &msidb.Table{Name: custom.Name}

		for _, col := range custom.Columns {
			typ, err := msidb.ParseType(col.Type)
			if err != nil {
				return fmt.Errorf("%s: %w", tablePath, err)
			}

			typ.Key = col.Key
			tbl.Columns = append(tbl.Columns, msidb.Column{Name: col.Name, Type: typ})
		}

		for rowIndex, row := range custom.Rows {
			cells := make([]any, len(row))

			for colIndex, value := range row {
				cells[colIndex] = value

				if tbl.Columns[colIndex].Type.Kind != msidb.KindBinary {
					continue
				}

				if path, ok := value.(string); ok && path != "" {
					data, err := b.readInput(path)
					if err != nil {
						b.errorf(fmt.Sprintf("%s.rows[%d][%d]", tablePath, rowIndex, colIndex), "%v", err)

						continue
					}

					cells[colIndex] = data
				}
			}

			tbl.Rows = append(tbl.Rows, cells)
		}

		if err := tbl.Validate(); err != nil {
			b.errorf(tablePath, "%v", err)

			continue
		}

		b.tables[custom.Name] = tbl
	}

	return nil
}

// finish assembles the database, packs the cabinet, derives the package code
// and verifies the package by reading it back.
func (b *build) finish(res *Result) (*msidb.Database, error) { //nolint:cyclop // one pass over the format; splitting would scatter it
	// Written last: custom actions, firewall rules and service recovery add
	// standard folders such as System64Folder after the tree is walked.
	b.writeDirectoryTable()
	b.finishProperties()

	database := &msidb.Database{Codepage: b.cfg.Package.Codepage}

	for _, name := range sortedTableNames(b.tables) {
		tbl := b.tables[name]

		if err := tbl.Validate(); err != nil {
			return nil, fmt.Errorf("table %s: %w", name, err)
		}

		database.Tables = append(database.Tables, tbl)
	}

	database.Tables = append(database.Tables, schema.Validation(database.Tables))

	slices.SortFunc(database.Tables, func(a, b *msidb.Table) int { return strings.Compare(a.Name, b.Name) })

	// A symlink to an existing file at Output is written through: the
	// temporary files go next to its target so that the final rename stays
	// on one file system. A dangling symlink is replaced by the package.
	output := b.opts.Output
	if resolved, err := filepath.EvalSymlinks(output); err == nil {
		output = resolved
	}

	target, closeTarget, err := b.newTarget("package", output)
	if err != nil {
		return nil, err
	}

	defer closeTarget()

	if len(b.files) > 0 {
		cabSpool, closeSpool, err := b.newTarget("cabinet", output)
		if err != nil {
			return nil, err
		}

		defer closeSpool()

		source, err := b.cabinet(cabSpool)
		if err != nil {
			return nil, err
		}

		database.Sources = append(database.Sources, source)
	}

	auto := b.packageCode == ""
	if auto {
		b.packageCode = placeholderCode
	}

	database.Summary = b.summaryProperties()

	layout, err := database.MarshalTo(target, msidb.Options{CFBVersion: b.opts.CFBVersion})
	if err != nil {
		return nil, fmt.Errorf("marshal database: %w", err)
	}

	if auto {
		if err := b.patchPackageCode(target, layout, database); err != nil {
			return nil, err
		}
	}

	if !b.opts.SkipVerify {
		if err := b.verify(target, layout.Size, database); err != nil {
			return nil, fmt.Errorf("verification of the built package failed: %w", err)
		}
	}

	res.Size = layout.Size

	switch out := target.(type) {
	case *spool.Buffer:
		res.Package = out.Bytes()
	case *os.File:
		if err := commitOutput(out, output); err != nil {
			return nil, err
		}

		// The cabinet spool is removed when finish returns; the returned
		// database keeps the cabinet's name and size only.
		for i := range database.Sources {
			database.Sources[i].Open = nil
		}
	}

	return database, nil
}

// patchPackageCode derives the automatic package code from a digest of the
// package as written with the placeholder and writes the summary stream
// again in place. The real code has the placeholder's length, so the stream
// and the layout do not change.
func (b *build) patchPackageCode(target spool.Target, layout *cfb.Layout, database *msidb.Database) error {
	written, err := database.MarshalSummary()
	if err != nil {
		return fmt.Errorf("marshal database: %w", err)
	}

	hasher := sha256.New()

	if _, err := io.Copy(hasher, io.NewSectionReader(target, 0, layout.Size)); err != nil {
		return fmt.Errorf("digest of the package: %w", err)
	}

	b.packageCode = uuid5(namespacePackage, hex.EncodeToString(hasher.Sum(nil)))
	database.Summary = b.summaryProperties()

	data, err := database.MarshalSummary()
	if err != nil {
		return fmt.Errorf("marshal database: %w", err)
	}

	offset, ok := layout.Offsets[summary.StreamName]
	if !ok {
		return errors.New("marshal database: the summary stream has no offset")
	}

	if len(data) != len(written) {
		return fmt.Errorf("marshal database: the summary stream changed from %d to %d bytes with the package code", len(written), len(data))
	}

	if _, err := target.WriteAt(data, offset); err != nil {
		return fmt.Errorf("write the summary stream: %w", err)
	}

	return nil
}

// newTarget returns where a build product is written: memory when the
// package stays in memory, otherwise a temporary file next to the output
// (the package itself is written to the output path). output is the
// resolved output path; the temporary files are created next to it. The
// returned function
// closes the file, and removes it unless it is the finished package.
func (b *build) newTarget(kind, output string) (spool.Target, func(), error) {
	if b.opts.Output == "" {
		return &spool.Buffer{}, func() {}, nil
	}

	file, err := os.CreateTemp(filepath.Dir(output), ".go-msi-*."+kind)
	if err != nil {
		return nil, nil, fmt.Errorf("create a temporary %s file: %w", kind, err)
	}

	// After commitOutput the package has moved and there is nothing to
	// remove; a failed build leaves no file behind.
	return file, func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}, nil
}

// commitOutput moves the finished package from its temporary file to path
// (a symlink already resolved), so that an existing package is replaced only
// by a verified one. An existing file keeps its mode.
func commitOutput(file *os.File, path string) error {
	mode := os.FileMode(0o644)

	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	if err := file.Chmod(mode); err != nil {
		return fmt.Errorf("set the mode of %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", file.Name(), err)
	}

	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("move the package to %s: %w", path, err)
	}

	return nil
}

func sortedTableNames(m map[string]*msidb.Table) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}

// cabinet packs every payload file in sequence order (section 11.6) into
// spool, streaming each file from disk, and returns the cabinet as a source
// for the package.
func (b *build) cabinet(target spool.Target) (cfb.Source, error) {
	entries := make([]cab.Entry, 0, len(b.files))

	for _, fi := range b.files {
		entries = append(entries, cab.Entry{
			Name:       fi.Key,
			Size:       fi.Size,
			Time:       b.opts.BuildTime,
			Attributes: cab.AttrArchive,
			Open:       func() (io.ReadCloser, error) { return os.Open(fi.path) },
		})
	}

	comp := cab.MSZIP
	if b.cfg.Package.Compression == "none" {
		comp = cab.None
	}

	size, err := cab.WriteTo(target, entries, cab.Options{Compression: comp})
	if err != nil {
		return cfb.Source{}, fmt.Errorf("cabinet: %w", err)
	}

	return cfb.Source{
		Name: b.cfg.Package.Cabinet,
		Size: size,
		Open: func() (io.ReadCloser, error) { return io.NopCloser(io.NewSectionReader(target, 0, size)), nil },
	}, nil
}

// summaryProperties builds the summary information (sections 4 and 11.1).
func (b *build) summaryProperties() []summary.Property {
	product := b.cfg.Product
	pkg := b.cfg.Package

	pageCount, _ := config.PageCount(pkg.InstallerVersion)

	subject := product.Description
	if subject == "" {
		subject = product.Name
	}

	comments := product.Comments
	if comments == "" {
		comments = "This installer database contains the logic and data required to install " + product.Name + "."
	}

	words := summary.WordCompressed
	if pkg.LongFileNames != nil && !*pkg.LongFileNames {
		words |= summary.WordShortNames
	}

	return []summary.Property{
		{ID: summary.PIDCodepage, Value: config.SummaryCodepage(pkg.Codepage)},
		{ID: summary.PIDTitle, Value: pkg.Summary.Title},
		{ID: summary.PIDSubject, Value: subject},
		{ID: summary.PIDAuthor, Value: pkg.Summary.Author},
		{ID: summary.PIDKeywords, Value: pkg.Summary.Keywords},
		{ID: summary.PIDComments, Value: comments},
		{ID: summary.PIDTemplate, Value: fmt.Sprintf("%s;%d", pkg.Platform, *product.Language)},
		{ID: summary.PIDRevision, Value: b.packageCode},
		{ID: summary.PIDCreated, Value: b.opts.BuildTime},
		{ID: summary.PIDLastSaved, Value: b.opts.BuildTime},
		{ID: summary.PIDPageCount, Value: pageCount},
		{ID: summary.PIDWordCount, Value: words},
		{ID: summary.PIDAppName, Value: b.opts.ToolName},
		{ID: summary.PIDSecurity, Value: 2},
	}
}

// verify reads the finished package back and compares it with what was
// written: tables, strings, summary and cabinet contents. It reads through
// the file, so a large cabinet is never in memory.
func (b *build) verify(target spool.Target, size int64, want *msidb.Database) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	file, err := cfb.Open(target, size)
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}

	got, err := msidb.Decode(file)
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}

	if len(got.Problems) > 0 {
		return fmt.Errorf("read-back problems: %s", strings.Join(got.Problems, "; "))
	}

	if len(got.Tables) != len(want.Tables) {
		return fmt.Errorf("read back %d tables, wrote %d", len(got.Tables), len(want.Tables))
	}

	for _, wantTable := range want.Tables {
		gotTable := got.Table(wantTable.Name)
		if gotTable == nil {
			return fmt.Errorf("table %s is missing", wantTable.Name)
		}

		if len(gotTable.Rows) != len(wantTable.Rows) {
			return fmt.Errorf("table %s: read back %d rows, wrote %d", wantTable.Name, len(gotTable.Rows), len(wantTable.Rows))
		}

		if len(gotTable.Columns) != len(wantTable.Columns) {
			return fmt.Errorf("table %s: read back %d columns, wrote %d", wantTable.Name, len(gotTable.Columns), len(wantTable.Columns))
		}
	}

	if code, ok := summary.Get(got.Summary, summary.PIDRevision).(string); !ok || code != b.packageCode {
		return fmt.Errorf("summary package code %q, want %q", code, b.packageCode)
	}

	if len(b.files) == 0 {
		return nil
	}

	encoded, err := msidb.EncodeStreamName(b.cfg.Package.Cabinet, false)
	if err != nil {
		return fmt.Errorf("cabinet stream: %w", err)
	}

	stream, err := file.Reader(encoded)
	if err != nil {
		return fmt.Errorf("cabinet stream %q is missing: %w", b.cfg.Package.Cabinet, err)
	}

	index := 0

	err = cab.Walk(stream, stream.Size(), func(entry cab.File, data io.Reader) error {
		if index >= len(b.files) {
			return fmt.Errorf("cabinet holds more than %d files", len(b.files))
		}

		want := b.files[index]
		index++

		if entry.Name != want.Key {
			return fmt.Errorf("cabinet entry %d is %q, want %q", index-1, entry.Name, want.Key)
		}

		hasher := sha256.New()

		n, err := io.Copy(hasher, data)
		if err != nil {
			return fmt.Errorf("cabinet entry %s: %w", want.Key, err)
		}

		if n != want.Size || !bytes.Equal(hasher.Sum(nil), want.digest[:]) {
			return fmt.Errorf("cabinet entry %s differs from its source", want.Key)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("cabinet: %w", err)
	}

	if index != len(b.files) {
		return fmt.Errorf("cabinet holds %d files, want %d", index, len(b.files))
	}

	return nil
}
