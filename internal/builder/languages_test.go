package builder_test

import (
	"io"
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

	for _, name := range []string{"Property", "Control", "Dialog", "UIText", "RadioButton", "CheckBox", "File", "Component", "_Validation"} {
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
// that the language transform, whose writer adds no tables, only adds rows.
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

	if got := transformTables(t, res.Database.Storages[0]); !slices.Contains(got, "Dialog") || !slices.Contains(got, "Control") || !slices.Contains(got, "CheckBox") {
		t.Errorf("storage 1031 changes %v, want the license dialog, its controls and its check box", got)
	}
}

// storageStrings returns the string data of a language transform storage.
func storageStrings(t *testing.T, storage cfb.Storage) string {
	t.Helper()

	encoded, err := msidb.EncodeStreamName(msidb.TableStringData, true)
	if err != nil {
		t.Fatal(err)
	}

	for _, stream := range storage.Streams {
		if stream.Name != encoded {
			continue
		}

		reader, err := stream.Open()
		if err != nil {
			t.Fatal(err)
		}

		data, err := io.ReadAll(reader)
		_ = reader.Close()

		if err != nil {
			t.Fatal(err)
		}

		return string(data)
	}

	t.Fatalf("storage %s has no string data", storage.Name)

	return ""
}

// TestLanguageDialogTexts translates the texts of an added dialog and of
// the launch check box in a ui.languages entry: the transform carries them,
// and the base keeps the configuration's own.
func TestLanguageDialogTexts(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.UI.Dialogs = []*config.Dialog{{
		ID: "ServerDlg", Title: "Server connection", Description: "Where the viewer reads its data.", After: "WelcomeDlg",
		Fields: []config.DialogField{
			{Property: "SERVER_URL", Label: "Server address:"},
			{Property: "LOG_LEVEL", Label: "Log level:", Type: config.FieldChoice, Options: []config.ChoiceOption{
				{Value: "warn", Label: "&Warnings"}, {Value: "debug"},
			}},
		},
	}}
	cfg.UI.Languages = map[string]*config.Language{
		"fr": {
			Launch: &config.LanguageLaunch{Text: "&Ouvrir le visionneur"},
			Dialogs: map[string]*config.LanguageDialog{"ServerDlg": {
				Title: "Serveur distant",
				Fields: map[string]*config.LanguageField{
					"SERVER_URL": {Label: "Nom du serveur :"},
					"LOG_LEVEL":  {Options: map[string]string{"debug": "&Detaille"}},
				},
			}},
		},
	}

	res := mustBuild(t, cfg)

	if len(res.Database.Storages) != 1 {
		t.Fatalf("%d storages, want one for fr", len(res.Database.Storages))
	}

	texts := storageStrings(t, res.Database.Storages[0])
	for _, want := range []string{"&Ouvrir le visionneur", "Serveur distant", "Nom du serveur :", "&Detaille"} {
		if !strings.Contains(texts, want) {
			t.Errorf("the French transform does not hold %q", want)
		}
	}

	// Texts the entry leaves out keep the base language's, so the
	// transform does not need them.
	for _, kept := range []string{"Where the viewer reads its data.", "Log level:", "&Warnings"} {
		if strings.Contains(texts, kept) {
			t.Errorf("the French transform holds the untranslated %q", kept)
		}
	}

	title := cell(t, res.Database, "Control", "Text", func(r []any) bool { return r[0] == "ServerDlg" && r[1] == "Title" })
	if s, ok := title.(string); !ok || !strings.Contains(s, "Server connection") {
		t.Errorf("the base title is %v, want the configuration's", title)
	}

	if cfg.UI.Dialogs[0].Title != "Server connection" || cfg.UI.Launch.Text != "" {
		t.Error("the language build changed the configuration")
	}
}
