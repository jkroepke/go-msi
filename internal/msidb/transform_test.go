package msidb //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/cfb"
)

func propertyDB(rows ...[]any) *Database {
	tbl := &Table{Name: "Property", Columns: []Column{Col("Property", "s72", true), Col("Value", "l0", false)}, Rows: rows}

	return &Database{Tables: []*Table{tbl}, Codepage: 1252}
}

// TestMarshalTransform checks the record layout MSIDB-12 observed in
// transforms that msi.dll writes: a 16-bit mask, insert with the column
// count in the high byte, update with one bit per present column, delete
// with the keys alone, rows in key order.
func TestMarshalTransform(t *testing.T) {
	t.Parallel()

	base := propertyDB([]any{"ProductName", "A"}, []any{"Gone", "x"}, []any{"Same", "s"})
	variant := propertyDB([]any{"ProductName", "B"}, []any{"New", "n"}, []any{"Same", "s"})

	out, err := MarshalTransform(base, variant, TransformOptions{})
	if err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(out)
	if err != nil {
		t.Fatal(err)
	}

	if file.CLSID != cfb.CLSIDTransform {
		t.Errorf("CLSID %x", file.CLSID)
	}

	stream := func(name string) string {
		encoded, err := EncodeStreamName(name, true)
		if err != nil {
			t.Fatal(err)
		}

		data, ok := file.Stream(encoded)
		if !ok {
			t.Fatalf("no stream %s", name)
		}

		return hex.EncodeToString(data)
	}

	// IDs by encoded bytes: B 1, Gone 2, New 3, ProductName 4, n 5.
	for name, want := range map[string]string{
		TableStringPool: "e4040000" + "01000100" + "04000100" + "03000100" + "0b000100" + "01000100",
		TableStringData: hex.EncodeToString([]byte("BGoneNewProductNamen")),
		"Property":      "0000" + "0200" + "0102" + "0300" + "0500" + "0200" + "0400" + "0100",
	} {
		if got := stream(name); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}

	if len(file.Streams()) != 3 {
		t.Errorf("%d streams, want the pool and Property only", len(file.Streams()))
	}
}

func TestMarshalTransformIntegers(t *testing.T) {
	t.Parallel()

	cols := []Column{Col("Key", "s72", true), Col("Num", "I2", false), Col("Big", "I4", false)}
	base := &Database{Codepage: 1252, Tables: []*Table{{Name: "T", Columns: cols, Rows: [][]any{{"b", 2, 200000}}}}}
	variant := &Database{Codepage: 1252, Tables: []*Table{{Name: "T", Columns: cols, Rows: [][]any{{"b", nil, -5}}}}}

	out, err := MarshalTransform(base, variant, TransformOptions{})
	if err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(out)
	if err != nil {
		t.Fatal(err)
	}

	encoded, _ := EncodeStreamName("T", true)
	data, _ := file.Stream(encoded)

	// The bytes msi.dll wrote for the same change in MSIDB-12.
	if want, _ := hex.DecodeString("060001000000fbffff7f"); !bytes.Equal(data, want) {
		t.Errorf("T = %x, want %x", data, want)
	}
}

func TestMarshalTransformRefuses(t *testing.T) {
	t.Parallel()

	wide := make([]Column, 17)
	for i := range wide {
		wide[i] = Col("C"+strings.Repeat("x", i+1), "s72", i == 0)
	}

	other := &Database{Codepage: 1252, Tables: []*Table{{Name: "Other", Columns: []Column{Col("K", "s72", true)}}}}

	withOther := func() *Database {
		return &Database{Codepage: 1252, Tables: append(propertyDB().Tables, other.Tables...)}
	}
	unicode := propertyDB()
	unicode.Codepage = 65001
	neutral := propertyDB()
	neutral.Codepage = 0
	wideDB := func() *Database { return &Database{Codepage: 1252, Tables: []*Table{{Name: "W", Columns: wide}}} }

	for name, pair := range map[string][2]*Database{
		"added table":   {propertyDB(), withOther()},
		"dropped table": {withOther(), propertyDB()},
		"code page":     {propertyDB(), unicode},
		"to neutral":    {unicode, neutral},
		"wide table":    {wideDB(), wideDB()},
	} {
		if _, err := MarshalTransform(pair[0], pair[1], TransformOptions{}); err == nil {
			t.Errorf("%s: MarshalTransform succeeded", name)
		}
	}
}

// TestMarshalTransformNeutralBase checks that a neutral base may take the
// variant's code page, the one switch msi.dll applied in MSIDB-12.
func TestMarshalTransformNeutralBase(t *testing.T) {
	t.Parallel()

	base := propertyDB([]any{"ProductName", "A"})
	base.Codepage = 0
	variant := propertyDB([]any{"ProductName", "Пример"})
	variant.Codepage = 65001

	out, err := MarshalTransform(base, variant, TransformOptions{})
	if err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(out)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeStreamName(TableStringPool, true)
	if err != nil {
		t.Fatal(err)
	}

	data, ok := file.Stream(encoded)
	if !ok || !bytes.HasPrefix(data, []byte{0xe9, 0xfd, 0, 0}) {
		t.Errorf("pool header % x, want code page 65001", data[:min(4, len(data))])
	}
}
