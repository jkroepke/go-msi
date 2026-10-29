//go:build windows

package nativetest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// languagesConfig is the full UI configuration with German and French as
// further wizard languages, in the code page they share.
func languagesConfig(t *testing.T) string {
	t.Helper()

	const anchor = "ui:\n  wizard: featureTree\n"

	if strings.Count(fullUIConfig, anchor) != 1 {
		t.Fatalf("the full UI configuration no longer holds %q once", anchor)
	}

	return strings.Replace(fullUIConfig, anchor, anchor+"  languages:\n    de: {finishText: 'Viel Spaß mit [ProductName].'}\n    fr: {}\n", 1)
}

// TestLanguagesInstallLANG03 installs a package with embedded language
// transforms built from ui.languages and checks the language msiexec
// registers: the base language on the runner's own regional format unless
// that is German or French, and the one ProductLanguage selects.
func TestLanguagesInstallLANG03(t *testing.T) { //nolint:paralleltest // one msiexec at a time
	if testing.Short() {
		t.Skip("msiexec install")
	}

	waitForMsiexec(t)

	dir := t.TempDir()
	pkg, res := buildFullUIFrom(t, dir, languagesConfig(t), "1.0.0", "auto")

	db := openDB(t, pkg)
	template := summaryString(t, db, pidTemplate)
	closeHandle(db)

	if !strings.HasSuffix(template, ";1033,1031,1036") {
		t.Fatalf("Template %q, want the base language, then 1031 and 1036", template)
	}

	for _, language := range []string{"1036", "1031"} {
		log := filepath.Join(dir, "install-"+language+".log")
		if code := msiexec(t, "/i", pkg, "/qn", "ProductLanguage="+language, "/l*v", log); code != 0 {
			t.Fatalf("LANG-03: install with ProductLanguage=%s: exit %d\n%s", language, code, excerpt(readLog(log), "Error", "Transform"))
		}

		got := productInfo(res.ProductCode, "Language")
		t.Logf("LANG-03: ProductLanguage=%s registered Language %q", language, got)

		if code := msiexec(t, "/x", res.ProductCode, "/qn", "/l*v", filepath.Join(dir, "uninstall-"+language+".log")); code != 0 {
			t.Errorf("uninstall: exit %d", code)
		}

		if got != language {
			t.Errorf("LANG-03: ProductLanguage=%s registered Language %q", language, got)
		}
	}
}

// TestFullUILanguages runs the German wizard of a package whose German
// texts come from the built-in translation, selected by ProductLanguage on
// an English system (LANG-03).
func TestFullUILanguages(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	requireFullUI(t)
	waitForMsiexec(t)

	dir := t.TempDir()
	pkg, res := buildFullUIFrom(t, dir, languagesConfig(t), "1.0.0", "auto")

	cleanupFullUI(t, pkg)

	log := filepath.Join(dir, "install.log")
	code, seen := fullUIDriven(t, dir, "", "^(Weiter|Installieren|Fertig stellen)$", "/i", pkg, "ProductLanguage=1031", "/l*v", log)

	if code != 0 {
		t.Errorf("full UI install: exit %d, want 0", code)
	}

	for _, want := range []string{"Willkommen beim Setup-Assistenten für MsiBuilderFullUI", "Zurück", "Viel Spaß mit MsiBuilderFullUI."} {
		if !strings.Contains(seen, want) {
			t.Errorf("no window showed %q", want)
		}
	}

	if !productInstalled(t, res.ProductCode) {
		t.Error("the package is not installed")
	}

	t.Logf("LANG-03 UI:\n%s", seen)
}

// TestLanguageOnlyLicenseInstall installs, with the German transform, a
// package whose license page only the German entry has. The base carries
// an empty CheckBox table, since msi-builder's transform writer adds rows,
// not tables.
func TestLanguageOnlyLicenseInstall(t *testing.T) { //nolint:paralleltest // one msiexec at a time
	if testing.Short() {
		t.Skip("msiexec install")
	}

	waitForMsiexec(t)

	const anchor = "ui:\n  wizard: featureTree\n"

	if strings.Count(fullUIConfig, anchor) != 1 {
		t.Fatalf("the full UI configuration no longer holds %q once", anchor)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "LIZENZ.txt"), []byte("Lizenz nur auf Deutsch\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	doc := strings.Replace(fullUIConfig, anchor, anchor+"  languages:\n    de: {license: LIZENZ.txt}\n", 1)
	pkg, res := buildFullUIFrom(t, dir, doc, "1.0.0", "auto")

	log := filepath.Join(dir, "install.log")
	if code := msiexec(t, "/i", pkg, "/qn", "ProductLanguage=1031", "/l*v", log); code != 0 {
		t.Fatalf("install with ProductLanguage=1031: exit %d\n%s", code, excerpt(readLog(log), "Error", "Transform"))
	}

	got := productInfo(res.ProductCode, "Language")

	if code := msiexec(t, "/x", res.ProductCode, "/qn", "/l*v", filepath.Join(dir, "uninstall.log")); code != 0 {
		t.Errorf("uninstall: exit %d", code)
	}

	if got != "1031" {
		t.Errorf("registered Language %q, want 1031", got)
	}
}
