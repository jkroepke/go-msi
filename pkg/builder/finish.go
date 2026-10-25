package builder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jkroepke/go-msi/internal/schema"
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
func (b *build) finish() ([]byte, *msidb.Database, error) { //nolint:cyclop // one pass over the format; splitting would scatter it
	// Written last: custom actions, firewall rules and service recovery add
	// standard folders such as System64Folder after the tree is walked.
	b.writeDirectoryTable()
	b.finishProperties()

	database := &msidb.Database{Codepage: b.cfg.Package.Codepage}

	for _, name := range sortedTableNames(b.tables) {
		tbl := b.tables[name]

		if err := tbl.Validate(); err != nil {
			return nil, nil, fmt.Errorf("table %s: %w", name, err)
		}

		database.Tables = append(database.Tables, tbl)
	}

	database.Tables = append(database.Tables, schema.Validation(database.Tables))

	slices.SortFunc(database.Tables, func(a, b *msidb.Table) int { return strings.Compare(a.Name, b.Name) })

	if len(b.files) > 0 {
		data, err := b.cabinet()
		if err != nil {
			return nil, nil, err
		}

		database.Streams = append(database.Streams, cfb.Stream{Name: b.cfg.Package.Cabinet, Data: data})
	}

	auto := b.packageCode == ""
	if auto {
		b.packageCode = placeholderCode
	}

	database.Summary = b.summaryProperties()

	pkg, err := database.Marshal(msidb.Options{CFBVersion: b.opts.CFBVersion})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal database: %w", err)
	}

	if auto {
		sum := sha256.Sum256(pkg)
		b.packageCode = uuid5(namespacePackage, hex.EncodeToString(sum[:]))
		database.Summary = b.summaryProperties()

		pkg, err = database.Marshal(msidb.Options{CFBVersion: b.opts.CFBVersion})
		if err != nil {
			return nil, nil, fmt.Errorf("marshal database: %w", err)
		}
	}

	if !b.opts.SkipVerify {
		if err := b.verify(pkg, database); err != nil {
			return nil, nil, fmt.Errorf("verification of the built package failed: %w", err)
		}
	}

	return pkg, database, nil
}

func sortedTableNames(m map[string]*msidb.Table) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}

// cabinet packs every payload file in sequence order (section 11.6).
func (b *build) cabinet() ([]byte, error) {
	files := make([]cab.File, 0, len(b.files))

	for _, fi := range b.files {
		files = append(files, cab.File{Name: fi.Key, Data: fi.Data, Time: b.opts.BuildTime, Attributes: cab.AttrArchive})
	}

	comp := cab.MSZIP
	if b.cfg.Package.Compression == "none" {
		comp = cab.None
	}

	data, err := cab.Write(files, cab.Options{Compression: comp})
	if err != nil {
		return nil, fmt.Errorf("cabinet: %w", err)
	}

	return data, nil
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
// written: tables, strings, summary and cabinet contents.
func (b *build) verify(pkg []byte, want *msidb.Database) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	got, err := msidb.Unmarshal(pkg)
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

	data, ok := got.Stream(b.cfg.Package.Cabinet)
	if !ok {
		return fmt.Errorf("cabinet stream %q is missing", b.cfg.Package.Cabinet)
	}

	entries, err := cab.Read(data)
	if err != nil {
		return fmt.Errorf("cabinet: %w", err)
	}

	if len(entries) != len(b.files) {
		return fmt.Errorf("cabinet holds %d files, want %d", len(entries), len(b.files))
	}

	for i, file := range b.files {
		if entries[i].Name != file.Key {
			return fmt.Errorf("cabinet entry %d is %q, want %q", i, entries[i].Name, file.Key)
		}

		if !bytes.Equal(entries[i].Data, file.Data) {
			return fmt.Errorf("cabinet entry %s differs from its source", file.Key)
		}
	}

	return nil
}
