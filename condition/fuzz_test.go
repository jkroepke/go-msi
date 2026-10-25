package condition_test

import (
	"testing"

	"github.com/jkroepke/go-msi/condition"
)

// FuzzCheck feeds arbitrary conditions to the parser.
func FuzzCheck(f *testing.F) {
	for _, s := range []string{
		`NOT Installed`, `VersionNT64 >= 603 AND (REMOVE~<>"ALL" OR &Main=3)`,
		`$Comp=2`, `?Comp=3`, `!Main=2`, `%PATH><"x"`, `PROP << "a" OR PROP >> "b"`,
		`1`, `"str"`, `((A))`, `A XOR B EQV C IMP D`,
	} {
		f.Add(s)
	}

	f.Fuzz(func(_ *testing.T, s string) {
		if condition.Check(s) == nil {
			_ = condition.Properties(s)
		}
	})
}
