package schema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/msidb"
)

func TestDefinitions(t *testing.T) {
	categories := map[string]bool{}
	for c := range strings.SplitSeq(Categories, ";") {
		categories[c] = true
	}

	if len(categories) != 26 {
		t.Errorf("%d categories, want 26", len(categories))
	}

	for _, name := range Names() {
		def, _ := Lookup(name)
		tb := def.Table()

		if err := tb.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}

		for _, c := range def.Columns {
			if c.Category != "" && !categories[c.Category] {
				t.Errorf("%s.%s: category %q is not in the set", name, c.Name, c.Category)
			}

			if c.Type.Kind == msidb.KindString && c.Category == "" && name != "_Validation" {
				t.Errorf("%s.%s: string column without a category", name, c.Name)
			}

			if c.Type.Kind != msidb.KindString && c.Type.Kind != msidb.KindBinary && c.Category != "" {
				t.Errorf("%s.%s: integer column with category %q", name, c.Name, c.Category)
			}

			if c.KeyTable != "" {
				for kt := range strings.SplitSeq(c.KeyTable, ";") {
					if _, ok := Lookup(kt); !ok && kt != "ODBCDataSource" {
						t.Errorf("%s.%s: key table %s is not defined", name, c.Name, kt)
					}
				}

				if c.KeyColumn == 0 {
					t.Errorf("%s.%s: key table without key column", name, c.Name)
				}
			}
		}
	}

	if New("File").Column("Sequence") != 7 || New("File").Columns[7].Type.String() != "i4" {
		t.Error("File.Sequence is not the eighth column of type i4")
	}
}

func TestValidation(t *testing.T) {
	custom := &msidb.Table{Name: "MyTable", Columns: []msidb.Column{msidb.Col("Key", "s72", true), msidb.Col("Value", "I4", false)}}
	v := Validation([]*msidb.Table{New("Property"), New("Media"), custom})

	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}

	count := map[string]int{}
	for _, r := range v.Rows {
		count[fmt.Sprint(r[0])]++
	}

	if count["Property"] != 2 || count["Media"] != 6 || count["_Validation"] != 10 || count["MyTable"] != 2 || count["_SummaryInformation"] != 2 || len(count) != 5 {
		t.Errorf("rows per table: %v", count)
	}

	db := &msidb.Database{Codepage: 1252, Tables: []*msidb.Table{v}}
	if _, err := db.Marshal(msidb.Options{}); err != nil {
		t.Fatal(err)
	}
}
