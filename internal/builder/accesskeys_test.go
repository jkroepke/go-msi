package builder_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// accessKey returns the lower-case letter after the & that marks a
// control's access key, or 0. && is a literal & and marks none.
func accessKey(text string) rune {
	for {
		_, after, ok := strings.Cut(text, "&")
		if !ok {
			return 0
		}

		r, _ := utf8.DecodeRuneInString(after)
		if r != '&' {
			return unicode.ToLower(r)
		}

		text = after[1:]
	}
}

// str returns a string cell, or "" for a null one.
func str(cell any) string {
	s, ok := cell.(string)
	if !ok {
		return ""
	}

	return s
}

// dialogAccessKeys returns, per dialog, the controls and radio buttons of
// each access key.
func dialogAccessKeys(t *testing.T, database *msidb.Database) map[string]map[rune][]string {
	t.Helper()

	out := make(map[string]map[rune][]string)
	add := func(dialog, name, text string) {
		key := accessKey(text)
		if key == 0 {
			return
		}

		if out[dialog] == nil {
			out[dialog] = make(map[rune][]string)
		}

		out[dialog][key] = append(out[dialog][key], fmt.Sprintf("%s %q", name, text))
	}

	groups := make(map[string][]string)

	for _, row := range rows(t, database, "Control") {
		dialog := str(row[0])
		add(dialog, str(row[1]), str(row[9]))

		if row[2] == "RadioButtonGroup" {
			groups[str(row[8])] = append(groups[str(row[8])], dialog)
		}
	}

	if database.Table("RadioButton") != nil {
		for _, row := range rows(t, database, "RadioButton") {
			for _, dialog := range groups[str(row[0])] {
				add(dialog, str(row[0])+"="+fmt.Sprint(row[2]), str(row[7]))
			}
		}
	}

	return out
}

// TestAccessKeysUnique builds the wizards in English and every built-in
// translation and checks that no two controls of a dialog share an access
// key, so Alt+key presses the one control it names.
func TestAccessKeysUnique(t *testing.T) {
	t.Parallel()

	lcids := append([]int{1033}, config.TranslatedLanguages()...)
	slices.Sort(lcids)

	for _, wizard := range []string{"minimal", "featureTree"} {
		for _, lcid := range lcids {
			cfg := desktop(t)
			cfg.UI.Wizard = wizard
			cfg.Product.Language = new(lcid)
			cfg.Package.Codepage = 0

			res := mustBuild(t, cfg)

			for dialog, keys := range dialogAccessKeys(t, res.Database) {
				for key, controls := range keys {
					if len(controls) > 1 {
						t.Errorf("%s %d %s: access key %q on %s", wizard, lcid, dialog, key, strings.Join(controls, ", "))
					}
				}
			}
		}
	}
}
