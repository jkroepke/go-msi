package builder_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/cfb"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/summary"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// transformTables returns the tables a language transform storage changes.
func transformTables(t *testing.T, storage cfb.Storage) []string {
	t.Helper()

	var out []string

	for _, name := range []string{"Property", "Control", "Dialog", "UIText", "RadioButton", "File", "Component", "_Validation"} {
		encoded, err := msidb.EncodeStreamName(name, true)
		if err != nil {
			t.Fatal(err)
		}

		if slices.ContainsFunc(storage.Streams, func(s cfb.Source) bool { return s.Name == encoded }) {
			out = append(out, name)
		}
	}

	return out
}

func TestLanguageTransforms(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.Package.Codepage = 0
	cfg.UI.Languages = map[string]*config.Language{
		"de":   {FinishText: "Viel Spaß mit [ProductName]."},
		"1083": nil,
		"1090": {Strings: map[string]string{"ExitText": "Нажмите кнопку «Готово»."}},
	}

	res := mustBuild(t, cfg)

	if got := res.Database.Codepage; got != 65001 {
		t.Errorf("code page %d, want 65001 for LCIDs outside the table", got)
	}

	if got, want := summary.Get(res.Database.Summary, summary.PIDTemplate), string(cfg.Package.Platform)+";1033,1031,1083,1090"; got != want {
		t.Errorf("Template %v, want %s", got, want)
	}

	names := make([]string, 0, len(res.Database.Storages))
	for _, s := range res.Database.Storages {
		names = append(names, s.Name)

		if s.CLSID != cfb.CLSIDTransform {
			t.Errorf("storage %s: CLSID %x", s.Name, s.CLSID)
		}
	}

	if !slices.Equal(names, []string{"1031", "1083", "1090"}) {
		t.Fatalf("storages %v, want 1031, 1083 and 1090", names)
	}

	// German translates the wizard (its dialog title is the English one).
	// 1083 and 1090 have no built-in translation: 1083 sets no text, so
	// its transform only changes ProductLanguage, and 1090 changes the one
	// text it sets. None touches the files.
	for i, want := range [][]string{{"Property", "Control", "UIText", "RadioButton"}, {"Property"}, {"Property", "Control"}} {
		if got := transformTables(t, res.Database.Storages[i]); !slices.Equal(got, want) {
			t.Errorf("storage %s changes %v, want %v", names[i], got, want)
		}
	}
}

func TestLanguageTransformsInPackage(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.UI.Languages = map[string]*config.Language{"fr": nil}

	res := mustBuild(t, cfg)

	file, err := cfb.Read(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.ContainsFunc(file.Entries, func(e *cfb.Entry) bool { return e.Name == "1036" }) {
		t.Error("the package has no storage 1036")
	}
}

func TestLanguageBuildDiagnostics(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.UI.Languages = map[string]*config.Language{"fr": {License: "missing.txt"}}

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil {
		t.Fatal("a missing license of a language built")
	}

	if !slices.ContainsFunc(res.Diagnostics, func(d config.Diagnostic) bool {
		return d.Severity == config.SeverityError && strings.HasPrefix(d.Path, "ui.languages.fr.license")
	}) {
		t.Errorf("no error under ui.languages.fr.license:\n%s", res.Diagnostics)
	}
}

// TestLanguageLicenseWithoutBaseLicense builds a package whose only license
// belongs to a ui.languages entry: the base has an empty CheckBox table, so
// that the language transform only adds rows.
func TestLanguageLicenseWithoutBaseLicense(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.UI.License = ""
	cfg.UI.Launch = nil
	cfg.UI.Languages = map[string]*config.Language{"de": {License: "LICENSE.txt"}}

	res := mustBuild(t, cfg)

	if res.Database.Table("CheckBox") == nil {
		t.Fatal("the base has no CheckBox table")
	}

	if got := rows(t, res.Database, "CheckBox"); len(got) != 0 {
		t.Errorf("CheckBox rows %v, want none in the base", got)
	}

	if len(res.Database.Storages) != 1 || res.Database.Storages[0].Name != "1031" {
		t.Fatalf("storages %v, want 1031", res.Database.Storages)
	}

	if got := transformTables(t, res.Database.Storages[0]); !slices.Contains(got, "Dialog") || !slices.Contains(got, "Control") {
		t.Errorf("storage 1031 changes %v, want the license dialog and its controls", got)
	}
}
