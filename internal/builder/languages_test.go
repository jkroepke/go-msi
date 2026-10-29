package builder_test

import (
	"slices"
	"testing"

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
