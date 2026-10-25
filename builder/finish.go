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

	"github.com/jkroepke/go-msi/cab"
	"github.com/jkroepke/go-msi/cfb"
	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/msidb"
	"github.com/jkroepke/go-msi/schema"
	"github.com/jkroepke/go-msi/summary"
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

	return os.ReadFile(path)
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
func (b *build) customTables() error {
	for i, ct := range b.cfg.Tables {
		p := fmt.Sprintf("tables[%d]", i)

		if _, exists := b.tables[ct.Name]; exists {
			b.errorf(p+".name", "table %s is written by the builder", ct.Name)

			continue
		}

		t := &msidb.Table{Name: ct.Name}

		for _, c := range ct.Columns {
			typ, err := msidb.ParseType(c.Type)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}

			typ.Key = c.Key
			t.Columns = append(t.Columns, msidb.Column{Name: c.Name, Type: typ})
		}

		for j, row := range ct.Rows {
			cells := make([]any, len(row))

			for k, v := range row {
				cells[k] = v

				if t.Columns[k].Type.Kind != msidb.KindBinary {
					continue
				}

				if path, ok := v.(string); ok && path != "" {
					data, err := b.readInput(path)
					if err != nil {
						b.errorf(fmt.Sprintf("%s.rows[%d][%d]", p, j, k), "%v", err)

						continue
					}

					cells[k] = data
				}
			}

			t.Rows = append(t.Rows, cells)
		}

		if err := t.Validate(); err != nil {
			b.errorf(p, "%v", err)

			continue
		}

		b.tables[ct.Name] = t
	}

	return nil
}

// finish assembles the database, packs the cabinet, derives the package code
// and verifies the package by reading it back.
func (b *build) finish() ([]byte, *msidb.Database, error) {
	b.finishProperties()

	db := &msidb.Database{Codepage: b.cfg.Package.Codepage}

	for _, name := range sortedTableNames(b.tables) {
		t := b.tables[name]

		if err := t.Validate(); err != nil {
			return nil, nil, fmt.Errorf("table %s: %w", name, err)
		}

		db.Tables = append(db.Tables, t)
	}

	db.Tables = append(db.Tables, schema.Validation(db.Tables))

	slices.SortFunc(db.Tables, func(a, b *msidb.Table) int { return strings.Compare(a.Name, b.Name) })

	if len(b.files) > 0 {
		data, err := b.cabinet()
		if err != nil {
			return nil, nil, err
		}

		db.Streams = append(db.Streams, cfb.Stream{Name: b.cfg.Package.Cabinet, Data: data})
	}

	auto := b.packageCode == ""
	if auto {
		b.packageCode = placeholderCode
	}

	db.Summary = b.summaryProperties()

	pkg, err := db.Marshal(msidb.Options{CFBVersion: b.opts.CFBVersion})
	if err != nil {
		return nil, nil, err
	}

	if auto {
		sum := sha256.Sum256(pkg)
		b.packageCode = uuid5(namespacePackage, hex.EncodeToString(sum[:]))
		db.Summary = b.summaryProperties()

		pkg, err = db.Marshal(msidb.Options{CFBVersion: b.opts.CFBVersion})
		if err != nil {
			return nil, nil, err
		}
	}

	if !b.opts.SkipVerify {
		if err := b.verify(pkg, db); err != nil {
			return nil, nil, fmt.Errorf("verification of the built package failed: %w", err)
		}
	}

	return pkg, db, nil
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
	p := b.cfg.Product
	pkg := b.cfg.Package

	pageCount, _ := config.PageCount(pkg.InstallerVersion)

	subject := p.Description
	if subject == "" {
		subject = p.Name
	}

	comments := p.Comments
	if comments == "" {
		comments = "This installer database contains the logic and data required to install " + p.Name + "."
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
		{ID: summary.PIDTemplate, Value: fmt.Sprintf("%s;%d", pkg.Platform, *p.Language)},
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
func (b *build) verify(pkg []byte, want *msidb.Database) error {
	got, err := msidb.Unmarshal(pkg)
	if err != nil {
		return err
	}

	if len(got.Problems) > 0 {
		return fmt.Errorf("read-back problems: %s", strings.Join(got.Problems, "; "))
	}

	if len(got.Tables) != len(want.Tables) {
		return fmt.Errorf("read back %d tables, wrote %d", len(got.Tables), len(want.Tables))
	}

	for _, wt := range want.Tables {
		gt := got.Table(wt.Name)
		if gt == nil {
			return fmt.Errorf("table %s is missing", wt.Name)
		}

		if len(gt.Rows) != len(wt.Rows) {
			return fmt.Errorf("table %s: read back %d rows, wrote %d", wt.Name, len(gt.Rows), len(wt.Rows))
		}

		if len(gt.Columns) != len(wt.Columns) {
			return fmt.Errorf("table %s: read back %d columns, wrote %d", wt.Name, len(gt.Columns), len(wt.Columns))
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

	for i, fi := range b.files {
		if entries[i].Name != fi.Key {
			return fmt.Errorf("cabinet entry %d is %q, want %q", i, entries[i].Name, fi.Key)
		}

		if !bytes.Equal(entries[i].Data, fi.Data) {
			return fmt.Errorf("cabinet entry %s differs from its source", fi.Key)
		}
	}

	return nil
}
