package config

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Language is one entry of ui.languages: a further wizard language of the
// package, built as an embedded language transform (spec section 7.6). A
// field left out keeps the base language's value.
type Language struct {
	// Strings replaces wizard texts in this language by ui.strings key.
	// Keys it leaves out take the built-in translation, if there is one,
	// and otherwise the base language's text.
	Strings map[string]string `yaml:"strings"`
	// FinishText replaces ui.finishText.
	FinishText string `yaml:"finishText"`
	// License replaces ui.license.
	License string `yaml:"license"`
}

// locale is a language tag with its LCID and ANSI code page; 0 marks a
// language that no ANSI code page can write: Hindi, and Kazakh, whose
// letters such as қ and ө are missing from code page 1251.
type locale struct {
	tag      string
	lcid     int
	codepage int
}

// locales are the language tags that ui.languages and LanguageID accept,
// besides a decimal LCID. A short tag names the language's main locale;
// pt, sr and zh have none, because their scripts or spellings differ.
var locales = []locale{ //nolint:gochecknoglobals // lookup table
	{"ar-SA", 1025, 1256}, {"bg-BG", 1026, 1251}, {"ca-ES", 1027, 1252}, {"cs-CZ", 1029, 1250},
	{"da-DK", 1030, 1252}, {"de-DE", 1031, 1252}, {"de-AT", 3079, 1252}, {"de-CH", 2055, 1252},
	{"el-GR", 1032, 1253}, {"en-US", 1033, 1252}, {"en-GB", 2057, 1252}, {"es-ES", 3082, 1252},
	{"es-MX", 2058, 1252}, {"et-EE", 1061, 1257}, {"fi-FI", 1035, 1252}, {"fr-FR", 1036, 1252},
	{"fr-BE", 2060, 1252}, {"fr-CA", 3084, 1252}, {"fr-CH", 4108, 1252}, {"he-IL", 1037, 1255},
	{"hi-IN", 1081, 0}, {"hr-HR", 1050, 1250}, {"hu-HU", 1038, 1250}, {"it-IT", 1040, 1252},
	{"it-CH", 2064, 1252}, {"ja-JP", 1041, 932}, {"kk-KZ", 1087, 0}, {"ko-KR", 1042, 949},
	{"lt-LT", 1063, 1257}, {"lv-LV", 1062, 1257}, {"nb-NO", 1044, 1252}, {"nl-NL", 1043, 1252},
	{"nl-BE", 2067, 1252}, {"pl-PL", 1045, 1250}, {"pt-BR", 1046, 1252}, {"pt-PT", 2070, 1252},
	{"ro-RO", 1048, 1250}, {"ru-RU", 1049, 1251}, {"sk-SK", 1051, 1250}, {"sl-SI", 1060, 1250},
	{"sr-Latn-CS", 2074, 1250}, {"sr-Latn-RS", 9242, 1250}, {"sv-SE", 1053, 1252}, {"th-TH", 1054, 874},
	{"tr-TR", 1055, 1254}, {"uk-UA", 1058, 1251}, {"zh-CN", 2052, 936}, {"zh-HK", 3076, 950},
	{"zh-TW", 1028, 950},
}

// shortTags maps a language without a region to its main locale.
var shortTags = map[string]string{ //nolint:gochecknoglobals // lookup table
	"ar": "ar-SA", "bg": "bg-BG", "ca": "ca-ES", "cs": "cs-CZ", "da": "da-DK", "de": "de-DE", "el": "el-GR",
	"en": "en-US", "es": "es-ES", "et": "et-EE", "fi": "fi-FI", "fr": "fr-FR", "he": "he-IL", "hi": "hi-IN",
	"hr": "hr-HR", "hu": "hu-HU", "it": "it-IT", "ja": "ja-JP", "kk": "kk-KZ", "ko": "ko-KR", "lt": "lt-LT",
	"lv": "lv-LV", "nb": "nb-NO", "nl": "nl-NL", "pl": "pl-PL", "ro": "ro-RO", "ru": "ru-RU", "sk": "sk-SK",
	"sl": "sl-SI", "sv": "sv-SE", "th": "th-TH", "tr": "tr-TR", "uk": "uk-UA",
}

// LanguageID returns the LCID of a ui.languages key: a decimal LCID, a
// language tag such as de-AT, or a short tag such as de.
func LanguageID(key string) (int, error) {
	if n, err := strconv.Atoi(key); err == nil {
		if n <= 0 || n > 0xFFFF {
			return 0, fmt.Errorf("%d is not an LCID", n)
		}

		return n, nil
	}

	tag := key
	if full, ok := shortTags[strings.ToLower(key)]; ok {
		tag = full
	}

	for _, l := range locales {
		if strings.EqualFold(l.tag, tag) {
			return l.lcid, nil
		}
	}

	return 0, fmt.Errorf("%q is not an LCID or a known language tag", key)
}

// LanguageCodepage returns the ANSI code page of an LCID: 0 for a language
// that Windows writes only in Unicode, and ok false for an LCID that is not
// in the table.
func LanguageCodepage(lcid int) (int, bool) {
	for _, l := range locales {
		if l.lcid == lcid {
			return l.codepage, true
		}
	}

	return 0, false
}

// LanguageIDs maps the LCIDs of ui.languages to their keys; of two keys
// for one LCID it keeps the first in sorted order. Keys
// that are not languages are left out; Validate reports them.
func (u *UI) LanguageIDs() map[int]string {
	out := make(map[int]string, len(u.Languages))

	for _, key := range slices.Sorted(maps.Keys(u.Languages)) {
		if lcid, err := LanguageID(key); err == nil {
			if _, dup := out[lcid]; !dup {
				out[lcid] = key
			}
		}
	}

	return out
}

// defaultCodepage is the default of package.codepage. A package whose
// wizard shows a built-in translation or has ui.languages gets the ANSI
// code page all its languages share, or 65001 when they need different
// ones or one is not in the table; any other package gets 1252.
func defaultCodepage(cfg *Config) int {
	base := *cfg.Product.Language
	translated := cfg.UI.Enabled &&
		primaryLanguage(base) != englishPrimary && Translation(base) != nil

	if !translated && len(cfg.UI.Languages) == 0 {
		return DefaultCodepage
	}

	lcids := append([]int{base}, slices.Collect(maps.Keys(cfg.UI.LanguageIDs()))...)

	return sharedCodepage(lcids)
}

// sharedCodepage is the ANSI code page of all lcids, or 65001 when they
// need different ones or one is Unicode-only or not in the table.
func sharedCodepage(lcids []int) int {
	shared := 0

	for _, lcid := range lcids {
		cp, ok := LanguageCodepage(lcid)
		if !ok || cp == 0 || (shared != 0 && cp != shared) {
			return 65001
		}

		shared = cp
	}

	return shared
}

// uiLanguages checks ui.languages: keys that name a language once, not the
// base language, and texts that ui.strings would accept.
func (v *validator) uiLanguages(cfg *Config) {
	if len(cfg.UI.Languages) == 0 {
		return
	}

	if !cfg.UI.Enabled {
		v.errorf("ui.languages", "wizard languages need a wizard")
	}

	seen := make(map[int]string)

	for _, key := range slices.Sorted(maps.Keys(cfg.UI.Languages)) {
		path := "ui.languages." + key

		lcid, err := LanguageID(key)
		if err != nil {
			v.errorf(path, "%v", err)

			continue
		}

		switch other, dup := seen[lcid]; {
		case dup:
			v.errorf(path, "%s and %s are both language %d", other, key, lcid)
		case lcid == *cfg.Product.Language:
			v.errorf(path, "%s is product.language, the package's base language; list only further languages", key)
		}

		seen[lcid] = key

		entry := cfg.UI.Languages[key]
		if entry == nil {
			continue
		}

		v.checkUIStrings(path+".strings", entry.Strings)
		v.checkFinishText(path+".finishText", entry.FinishText)
	}
}
