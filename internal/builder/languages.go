package builder

import (
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

		variant, err := b.languageBuild(lcid, b.cfg.UI.Languages[keys[lcid]])
		if err != nil {
			b.errorf(path, "%v", err)

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

// languageBuild builds the tables of the package in language lcid: the
// configuration with the entry's finish text and license, the base build's
// product code and its payload files.
func (b *build) languageBuild(lcid int, entry *config.Language) (*msidb.Database, error) {
	cfg := *b.cfg
	cfg.Product.ProductCode = b.productCode

	if entry != nil {
		if entry.FinishText != "" {
			cfg.UI.FinishText = entry.FinishText
		}

		if entry.License != "" {
			cfg.UI.License = entry.License
		}
	}

	variant := newBuild(&cfg, b.opts, nil)
	variant.language = lcid
	variant.fileCache = b.fileCache

	err := variant.writeTables()
	if err == nil {
		var database *msidb.Database

		database, err = variant.assemble()
		if err == nil {
			return database, nil
		}
	}

	if diagErr := variant.diags.Err(); diagErr != nil {
		return nil, diagErr //nolint:wrapcheck // the diagnostics carry their own paths
	}

	return nil, fmt.Errorf("language %d: %w", lcid, err)
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
