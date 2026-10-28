package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/msidb"
)

// compareTable checks that a table read back from the package holds the
// same columns and the same cells as the table that was written. Rows are
// compared as a set: the writer stores them in key order, which is not the
// order the builder produced them in.
func compareTable(want, got *msidb.Table) error {
	if len(got.Columns) != len(want.Columns) {
		return fmt.Errorf("table %s: read back %d columns, wrote %d", want.Name, len(got.Columns), len(want.Columns))
	}

	for i, col := range want.Columns {
		// Type is a plain struct, so == covers the kind, width, the
		// nullable and localizable flags and the key flag.
		if got.Columns[i].Name != col.Name || got.Columns[i].Type != col.Type {
			return fmt.Errorf("table %s: column %d read back as %s %s (key %t), wrote %s %s (key %t)", want.Name, i,
				got.Columns[i].Name, got.Columns[i].Type, got.Columns[i].Type.Key, col.Name, col.Type, col.Type.Key)
		}
	}

	if len(got.Rows) != len(want.Rows) {
		return fmt.Errorf("table %s: read back %d rows, wrote %d", want.Name, len(got.Rows), len(want.Rows))
	}

	wantRows, err := canonicalRows(want)
	if err != nil {
		return err
	}

	gotRows, err := canonicalRows(got)
	if err != nil {
		return err
	}

	for i := range wantRows {
		if wantRows[i] != gotRows[i] {
			return fmt.Errorf("table %s: row %s was read back as %s", want.Name, describeRow(wantRows[i]), describeRow(gotRows[i]))
		}
	}

	return nil
}

// canonicalRows encodes every row of tbl in a form that is the same for
// equal cells whatever their Go type, sorted so that two tables with the
// same rows in any order compare equal.
func canonicalRows(tbl *msidb.Table) ([]string, error) {
	rows := make([]string, 0, len(tbl.Rows))

	for i, row := range tbl.Rows {
		if len(row) != len(tbl.Columns) {
			return nil, fmt.Errorf("table %s: row %d has %d cells, want %d", tbl.Name, i, len(row), len(tbl.Columns))
		}

		var sb strings.Builder

		for col, cell := range row {
			if col > 0 {
				sb.WriteByte('\x1f')
			}

			canonical, err := canonicalCell(cell)
			if err != nil {
				return nil, fmt.Errorf("table %s: row %d column %s: %w", tbl.Name, i, tbl.Columns[col].Name, err)
			}

			sb.WriteString(canonical)
		}

		rows = append(rows, sb.String())
	}

	slices.Sort(rows)

	return rows, nil
}

// canonicalCell returns a tagged encoding of a cell: N for NULL (nil or the
// empty string), S for a string (quoted, so that a separator inside the
// value cannot shift cell boundaries), I for any integer type and B for
// binary data, which is represented by its SHA-256 digest and length.
func canonicalCell(cell any) (string, error) {
	switch value := cell.(type) {
	case nil:
		return "N", nil
	case string:
		if value == "" {
			return "N", nil
		}

		return "S" + strconv.Quote(value), nil
	case []byte:
		sum := sha256.Sum256(value)

		return "B" + hex.EncodeToString(sum[:]) + ":" + strconv.Itoa(len(value)), nil
	default:
		num, ok := canonicalInt(cell)
		if !ok {
			return "", fmt.Errorf("unexpected cell type %T", cell)
		}

		return "I" + strconv.FormatInt(num, 10), nil
	}
}

// canonicalInt widens the integer types a cell may hold to int64.
func canonicalInt(cell any) (int64, bool) {
	switch value := cell.(type) {
	case int:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	case uint16:
		return int64(value), true
	case uint32:
		return int64(value), true
	default:
		return 0, false
	}
}

// describeRow renders a canonical row for an error message.
func describeRow(row string) string {
	cells := strings.Split(row, "\x1f")
	for i, cell := range cells {
		switch {
		case cell == "N":
			cells[i] = "NULL"
		case strings.HasPrefix(cell, "S"):
			cells[i] = cell[1:]
		case strings.HasPrefix(cell, "B"):
			cells[i] = "binary " + cell[1:]
		default:
			cells[i] = cell[1:]
		}
	}

	return "[" + strings.Join(cells, ", ") + "]"
}
