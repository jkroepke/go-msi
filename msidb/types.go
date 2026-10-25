// Package msidb models a Windows Installer database and serializes it into
// the streams of a compound file: string pool, catalog, column-major table
// streams, binary streams, embedded streams and summary information
// (docs/spec/MSISPEC.md section 3). Unmarshal reads a package back, which the
// builder uses to verify its own output.
package msidb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jkroepke/go-msi/cfb"
	"github.com/jkroepke/go-msi/summary"
)

// Kind is a column's storage kind.
type Kind int

// Column kinds.
const (
	KindString Kind = iota
	KindInt16
	KindInt32
	KindBinary
)

// Limits of the engine (DB-07, DB-11, section 3).
const (
	MaxColumns      = 31
	MaxTableName    = 31
	MaxRefcount     = 32767 // msi.dll keeps 15 bits (DB-05)
	longRefThresh   = 65535 // highest ID at which references become 3 bytes (DB-06)
	MinInt16        = -32767
	MaxInt16        = 32767
	MinInt32        = -2147483647
	MaxInt32        = 2147483647
	maxColumnName   = 64
	maxStringLength = 0xFFFFFFFF
)

// Type is a column type as the .idt notation describes it: s72, S255, l0,
// L255, i2, I4, v0, V0. Key marks a primary key column.
type Type struct {
	Kind        Kind
	Nullable    bool
	Localizable bool
	Key         bool
	// Width is the maximum string length in characters (0 = unbounded); it
	// is 2 or 4 for integers and 0 for binary.
	Width int
}

var typePattern = regexp.MustCompile(`^([sSlL])(\d{1,3})$|^([iI])([24])$|^([vV])0$`)

// ParseType parses the .idt notation. A trailing "!" or the Key field marks
// a primary key column in callers; ParseType itself only reads the type.
func ParseType(s string) (Type, error) {
	m := typePattern.FindStringSubmatch(s)
	if m == nil {
		return Type{}, fmt.Errorf("msidb: %q is not a column type (s72, S255, l0, L255, i2, I4, v0, V0)", s)
	}

	switch {
	case m[1] != "":
		w, _ := strconv.Atoi(m[2])
		if w > 255 {
			return Type{}, fmt.Errorf("msidb: string width %d exceeds 255", w)
		}

		return Type{
			Kind:        KindString,
			Nullable:    m[1] == "S" || m[1] == "L",
			Localizable: m[1] == "l" || m[1] == "L",
			Width:       w,
		}, nil
	case m[3] != "":
		t := Type{Kind: KindInt16, Width: 2, Nullable: m[3] == "I"}
		if m[4] == "4" {
			t.Kind, t.Width = KindInt32, 4
		}

		return t, nil
	default:
		return Type{Kind: KindBinary, Nullable: m[5] == "V"}, nil
	}
}

// MustType is ParseType for literals in table definitions; it panics on a
// malformed type.
func MustType(s string) Type {
	t, err := ParseType(s)
	if err != nil {
		panic(err)
	}

	return t
}

// String returns the .idt notation.
func (t Type) String() string {
	switch t.Kind {
	case KindString:
		c := "s"
		if t.Localizable {
			c = "l"
		}

		if t.Nullable {
			c = strings.ToUpper(c)
		}

		return c + strconv.Itoa(t.Width)
	case KindInt16:
		if t.Nullable {
			return "I2"
		}

		return "i2"
	case KindInt32:
		if t.Nullable {
			return "I4"
		}

		return "i4"
	default:
		if t.Nullable {
			return "V0"
		}

		return "v0"
	}
}

// Word is the 16-bit type word stored in _Columns.Type (section 3).
func (t Type) Word() uint16 {
	var w uint16

	switch t.Kind {
	case KindString:
		w = 0x0D00 | uint16(t.Width)
		if t.Localizable {
			w |= 0x0200
		}
	case KindInt16:
		w = 0x0502
	case KindInt32:
		w = 0x0104
	case KindBinary:
		w = 0x0900
	}

	if t.Nullable {
		w |= 0x1000
	}

	if t.Key {
		w |= 0x2000
	}

	return w
}

// TypeFromWord decodes a _Columns.Type word.
func TypeFromWord(w uint16) (Type, error) {
	t := Type{Nullable: w&0x1000 != 0, Key: w&0x2000 != 0}

	switch {
	case w&^0x3000 == 0x0900:
		t.Kind = KindBinary
	case w&0x0800 != 0:
		t.Kind = KindString
		t.Width = int(w & 0xFF)
		t.Localizable = w&0x0200 != 0
	case w&0xFF == 4:
		t.Kind, t.Width = KindInt32, 4
	case w&0xFF == 1 || w&0xFF == 2:
		t.Kind, t.Width = KindInt16, 2
	default:
		return Type{}, fmt.Errorf("msidb: unknown column type word %#x", w)
	}

	return t, nil
}

// storageWidth is the number of bytes a cell takes in a table stream.
func (t Type) storageWidth(longRefs bool) int {
	switch t.Kind {
	case KindInt32:
		return 4
	case KindString:
		if longRefs {
			return 3
		}

		return 2
	default:
		return 2
	}
}

// Column is a table column.
type Column struct {
	Name string
	Type Type
}

// Col is a shorthand constructor for table definitions: Col("File", "s72",
// true) is the primary key column File of type s72.
func Col(name, typ string, key bool) Column {
	t := MustType(typ)
	t.Key = key

	return Column{Name: name, Type: t}
}

// Table is a table with its rows. A cell is nil (NULL), string, int, int32,
// int64 or []byte (binary). An empty string is NULL.
type Table struct {
	Name    string
	Columns []Column
	Rows    [][]any
}

// Keys returns the number of primary key columns.
func (t *Table) Keys() int {
	n := 0
	for _, c := range t.Columns {
		if c.Type.Key {
			n++
		}
	}

	return n
}

// Column returns the index of the named column, or -1.
func (t *Table) Column(name string) int {
	for i, c := range t.Columns {
		if c.Name == name {
			return i
		}
	}

	return -1
}

// Add appends a row.
func (t *Table) Add(values ...any) {
	t.Rows = append(t.Rows, values)
}

// Database is a whole package database.
type Database struct {
	// Codepage of every string (0 ASCII, 65001 UTF-8 or an ANSI page).
	Codepage int
	Tables   []*Table
	// Streams are extra streams such as embedded cabinets, with decoded names
	// (the name without the "#" of Media.Cabinet).
	Streams []cfb.Stream
	// Summary is the summary information; nil writes no summary stream.
	Summary []summary.Property
	// Problems lists what Unmarshal found questionable but readable, such as
	// refcounts that do not match or unsorted rows.
	Problems []string
}

// Table returns the named table, or nil.
func (db *Database) Table(name string) *Table {
	for _, t := range db.Tables {
		if t.Name == name {
			return t
		}
	}

	return nil
}

// Stream returns the named extra stream.
func (db *Database) Stream(name string) ([]byte, bool) {
	for _, s := range db.Streams {
		if s.Name == name {
			return s.Data, true
		}
	}

	return nil, false
}

// normalizeCell converts a cell to its canonical Go value: nil, string,
// int or []byte, checked against the column type.
func normalizeCell(c Column, v any) (any, error) {
	if v == nil {
		if !c.Type.Nullable {
			return nil, fmt.Errorf("column %s is not nullable", c.Name)
		}

		return nil, nil
	}

	switch c.Type.Kind {
	case KindString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("column %s wants a string, got %T", c.Name, v)
		}

		if s == "" {
			if !c.Type.Nullable {
				return nil, fmt.Errorf("column %s is not nullable", c.Name)
			}

			return nil, nil
		}

		return s, nil
	case KindInt16, KindInt32:
		var n int64

		switch x := v.(type) {
		case int:
			n = int64(x)
		case int32:
			n = int64(x)
		case int64:
			n = x
		case int16:
			n = int64(x)
		case uint16:
			n = int64(x)
		case uint32:
			n = int64(x)
		default:
			return nil, fmt.Errorf("column %s wants an integer, got %T", c.Name, v)
		}

		if c.Type.Kind == KindInt16 && (n < MinInt16 || n > MaxInt16) {
			return nil, fmt.Errorf("column %s: %d is outside %d…%d", c.Name, n, MinInt16, MaxInt16)
		}

		if c.Type.Kind == KindInt32 && (n < MinInt32 || n > MaxInt32) {
			return nil, fmt.Errorf("column %s: %d is outside %d…%d", c.Name, n, MinInt32, MaxInt32)
		}

		return int(n), nil
	default:
		b, ok := v.([]byte)
		if !ok {
			return nil, fmt.Errorf("column %s wants []byte, got %T", c.Name, v)
		}

		return b, nil
	}
}

// Validate checks the table shape: name, column count, key placement and
// every cell.
func (t *Table) Validate() error {
	if t.Name == "" || len(t.Name) > MaxTableName {
		return fmt.Errorf("msidb: table name %q must be 1 to %d characters", t.Name, MaxTableName)
	}

	if _, err := EncodeStreamName(t.Name, true); err != nil {
		return err
	}

	if len(t.Columns) == 0 || len(t.Columns) > MaxColumns {
		return fmt.Errorf("msidb: table %s has %d columns, want 1 to %d (the engine rejects the 32nd, DB-07)", t.Name, len(t.Columns), MaxColumns)
	}

	keys, binaries := 0, 0
	seen := map[string]bool{}

	for i, c := range t.Columns {
		if c.Name == "" || len(c.Name) > maxColumnName {
			return fmt.Errorf("msidb: table %s: column %d has an invalid name %q", t.Name, i+1, c.Name)
		}

		if seen[c.Name] {
			return fmt.Errorf("msidb: table %s: column %s defined twice", t.Name, c.Name)
		}

		seen[c.Name] = true

		if c.Type.Key {
			if i != keys {
				return fmt.Errorf("msidb: table %s: key column %s must come before the other columns", t.Name, c.Name)
			}

			if c.Type.Kind == KindBinary {
				return fmt.Errorf("msidb: table %s: binary column %s cannot be a key", t.Name, c.Name)
			}

			keys++
		}

		if c.Type.Kind == KindBinary {
			binaries++
		}
	}

	if keys == 0 {
		return fmt.Errorf("msidb: table %s has no primary key", t.Name)
	}

	if binaries > 1 {
		return fmt.Errorf("msidb: table %s has %d binary columns, at most one is allowed", t.Name, binaries)
	}

	for r, row := range t.Rows {
		if len(row) != len(t.Columns) {
			return fmt.Errorf("msidb: table %s row %d has %d cells, want %d", t.Name, r, len(row), len(t.Columns))
		}

		for i, c := range t.Columns {
			v, err := normalizeCell(c, row[i])
			if err != nil {
				return fmt.Errorf("msidb: table %s row %d: %w", t.Name, r, err)
			}

			row[i] = v
		}
	}

	return nil
}
