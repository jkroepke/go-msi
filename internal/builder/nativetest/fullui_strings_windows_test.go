//go:build windows

package nativetest_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// germanStrings translate the pages a new installation passes through and
// the texts the test checks; the rest stays English.
const germanStrings = `  strings:
    DialogTitle: '[ProductName] Setup'
    Back: '< &Zurück'
    Next: '&Weiter >'
    Cancel: Abbrechen
    Install: '&Installieren'
    Finish: '&Fertig stellen'
    WelcomeTitle: 'Willkommen beim Setup-Assistenten für [ProductName]'
    WelcomeText: 'Der Setup-Assistent installiert [ProductName] auf Ihrem Computer. Klicken Sie auf Weiter, um fortzufahren.'
    ReadyInstallTitle: 'Bereit zur Installation von [ProductName]'
    ReadyInstallText: 'Klicken Sie auf Installieren, um die Installation zu starten.'
    ProgressTitle: '[ProductName] wird installiert'
    ProgressText: 'Bitte warten Sie, während der Setup-Assistent [ProductName] installiert.'
    ProgressStatus: 'Status:'
    ExitTitle: 'Der Setup-Assistent für [ProductName] wurde abgeschlossen'
    ExitText: 'Klicken Sie auf Fertig stellen, um den Setup-Assistenten zu beenden.'
`

// russianStrings need a UTF-8 database: Cyrillic and Japanese share no ANSI
// code page.
const russianStrings = `  strings:
    Back: '< &Назад'
    Next: '&Далее >'
    Cancel: Отмена
    Install: '&Установить'
    Finish: '&Готово'
    WelcomeTitle: '[ProductName] セットアップへようこそ'
    ReadyInstallTitle: 'Всё готово к установке [ProductName]'
    ExitTitle: 'Установка [ProductName] завершена'
`

// TestFullUIStrings installs packages whose wizard texts ui.strings
// translated, clicking through by the translated button names (UI-06): a
// German wizard in code page 1252 with product language 1031, and a
// Russian and Japanese one in a UTF-8 database, both on an English system.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestFullUIStrings(t *testing.T) {
	requireFullUI(t)

	for _, tc := range []struct {
		name, language, codepage, strings, advance string
		want                                       []string
	}{
		{
			"german", "1031", "1252", germanStrings, "^(Weiter|Installieren|Fertig stellen)$",
			[]string{"Willkommen beim Setup-Assistenten für MsiBuilderFullUI", "Zurück", "Der Setup-Assistent für MsiBuilderFullUI wurde abgeschlossen"},
		},
		{
			"utf8", "1049", "65001", russianStrings, "^(Далее|Установить|Готово)$",
			[]string{"MsiBuilderFullUI セットアップへようこそ", "Назад", "Установка MsiBuilderFullUI завершена"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			waitForMsiexec(t)

			doc := translatedConfig(t, tc.language, tc.codepage, tc.strings)
			dir := t.TempDir()
			pkg, res := buildFullUIFrom(t, dir, doc, "1.0.0", "auto")

			cleanupFullUI(t, pkg)

			log := filepath.Join(dir, "install.log")
			code, seen := fullUIDriven(t, dir, "", tc.advance, "/i", pkg, "/l*v", log)
			text := readLog(log)

			if code != 0 {
				t.Errorf("full UI install: exit %d, want 0", code)
			}

			for _, want := range tc.want {
				if !strings.Contains(seen, want) {
					t.Errorf("no window showed %q", want)
				}
			}

			if strings.Contains(seen, "UNEXPECTED:") {
				t.Error("the engine showed an unexpected error box")
			}

			if !productInstalled(t, res.ProductCode) {
				t.Error("the package is not installed")
			}

			t.Logf("UI-06 %s UI:\n%s\nlog excerpt:\n%s", tc.name, seen, excerpt(text, "ProductLanguage", "Error 28", "error code is"))
		})
	}

	// The language is part of the configuration, so each language has its
	// own ProductCode; the same version of another language replaces the
	// installed one through the major upgrade.
	t.Run("switch", func(t *testing.T) {
		waitForMsiexec(t)

		dir := t.TempDir()
		german, germanRes := buildFullUIFrom(t, dir, translatedConfig(t, "1031", "1252", germanStrings), "1.0.0", "auto")
		utf8, utf8Res := buildFullUIFrom(t, t.TempDir(), translatedConfig(t, "1049", "65001", russianStrings), "1.0.0", "auto")

		cleanupFullUI(t, utf8, german)

		if germanRes.ProductCode == utf8Res.ProductCode {
			t.Fatalf("both languages have ProductCode %s", germanRes.ProductCode)
		}

		if code := msiexec(t, "/i", german, "/qn", "/l*v", filepath.Join(dir, "german.log")); code != 0 {
			t.Fatalf("installing the German package: exit %d", code)
		}

		log := filepath.Join(dir, "switch.log")
		if code := msiexec(t, "/i", utf8, "/qn", "/l*v", log); code != 0 {
			t.Errorf("installing the UTF-8 package over the German one: exit %d", code)
		}

		if productInstalled(t, germanRes.ProductCode) {
			t.Error("the German package is still installed")
		}

		if !productInstalled(t, utf8Res.ProductCode) {
			t.Error("the UTF-8 package is not installed")
		}

		t.Logf("UI-06 switch log excerpt:\n%s", excerpt(readLog(log), "FindRelatedProducts", "RemoveExistingProducts", "Product:"))
	})
}

// translatedConfig is the full UI configuration with another product
// language, database code page and ui.strings.
func translatedConfig(t *testing.T, language, codepage, texts string) string {
	t.Helper()

	doc := fullUIConfig

	for _, edit := range [][2]string{
		{"  language: 1033\n", "  language: " + language + "\n"},
		{"  installerVersion: \"5.0\"\n", "  installerVersion: \"5.0\"\n  codepage: " + codepage + "\n"},
		{"ui:\n  wizard: featureTree\n", "ui:\n  wizard: featureTree\n" + texts},
	} {
		if strings.Count(doc, edit[0]) != 1 {
			t.Fatalf("the full UI configuration no longer holds %q once", edit[0])
		}

		doc = strings.Replace(doc, edit[0], edit[1], 1)
	}

	return doc
}
