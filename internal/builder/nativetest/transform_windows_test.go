//go:build windows

package nativetest_test

// LANG-02: a language transform written by msidb.MarshalTransform, applied
// by msi.dll and embedded as a storage that msiexec selects.

import (
	"bytes"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/cfb"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/summary"
)

//nolint:gochecknoglobals // lazily bound msi.dll procedures
var (
	pMsiDatabaseApplyTransformW = msiDLL.NewProc("MsiDatabaseApplyTransformW")
	pMsiGetProductInfoW         = msiDLL.NewProc("MsiGetProductInfoW")
)

const langProductName = "Minimal Beispiel Zurück"

// setProperty replaces or adds a Property row.
func setProperty(db *msidb.Database, name, value string) {
	tbl := db.Table("Property")
	for _, row := range tbl.Rows {
		if row[0] == name {
			row[1] = value

			return
		}
	}

	tbl.Add(name, value)
}

func property(db *msidb.Database, name string) string {
	for _, row := range db.Table("Property").Rows {
		if row[0] == name {
			if value, ok := row[1].(string); ok {
				return value
			}
		}
	}

	return ""
}

// languagePair returns the minimal example as a base with a LangGone
// property and a German variant of it, decoded twice so they share no rows.
func languagePair(t *testing.T) (*msidb.Database, *msidb.Database) {
	t.Helper()

	data := unsignedMinimal(t)

	base, err := msidb.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	variant, err := msidb.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	setProperty(base, "LangGone", "base only")
	setProperty(variant, "ProductName", langProductName)
	setProperty(variant, "ProductLanguage", "1031")
	setProperty(variant, "LangProbe", "de")

	return base, variant
}

// transformSummary is the summary information msi.dll's
// MsiCreateTransformSummaryInfoW wrote in MSIDB-12, without validation.
func transformSummary(base, variant *msidb.Database) []summary.Property {
	template, ok := summary.Get(base.Summary, summary.PIDTemplate).(string)
	if !ok {
		template = ""
	}

	platform, _, _ := strings.Cut(template, ";")

	return []summary.Property{
		{ID: summary.PIDCodepage, Value: 1252},
		{ID: summary.PIDTitle, Value: "Transform"},
		{ID: summary.PIDTemplate, Value: template},
		{ID: summary.PIDLastAuthor, Value: platform + ";" + property(variant, "ProductLanguage")},
		{ID: summary.PIDRevision, Value: property(base, "ProductCode") + property(base, "ProductVersion") + ";" +
			property(variant, "ProductCode") + property(variant, "ProductVersion") + ";" + property(base, "UpgradeCode")},
		{ID: summary.PIDPageCount, Value: 200},
		{ID: summary.PIDCharCount, Value: 0},
		{ID: summary.PIDSecurity, Value: 2},
	}
}

func productInfo(code, prop string) string {
	buf := make([]uint16, 512)
	size := uint32(len(buf)) //nolint:gosec // 512

	p1, _ := syscall.UTF16PtrFromString(code)
	p2, _ := syscall.UTF16PtrFromString(prop)

	r, _, _ := pMsiGetProductInfoW.Call(uintptr(unsafe.Pointer(p1)), uintptr(unsafe.Pointer(p2)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return ""
	}

	return syscall.UTF16ToString(buf[:size])
}

// TestTransformApplyLANG02 applies a MarshalTransform transform with
// msi.dll and reads the changed rows back, in code page 1252 and, with
// Cyrillic and Japanese text, in 65001. msi.dll set bit 15 of the refcount
// of a non-ASCII string in a 65001 transform (MSIDB-12); MarshalTransform
// does not, which the 65001 case checks.
func TestTransformApplyLANG02(t *testing.T) { //nolint:paralleltest // msi.dll calls
	for _, tc := range []struct {
		name     string
		codepage int
		text     string
	}{
		{"cp1252", 1252, langProductName},
		{"cp65001", 65001, "Пример Zurück 日本"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, variant := languagePair(t)
			base.Codepage, variant.Codepage = tc.codepage, tc.codepage
			setProperty(variant, "ProductName", tc.text)

			mst, err := msidb.MarshalTransform(base, variant, msidb.TransformOptions{Summary: transformSummary(base, variant)})
			if err != nil {
				t.Fatal(err)
			}

			pkg, err := base.Marshal(msidb.Options{})
			if err != nil {
				t.Fatal(err)
			}

			pkgPath := writeTemp(t, "lang02-base-"+tc.name+".msi", pkg)
			mstPath := writeTemp(t, "lang02-"+tc.name+".mst", mst)

			var handle uint32
			if err := call("MsiOpenDatabaseW", pMsiOpenDatabaseW, uintptr(unsafe.Pointer(wstr(t, pkgPath))), 1 /* MSIDBOPEN_TRANSACT */, uintptr(unsafe.Pointer(&handle))); err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() { closeHandle(handle) })

			if err := call("MsiDatabaseApplyTransformW", pMsiDatabaseApplyTransformW, uintptr(handle), uintptr(unsafe.Pointer(wstr(t, mstPath))), 0); err != nil {
				t.Fatalf("LANG-02: msi.dll refuses the transform: %v", err)
			}

			got := map[string]string{}
			for _, row := range query(t, handle, "SELECT `Property`, `Value` FROM `Property`") {
				got[row[0]] = row[1]
			}

			for name, want := range map[string]string{"ProductName": tc.text, "ProductLanguage": "1031", "LangProbe": "de", "LangGone": ""} {
				if got[name] != want {
					t.Errorf("LANG-02 %s: after the transform %s = %q, want %q", tc.name, name, got[name], want)
				}
			}
		})
	}
}

// TestTransformInstallLANG02 embeds the transform as storage 1031 of a
// package whose Template lists 1033,1031 and installs it with
// ProductLanguage=1031.
func TestTransformInstallLANG02(t *testing.T) { //nolint:paralleltest // one msiexec at a time
	if testing.Short() {
		t.Skip("msiexec install")
	}

	base, variant := languagePair(t)

	mst, err := msidb.MarshalTransform(base, variant, msidb.TransformOptions{Summary: transformSummary(base, variant)})
	if err != nil {
		t.Fatal(err)
	}

	template, ok := summary.Get(base.Summary, summary.PIDTemplate).(string)
	if !ok {
		t.Fatal("no Template")
	}

	platform, _, _ := strings.Cut(template, ";")

	for i := range base.Summary {
		if base.Summary[i].ID == summary.PIDTemplate {
			base.Summary[i].Value = platform + ";1033,1031"
		}
	}

	var plain bytes.Buffer
	if _, err := base.MarshalTo(&plain, msidb.Options{}); err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(plain.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	transform, err := cfb.Read(mst)
	if err != nil {
		t.Fatal(err)
	}

	sources := make([]cfb.Source, 0, len(file.Streams()))
	for _, s := range file.Streams() {
		sources = append(sources, s.Source())
	}

	storage := cfb.Storage{Name: "1031", CLSID: cfb.CLSIDTransform}
	for _, s := range transform.Streams() {
		storage.Streams = append(storage.Streams, s.Source())
	}

	var pkg bytes.Buffer
	if _, err := cfb.WriteStoragesTo(&pkg, sources, []cfb.Storage{storage}, cfb.Options{Version: 4, CLSID: cfb.CLSIDMSI}); err != nil {
		t.Fatal(err)
	}

	pkgPath := writeTemp(t, "lang02.msi", pkg.Bytes())
	code := property(base, "ProductCode")
	logDir := t.TempDir()

	msiexecQuiet(t, filepath.Join(logDir, "install.log"), "/i", pkgPath, "ProductLanguage=1031")

	// Not t.Cleanup: msiexecQuiet runs under t.Context, which is cancelled
	// before cleanups run.
	defer msiexecQuiet(t, filepath.Join(logDir, "uninstall.log"), "/x", code)

	name, language := productInfo(code, "ProductName"), productInfo(code, "Language")
	t.Logf("LANG-02: installed ProductName %q, Language %q", name, language)

	if name != langProductName || language != "1031" {
		t.Errorf("LANG-02: installed ProductName %q, Language %q, want %q and 1031\n%s", name, language, langProductName, readLog(filepath.Join(logDir, "install.log")))
	}
}
