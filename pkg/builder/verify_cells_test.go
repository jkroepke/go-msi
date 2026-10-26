package builder //nolint:testpackage // the tests reach the unexported comparison

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/pkg/msidb"
)

func cellTable(rows ...[]any) *msidb.Table {
	return &msidb.Table{
		Name:    "T",
		Columns: []msidb.Column{msidb.Col("Key", "s72", true), msidb.Col("Num", "I4", false), msidb.Col("Text", "S255", false), msidb.Col("Data", "V0", false)},
		Rows:    rows,
	}
}

// TestCompareTableSeparator checks that a separator byte inside a string
// cannot move a cell boundary and make two different rows compare equal.
func TestCompareTableSeparator(t *testing.T) {
	t.Parallel()

	want := cellTable([]any{"a", 1, "x\x1fSy", nil})
	got := cellTable([]any{"a", 1, "x", "y\x1fN"})

	if err := compareTable(want, got); err == nil {
		t.Error("rows that differ only around the separator compared equal")
	}
}

// TestCompareTableReadBack runs the comparison against a table decoded by
// msidb, so that the cell types the decoder produces are accepted, and
// checks that a corrupted string pool or integer cell is caught.
func TestCompareTableReadBack(t *testing.T) {
	t.Parallel()

	want := cellTable([]any{"alpha", int32(0x01020304), "hello", []byte{9, 8, 7}}, []any{"beta", nil, "", nil})

	data, err := (&msidb.Database{Codepage: 1252, Tables: []*msidb.Table{cellTable(want.Rows...)}}).Marshal(msidb.Options{})
	if err != nil {
		t.Fatal(err)
	}

	back, err := msidb.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if err := compareTable(want, back.Table("T")); err != nil {
		t.Errorf("an unchanged package does not verify: %v", err)
	}

	// Corrupt the string pool ("hello" -> "jello") and the integer cell.
	for name, patch := range map[string]func([]byte) bool{
		"string": func(pkg []byte) bool {
			i := bytes.Index(pkg, []byte("hello"))
			if i < 0 {
				return false
			}

			pkg[i] = 'j'

			return true
		},
		"integer": func(pkg []byte) bool {
			// The I4 cell is stored biased: 0x01020304 ^ 0x80000000, little endian.
			i := bytes.Index(pkg, []byte{0x04, 0x03, 0x02, 0x81})
			if i < 0 {
				return false
			}

			pkg[i]++

			return true
		},
	} {
		patched := append([]byte(nil), data...)
		if !patch(patched) {
			t.Fatalf("%s: the pattern is not in the package", name)
		}

		back, err := msidb.Unmarshal(patched)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if err := compareTable(want, back.Table("T")); err == nil {
			t.Errorf("%s: the corrupted cell was not caught", name)
		}
	}
}

func TestCompareTable(t *testing.T) {
	t.Parallel()

	want := cellTable([]any{"a", 1, "x", []byte{1, 2}}, []any{"b", nil, "", nil})

	for name, tc := range map[string]struct {
		got  *msidb.Table
		fail string
	}{
		"same":            {got: cellTable([]any{"a", 1, "x", []byte{1, 2}}, []any{"b", nil, "", nil})},
		"other order":     {got: cellTable([]any{"b", nil, nil, nil}, []any{"a", 1, "x", []byte{1, 2}})},
		"other int types": {got: cellTable([]any{"a", int32(1), "x", []byte{1, 2}}, []any{"b", nil, nil, nil})},
		"changed string":  {got: cellTable([]any{"a", 1, "y", []byte{1, 2}}, []any{"b", nil, nil, nil}), fail: `row ["a", 1, "x", binary`},
		"changed int":     {got: cellTable([]any{"a", 2, "x", []byte{1, 2}}, []any{"b", nil, nil, nil}), fail: "was read back as"},
		"changed binary":  {got: cellTable([]any{"a", 1, "x", []byte{2, 1}}, []any{"b", nil, nil, nil}), fail: "was read back as"},
		"null for value":  {got: cellTable([]any{"a", 1, "x", []byte{1, 2}}, []any{"b", 0, nil, nil}), fail: "was read back as"},
		"missing row":     {got: cellTable([]any{"a", 1, "x", []byte{1, 2}}), fail: "read back 1 rows, wrote 2"},
		"short row":       {got: cellTable([]any{"a", 1, "x"}, []any{"b", nil, nil, nil}), fail: "has 3 cells, want 4"},
		"column type": {got: &msidb.Table{
			Name:    "T",
			Columns: []msidb.Column{msidb.Col("Key", "s72", true), msidb.Col("Num", "i4", false), msidb.Col("Text", "S255", false), msidb.Col("Data", "V0", false)},
			Rows:    [][]any{{"a", 1, "x", []byte{1, 2}}, {"b", nil, nil, nil}},
		}, fail: "column 1 read back as Num i4 (key false), wrote Num I4 (key false)"},
		"column name": {got: &msidb.Table{
			Name:    "T",
			Columns: []msidb.Column{msidb.Col("Key", "s72", true), msidb.Col("Number", "I4", false), msidb.Col("Text", "S255", false), msidb.Col("Data", "V0", false)},
			Rows:    [][]any{{"a", 1, "x", []byte{1, 2}}, {"b", nil, nil, nil}},
		}, fail: "column 1 read back as Number"},
		"column count":  {got: &msidb.Table{Name: "T", Columns: want.Columns[:3], Rows: [][]any{{"a", 1, "x"}, {"b", nil, nil}}}, fail: "read back 3 columns, wrote 4"},
		"bad cell type": {got: cellTable([]any{"a", 1.5, "x", []byte{1, 2}}, []any{"b", nil, nil, nil}), fail: "unexpected cell type float64"},
		"key flag": {got: &msidb.Table{
			Name:    "T",
			Columns: []msidb.Column{msidb.Col("Key", "s72", false), msidb.Col("Num", "I4", false), msidb.Col("Text", "S255", false), msidb.Col("Data", "V0", false)},
			Rows:    [][]any{{"a", 1, "x", []byte{1, 2}}, {"b", nil, nil, nil}},
		}, fail: "column 0 read back as Key s72 (key false), wrote Key s72 (key true)"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := compareTable(want, tc.got)

			switch {
			case tc.fail == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tc.fail != "" && err == nil:
				t.Errorf("no error, want one containing %q", tc.fail)
			case tc.fail != "" && !strings.Contains(err.Error(), tc.fail):
				t.Errorf("error %q does not contain %q", err, tc.fail)
			}
		})
	}
}
