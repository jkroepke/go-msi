package config

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Language is one entry of ui.languages: a further wizard language of the
// package, built as an embedded language transform (spec section 7.6). A
// field left out keeps the base language's value.
type Language struct {
	// Strings replaces wizard texts in this language by ui.strings key.
	// Keys it leaves out take the built-in translation, if there is one,
	// and otherwise the base language's text.
	Strings map[string]string `yaml:"strings"`
	// Dialogs replaces the texts of ui.dialogs pages, by dialog id.
	Dialogs map[string]*LanguageDialog `yaml:"dialogs"`
	// Launch replaces the texts of ui.launch.
	Launch *LanguageLaunch `yaml:"launch"`
	// Progress replaces the progress texts of customActions, by action ID.
	Progress map[string]string `yaml:"progress"`
	// FinishText replaces ui.finishText.
	FinishText string `yaml:"finishText"`
	// License replaces ui.license.
	License string `yaml:"license"`
}

// LanguageLaunch holds the texts of the launch check box in one language.
type LanguageLaunch struct {
	// Text replaces ui.launch.text; "&" marks the access key.
	Text string `yaml:"text"`
}

// LanguageDialog holds the texts of one ui.dialogs page in one language. A
// text left out keeps the base language's.
type LanguageDialog struct {
	// Fields replaces the texts of the page's fields, by property.
	Fields      map[string]*LanguageField `yaml:"fields"`
	Title       string                    `yaml:"title"`
	Description string                    `yaml:"description"`
}

// LanguageField holds the texts of one ui.dialogs field in one language.
type LanguageField struct {
	// Options replaces the labels of a choice field's options, by value.
	Options map[string]string `yaml:"options"`
	Label   string            `yaml:"label"`
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
		v.languageLaunch(cfg, path+".launch", entry.Launch)
		v.languageDialogs(cfg, path+".dialogs", entry.Dialogs)
		v.languageProgress(cfg, path+".progress", entry.Progress)
	}
}

// languageProgress checks the progress texts of a ui.languages entry: each
// key names a custom action that has a progress text in the base language.
func (v *validator) languageProgress(cfg *Config, path string, texts map[string]string) {
	for _, actionID := range slices.Sorted(maps.Keys(texts)) {
		textPath := path + "." + actionID

		index := slices.IndexFunc(cfg.CustomActions, func(a *CustomAction) bool { return a != nil && a.origin == "" && a.ID == actionID })

		switch {
		case index < 0:
			v.errorf(textPath, "%s is not the id of a custom action", actionID)
		case cfg.CustomActions[index].Progress == "":
			v.errorf(textPath, "custom action %s has no progress text to translate", actionID)
		case texts[actionID] == "":
			v.errorf(textPath, "the text is empty; leave the key out to keep the base language's")
		default:
			v.checkProgress(textPath, texts[actionID])
		}
	}
}

// languageLaunch checks the launch texts of a ui.languages entry: they need
// ui.launch, and a text that is set must not be blank.
func (v *validator) languageLaunch(cfg *Config, path string, launch *LanguageLaunch) {
	if launch == nil {
		return
	}

	if cfg.UI.Launch == nil {
		v.errorf(path, "there is no ui.launch check box to translate")
	}

	v.notBlank(path+".text", launch.Text)
}

// languageDialogs checks the dialog texts of a ui.languages entry: each key
// names a ui.dialogs page, each field key a property that one field of
// that page binds, and each option key a value of that choice field.
func (v *validator) languageDialogs(cfg *Config, path string, dialogs map[string]*LanguageDialog) {
	for _, dialogID := range slices.Sorted(maps.Keys(dialogs)) {
		dialogPath := path + "." + dialogID

		index := slices.IndexFunc(cfg.UI.Dialogs, func(d *Dialog) bool { return d != nil && d.ID == dialogID })
		if index < 0 {
			v.errorf(dialogPath, "%s is not the id of a ui.dialogs page", dialogID)

			continue
		}

		entry := dialogs[dialogID]
		if entry == nil {
			continue
		}

		v.notBlank(dialogPath+".title", entry.Title)
		v.notBlank(dialogPath+".description", entry.Description)

		for _, property := range slices.Sorted(maps.Keys(entry.Fields)) {
			v.languageField(fmt.Sprintf("%s.fields.%s", dialogPath, property), cfg.UI.Dialogs[index], property, entry.Fields[property])
		}
	}
}

// languageField checks the texts of a ui.languages entry for the field of
// dialog that binds property.
func (v *validator) languageField(path string, dialog *Dialog, property string, entry *LanguageField) {
	var field *DialogField

	for i := range dialog.Fields {
		if dialog.Fields[i].Property != property {
			continue
		}

		if field != nil {
			v.errorf(path, "two fields of %s bind %s, so the key cannot tell them apart", dialog.ID, property)

			return
		}

		field = &dialog.Fields[i]
	}

	if field == nil {
		v.errorf(path, "no field of %s binds %s", dialog.ID, property)

		return
	}

	if entry == nil {
		return
	}

	v.notBlank(path+".label", entry.Label)
	v.languageOptions(path+".options", field, entry.Options)
}

// languageOptions checks the option labels of a ui.languages entry for
// field: keys that are values of a choice field, and labels that fit.
func (v *validator) languageOptions(path string, field *DialogField, options map[string]string) {
	if len(options) > 0 && field.Type != FieldChoice {
		v.errorf(path, "only a choice field has options")

		return
	}

	for _, value := range slices.Sorted(maps.Keys(options)) {
		optionPath := path + "." + value

		switch label := options[value]; {
		case !slices.ContainsFunc(field.Options, func(o ChoiceOption) bool { return o.Value == value }):
			v.errorf(optionPath, "%q is not an option of the field", value)
		case label != "" && strings.TrimSpace(label) == "":
			v.errorf(optionPath, "the text is blank; leave it out to keep the base language's")
		case utf8.RuneCountInString(label) > maxChoiceText:
			v.errorf(optionPath, "at most %d characters", maxChoiceText)
		}
	}
}

// notBlank reports a text at path that is set but only blanks. An empty
// text is not set: the base language's text stays.
func (v *validator) notBlank(at, text string) {
	if text != "" && strings.TrimSpace(text) == "" {
		v.errorf(at, "the text is blank; leave it out to keep the base language's")
	}
}
