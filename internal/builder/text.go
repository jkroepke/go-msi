package builder

import (
	"fmt"
	"strings"

	"github.com/jkroepke/msi-builder/internal/codepage"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/summary"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// checkText reports every string cell that the database code page cannot
// hold, once per column, so that the build fails with a diagnostic instead
// of a marshal error (section 3.2).
func (b *build) checkText(tables []*msidb.Table) {
	codePage := b.cfg.Package.Codepage

	for _, tbl := range tables {
		for col := range tbl.Columns {
			for _, row := range tbl.Rows {
				text, ok := row[col].(string)
				if !ok {
					continue
				}

				if _, err := codepage.Encode(text, codePage); err != nil {
					b.errorf("package.codepage", "%s.%s value %q cannot be represented in code page %d; "+
						"use 65001 or a code page that holds it", tbl.Name, tbl.Columns[col].Name, text, codePage)

					break
				}
			}
		}
	}
}

// summaryText fits the string summary properties into the summary code
// page, which is ANSI even for a UTF-8 database (SUM-04). The summary is
// informational, so characters it cannot hold become "?" with a warning
// rather than failing the build.
func (b *build) summaryText(props []summary.Property) {
	codePage := config.SummaryCodepage(b.cfg.Package.Codepage)

	for i, prop := range props {
		text, ok := prop.Value.(string)
		if !ok {
			continue
		}

		if _, err := codepage.Encode(text, codePage); err == nil {
			continue
		}

		var out strings.Builder

		for _, r := range text {
			if _, err := codepage.Encode(string(r), codePage); err != nil {
				out.WriteByte('?')
			} else {
				out.WriteRune(r)
			}
		}

		props[i].Value = out.String()

		// summaryProperties runs twice for an automatic package code, and
		// the subject and comments can both come from the product name.
		path := b.summaryPath(prop.ID)
		if !b.summaryWarned[path] {
			if b.summaryWarned == nil {
				b.summaryWarned = make(map[string]bool)
			}

			b.summaryWarned[path] = true
			b.warnf(path, "the summary information uses code page %d, so %q is stored as %q",
				codePage, text, out.String())
		}
	}
}

// summaryPath is the config field a summary string comes from.
func (b *build) summaryPath(pid int) string {
	product := b.cfg.Product

	switch pid {
	case summary.PIDTitle:
		return "package.summary.title"
	case summary.PIDSubject:
		if product.Description != config.DefaultDescription(&product) {
			return "product.description"
		}

		return "product.name"
	case summary.PIDAuthor:
		if b.cfg.Package.Summary.Author == product.Manufacturer {
			return "product.manufacturer"
		}

		return "package.summary.author"
	case summary.PIDKeywords:
		return "package.summary.keywords"
	case summary.PIDComments:
		if product.Comments != config.DefaultComments(&product) {
			return "product.comments"
		}

		return "product.name"
	default:
		return "package.codepage"
	}
}

// checkBinaryStreams reports binary cells of a custom table whose stream
// name, the table name and the row's keys (section 3), cannot be stored.
func (b *build) checkBinaryStreams(tablePath string, tbl *msidb.Table) {
	keys := tbl.Keys()

	for rowIndex, row := range tbl.Rows {
		for colIndex, col := range tbl.Columns {
			if col.Type.Kind != msidb.KindBinary || row[colIndex] == nil {
				continue
			}

			if err := msidb.CheckStreamName(msidb.BinaryStreamName(tbl.Name, row[:keys])); err != nil {
				b.errorf(fmt.Sprintf("%s.rows[%d]", tablePath, rowIndex),
					"the keys give a stream name that cannot be stored: %v", err)
			}
		}
	}
}

// checkStreams reports two streams with the same name: the cabinet and a
// binary cell, or rows whose keys join to one name, possibly across tables
// (a custom table Binary.x with key y and the binaries entry x.y). The
// package would otherwise fail to marshal without a config path. The
// builder's tables are claimed first, then the custom tables in config
// order, then the cabinet, and a clash is reported at the later claim.
func (b *build) checkStreams(tables []*msidb.Table) {
	byName := make(map[string]*msidb.Table, len(tables))
	for _, tbl := range tables {
		byName[tbl.Name] = tbl
	}

	custom := make(map[string]bool, len(b.cfg.Tables))
	for _, tbl := range b.cfg.Tables {
		custom[tbl.Name] = true
	}

	owners := make(map[string]string)

	for _, tbl := range tables {
		if !custom[tbl.Name] {
			b.claimRows(owners, tbl, func(_ int, key string) string { return b.builderRowPath(tbl.Name, key, owners) })
		}
	}

	for i, ct := range b.cfg.Tables {
		if tbl := byName[ct.Name]; tbl != nil {
			b.claimRows(owners, tbl, func(rowIndex int, _ string) string { return fmt.Sprintf("tables[%d].rows[%d]", i, rowIndex) })
		}
	}

	if b.cfg.Package.CabinetsEmbedded() {
		for _, cabinet := range b.cabinets {
			b.claimStream(owners, cabinet.name, "package.cabinet")
		}
	}
}

// claimRows claims the stream of every binary cell of tbl.
func (b *build) claimRows(owners map[string]string, tbl *msidb.Table, path func(rowIndex int, key string) string) {
	keys := tbl.Keys()

	for rowIndex, row := range tbl.Rows {
		for colIndex, col := range tbl.Columns {
			if col.Type.Kind == msidb.KindBinary && row[colIndex] != nil {
				b.claimStream(owners, msidb.BinaryStreamName(tbl.Name, row[:keys]), path(rowIndex, fmt.Sprint(row[0])))
			}
		}
	}
}

// claimStream records path as the owner of the stream name, or reports the
// owner it already has.
func (b *build) claimStream(owners map[string]string, name, path string) {
	if other, seen := owners[name]; seen {
		b.errorf(path, "the stream name %q is also used by %s; rename one of them", name, other)

		return
	}

	owners[name] = path
}

// builderRowPath is the config field a binary row of a builder table comes
// from. The binaries entry is added before the wizard bitmaps, so a second
// Binary row with the same key is the wizard's.
func (b *build) builderRowPath(table, key string, owners map[string]string) string {
	switch table {
	case "Icon":
		return "icons." + key
	case tableBinary:
		path := "binaries." + key
		if b.cfg.Binaries[key] != "" && owners["Binary."+key] != path {
			return path
		}

		return "ui"
	default:
		return "table " + table
	}
}
