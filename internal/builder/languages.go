package builder

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/summary"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// languageUI returns ui with the wizard texts of the language this build
// writes (config.WizardTexts).
func (b *build) languageUI() *config.UI {
	ui := b.cfg.UI
	ui.Strings = b.cfg.WizardTexts(b.language)

	return &ui
}

// languageIDs returns the LCIDs of ui.languages, sorted.
func (b *build) languageIDs() []int {
	return slices.Sorted(maps.Keys(b.cfg.UI.LanguageIDs()))
}

// template is the summary Template: the platform, the base language and
// then every language of ui.languages. msiexec applies the transform of the
// user's language and installs the base language when the first entry
// matches (spec section 7.6, LANG-01).
func (b *build) template() string {
	ids := b.languageIDs()
	langs := make([]string, 0, 1+len(ids))
	langs = append(langs, strconv.Itoa(*b.cfg.Product.Language))

	for _, lcid := range ids {
		langs = append(langs, strconv.Itoa(lcid))
	}

	return string(b.cfg.Package.Platform) + ";" + strings.Join(langs, ",")
}

// languageTransforms builds the package once more for each language of
// ui.languages and embeds the transform from base to that build as a
// storage named by the LCID (spec section 7.6). The builds reuse the
// payload files that the base build read.
func (b *build) languageTransforms(base *msidb.Database) error {
	keys := b.cfg.UI.LanguageIDs()

	for _, lcid := range b.languageIDs() {
		path := "ui.languages." + keys[lcid]

		variant, ok := b.languageBuild(lcid, path, b.cfg.UI.Languages[keys[lcid]])
		if !ok {
			continue
		}

		storage, err := msidb.TransformStorage(strconv.Itoa(lcid), base, variant, msidb.TransformOptions{Summary: b.transformSummary(lcid)})
		if err != nil {
			b.errorf(path, "%v", err)

			continue
		}

		base.Storages = append(base.Storages, storage)
	}

	return b.diags.Err() //nolint:wrapcheck // the diagnostics carry their own paths
}

// entryPaths map the configuration paths that a ui.languages entry
// replaces to the entry's own keys, so a diagnostic of the language build
// names the entry's field.
var entryPaths = []struct{ from, to string }{ //nolint:gochecknoglobals // lookup table
	{"ui.strings", ".strings"},
	{"ui.finishText", ".finishText"},
	{"ui.license", ".license"},
}

// languageBuild builds the tables of the package in language lcid: the
// configuration with the entry's finish text, license, launch and dialog
// texts, the base build's
// product code and its payload files. It adds the diagnostics that the
// base build did not report, under the entry's path, and reports whether
// the build succeeded.
func (b *build) languageBuild(lcid int, path string, entry *config.Language) (*msidb.Database, bool) {
	cfg := *b.cfg
	cfg.Product.ProductCode = b.productCode

	if entry != nil {
		if entry.FinishText != "" {
			cfg.UI.FinishText = entry.FinishText
		}

		if entry.License != "" {
			cfg.UI.License = entry.License
		}

		if entry.Launch != nil && entry.Launch.Text != "" && cfg.UI.Launch != nil {
			launch := *cfg.UI.Launch
			launch.Text = entry.Launch.Text
			cfg.UI.Launch = &launch
		}

		cfg.UI.Dialogs = translatedDialogs(cfg.UI.Dialogs, entry.Dialogs)
	}

	variant := newBuild(&cfg, b.opts, nil)
	variant.language = lcid
	variant.fileCache = b.fileCache

	err := variant.writeTables()

	var database *msidb.Database
	if err == nil {
		database, err = variant.assemble()
	}

	b.addLanguageDiagnostics(path, variant.diags)

	if err != nil && variant.diags.Err() == nil {
		b.errorf(path, "language %d: %v", lcid, err)
	}

	return database, err == nil
}

// translatedDialogs returns copies of dialogs with the texts that a
// ui.languages entry replaces; the base configuration stays unchanged.
func translatedDialogs(dialogs []*config.Dialog, texts map[string]*config.LanguageDialog) []*config.Dialog {
	if len(texts) == 0 {
		return dialogs
	}

	out := make([]*config.Dialog, len(dialogs))

	for i, dialog := range dialogs {
		entry := texts[dialog.ID]
		if entry == nil {
			out[i] = dialog

			continue
		}

		translated := *dialog
		translated.Title = cmp.Or(entry.Title, dialog.Title)
		translated.Description = cmp.Or(entry.Description, dialog.Description)
		translated.Fields = make([]config.DialogField, len(dialog.Fields))

		for j, field := range dialog.Fields {
			translated.Fields[j] = translatedField(field, entry.Fields[field.Property])
		}

		out[i] = &translated
	}

	return out
}

// translatedField returns field with the label and option labels of entry.
func translatedField(field config.DialogField, entry *config.LanguageField) config.DialogField {
	if entry == nil {
		return field
	}

	field.Label = cmp.Or(entry.Label, field.Label)
	field.Options = slices.Clone(field.Options)

	for k, option := range field.Options {
		field.Options[k].Label = cmp.Or(entry.Options[option.Value], option.Label)
	}

	return field
}

// addLanguageDiagnostics adds the diagnostics of the build of the
// ui.languages entry at path that the base build did not report, with the
// paths of the fields the entry replaces moved under the entry.
func (b *build) addLanguageDiagnostics(path string, diags config.Diagnostics) {
	for _, diag := range diags {
		if slices.Contains(b.diags, diag) {
			continue
		}

		for _, p := range entryPaths {
			if rest, ok := strings.CutPrefix(diag.Path, p.from); ok {
				diag.Path = path + p.to + rest

				break
			}
		}

		b.diags = append(b.diags, diag)
	}
}

// transformSummary is the summary information of the transform to lcid,
// as msi.dll writes it for a transform without validation (MSIDB-12):
// the base's Template, the platform and the variant's language, and the
// product codes and versions, which a language transform does not change.
func (b *build) transformSummary(lcid int) []summary.Property {
	code := b.productCode + b.cfg.Product.Version

	return []summary.Property{
		{ID: summary.PIDCodepage, Value: config.SummaryCodepage(b.cfg.Package.Codepage)},
		{ID: summary.PIDTitle, Value: "Transform"},
		{ID: summary.PIDTemplate, Value: b.template()},
		{ID: summary.PIDLastAuthor, Value: fmt.Sprintf("%s;%d", b.cfg.Package.Platform, lcid)},
		{ID: summary.PIDRevision, Value: code + ";" + code + ";" + b.upgradeCode},
		{ID: summary.PIDPageCount, Value: 200},
		{ID: summary.PIDCharCount, Value: 0},
		{ID: summary.PIDSecurity, Value: 2},
	}
}
