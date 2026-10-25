package builder //nolint:testpackage // the tests reach the unexported comparison

import (
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
		}, fail: "column 1 read back as Num i4, wrote Num I4"},
		"column name": {got: &msidb.Table{
			Name:    "T",
			Columns: []msidb.Column{msidb.Col("Key", "s72", true), msidb.Col("Number", "I4", false), msidb.Col("Text", "S255", false), msidb.Col("Data", "V0", false)},
			Rows:    [][]any{{"a", 1, "x", []byte{1, 2}}, {"b", nil, nil, nil}},
		}, fail: "column 1 read back as Number"},
		"column count":  {got: &msidb.Table{Name: "T", Columns: want.Columns[:3], Rows: [][]any{{"a", 1, "x"}, {"b", nil, nil}}}, fail: "read back 3 columns, wrote 4"},
		"bad cell type": {got: cellTable([]any{"a", 1.5, "x", []byte{1, 2}}, []any{"b", nil, nil, nil}), fail: "unexpected cell type float64"},
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
