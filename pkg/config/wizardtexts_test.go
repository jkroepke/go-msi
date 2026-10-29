package config //nolint:testpackage // the tests reach the catalog files

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jkroepke/msi-builder/internal/codepage"
)

// placeholders are the parts of a wizard text that the engine resolves:
// in formatted text any [name] is a property reference, so a translation
// must not add brackets.
var placeholders = regexp.MustCompile(`\[[^\]]*\]|[{}]`)

// TestTranslations checks every built-in translation against the English
// texts: the same keys, the same placeholders, an access key where English
// has one, the column widths, and text that its code page can hold.
func TestTranslations(t *testing.T) {
	t.Parallel()

	english := UIStrings()

	for lcid, texts := range catalogs() {
		codepageID, ok := LanguageCodepage(lcid)
		if !ok {
			t.Errorf("%d: not in the language table", lcid)
		}

		if codepageID == 0 {
			codepageID = 65001
		}

		if got, want := slices.Sorted(maps.Keys(texts)), slices.Sorted(maps.Keys(english)); !slices.Equal(got, want) {
			t.Errorf("%d: keys differ from the English texts:\n%v\nwant\n%v", lcid, got, want)
		}

		for key, text := range texts {
			source := english[key]

			if got, want := slices.Sorted(slices.Values(placeholders.FindAllString(text, -1))),
				slices.Sorted(slices.Values(placeholders.FindAllString(source, -1))); !slices.Equal(got, want) {
				t.Errorf("%d %s: placeholders %v, want %v", lcid, key, got, want)
			}

			if strings.Count(text, "&") != strings.Count(source, "&") {
				t.Errorf("%d %s: %q has %d &, English %d", lcid, key, text, strings.Count(text, "&"), strings.Count(source, "&"))
			}

			if strings.HasSuffix(text, "&") {
				t.Errorf("%d %s: & marks no access key", lcid, key)
			}

			if (text == "") != (source == "") {
				t.Errorf("%d %s: empty differs from English", lcid, key)
			}

			if width := uiStringWidth(key); width > 0 && utf8.RuneCountInString(text) > width {
				t.Errorf("%d %s: %d characters exceed the column width %d", lcid, key, utf8.RuneCountInString(text), width)
			}

			if _, err := codepage.Encode(text, codepageID); err != nil {
				t.Errorf("%d %s: code page %d: %v", lcid, key, codepageID, err)
			}
		}
	}
}

func TestTranslationFallback(t *testing.T) {
	t.Parallel()

	if Translation(1031) == nil {
		t.Fatal("no German translation")
	}

	if got, want := Translation(3079)["Next"], Translation(1031)["Next"]; got != want {
		t.Errorf("de-AT Next = %q, want the German %q", got, want)
	}

	if got := Translation(2057)["Next"]; got != uiStrings["Next"] {
		t.Errorf("en-GB Next = %q, want the built-in English text", got)
	}

	if Translation(0x7F) != nil {
		t.Error("an LCID without a translation returned one")
	}
}

func TestWizardTexts(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Product: Product{Language: new(1033)},
		UI: UI{
			Strings: map[string]string{"WelcomeTitle": "Hello", "ExitText": "Bye"},
			Languages: map[string]*Language{
				"de": {Strings: map[string]string{"ExitText": "Tschüss"}},
				"1087": nil,
			},
		},
	}

	base := cfg.WizardTexts(1033)
	if base["WelcomeTitle"] != "Hello" || base["Next"] != uiStrings["Next"] {
		t.Errorf("base texts: %q, %q", base["WelcomeTitle"], base["Next"])
	}

	german := cfg.WizardTexts(1031)
	for key, want := range map[string]string{"ExitText": "Tschüss", "Next": Translation(1031)["Next"], "WelcomeTitle": Translation(1031)["WelcomeTitle"]} {
		if german[key] != want {
			t.Errorf("German %s = %q, want %q", key, german[key], want)
		}
	}
}

func TestLanguageID(t *testing.T) {
	t.Parallel()

	for key, want := range map[string]int{"de": 1031, "DE-at": 3079, "1031": 1031, "es": 3082, "pt-PT": 2070, "hi-IN": 1081, "zh-CN": 2052} {
		if got, err := LanguageID(key); err != nil || got != want {
			t.Errorf("LanguageID(%q) = %d, %v; want %d", key, got, err, want)
		}
	}

	for _, key := range []string{"pt", "zh", "xx", "0", "70000", "de_DE"} {
		if _, err := LanguageID(key); err == nil {
			t.Errorf("LanguageID(%q) succeeded", key)
		}
	}
}

func TestDefaultCodepage(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		wizard    string
		languages []string
		base      int
		want      int
	}{
		"no wizard":         {None, nil, 1049, 1252},
		"English":           {"minimal", nil, 1033, 1252},
		"German base":       {"minimal", nil, 1031, 1252},
		"one ANSI page":     {"minimal", []string{"de", "fr"}, 1033, 1252},
		"Cyrillic":          {"minimal", []string{"ru", "uk"}, 1033, 65001},
		"Hindi":             {"minimal", []string{"hi"}, 1033, 65001},
		"unknown LCID base": {"minimal", []string{"de"}, 1279, 65001},
	} {
		cfg := &Config{Product: Product{Language: new(tc.base)}, UI: UI{Wizard: tc.wizard, Languages: map[string]*Language{}}}
		for _, l := range tc.languages {
			cfg.UI.Languages[l] = nil
		}

		if got := defaultCodepage(cfg); got != tc.want {
			t.Errorf("%s: %d, want %d", name, got, tc.want)
		}
	}
}
