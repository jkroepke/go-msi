package main

// LANG-01: how the engine picks an embedded language transform. The
// transforms are made with msi.dll's own MsiDatabaseGenerateTransformW and
// MsiCreateTransformSummaryInfoW and embedded through the _Storages table,
// so no msi-builder transform or storage writer is involved.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"
	"unsafe"
)

var (
	pMsiDatabaseGenerateTransformW = msiDLL.NewProc("MsiDatabaseGenerateTransformW")
	pMsiCreateTransformSummaryInfo = msiDLL.NewProc("MsiCreateTransformSummaryInfoW")
	pMsiGetProductInfoW            = msiDLL.NewProc("MsiGetProductInfoW")

	kernel32Lang                = syscall.NewLazyDLL("kernel32.dll")
	pGetUserDefaultUILanguage   = kernel32Lang.NewProc("GetUserDefaultUILanguage")
	pGetSystemDefaultUILanguage = kernel32Lang.NewProc("GetSystemDefaultUILanguage")
	pGetUserDefaultLangID       = kernel32Lang.NewProc("GetUserDefaultLangID")
	pGetSystemDefaultLangID     = kernel32Lang.NewProc("GetSystemDefaultLangID")
)

func init() { cases["LANG-01"] = caseLANG01 }

// productInfo returns an installed product's property, or the error code.
func productInfo(code, prop string) string {
	buf := make([]uint16, 512)
	n := uint32(len(buf))
	r, _, _ := pMsiGetProductInfoW.Call(uintptr(unsafe.Pointer(wstr(code))), uintptr(unsafe.Pointer(wstr(prop))),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return fmt.Sprintf("error %d", r)
	}
	return syscall.UTF16ToString(buf[:n])
}

// langLines are the log lines that show which transform and language the
// engine took.
var langLines = regexp.MustCompile(`(?im)^[^\r\n]*(transform|Property\([CS]\): ProductName =|Property\([CS]\): ProductLanguage =|Property\([CS]\): UserLanguageID =|Property\([CS]\): SystemLanguageID =|Error 2203|Note: 1: 2203|MsiSpec Test lang01)[^\r\n]*$`)

// logLanguages records the languages Windows reports, which the engine may
// match against the Template list.
func logLanguages(c *caseLog) {
	for _, p := range []struct {
		name string
		proc *syscall.LazyProc
	}{
		{"GetUserDefaultUILanguage", pGetUserDefaultUILanguage},
		{"GetSystemDefaultUILanguage", pGetSystemDefaultUILanguage},
		{"GetUserDefaultLangID", pGetUserDefaultLangID},
		{"GetSystemDefaultLangID", pGetSystemDefaultLangID},
	} {
		r, _, _ := p.proc.Call()
		c.logf("%s = %d (0x%04x)", p.name, uint16(r), uint16(r))
	}

	ps := "Get-WinSystemLocale | Select-Object -ExpandProperty Name; (Get-Culture).Name; (Get-UICulture).Name; " +
		"Get-WinUserLanguageList | ForEach-Object { $_.LanguageTag }; Get-WinUILanguageOverride"
	c.logf("PowerShell (system locale, culture, UI culture, user languages, UI override):")
	for _, line := range strings.Split(strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)), "\n") {
		c.logf("  %s", strings.TrimSpace(line))
	}
}

// langVariant copies base to path and sets the ProductLanguage and
// ProductName that a language transform will carry.
func langVariant(base, path, language, name string) error {
	data, err := os.ReadFile(base)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	db, err := openDB(path, dbTransact)
	if err != nil {
		return err
	}
	defer closeHandle(db)
	for prop, value := range map[string]string{"ProductLanguage": language, "ProductName": name} {
		if _, err := query(db, fmt.Sprintf("UPDATE `Property` SET `Value`='%s' WHERE `Property`='%s'", value, prop)); err != nil {
			return fmt.Errorf("%s: %w", prop, err)
		}
	}
	return commit(db)
}

// makeTransform writes the transform from reference to variant into mst
// with its summary information (no error conditions, no validation).
func makeTransform(variant, reference, mst string) error {
	db, err := openDB(variant, dbReadOnly)
	if err != nil {
		return err
	}
	defer closeHandle(db)
	ref, err := openDB(reference, dbReadOnly)
	if err != nil {
		return err
	}
	defer closeHandle(ref)
	os.Remove(mst)
	if err := call("MsiDatabaseGenerateTransformW", pMsiDatabaseGenerateTransformW, uintptr(db), uintptr(ref),
		uintptr(unsafe.Pointer(wstr(mst))), 0, 0); err != nil {
		return err
	}
	return call("MsiCreateTransformSummaryInfoW", pMsiCreateTransformSummaryInfo, uintptr(db), uintptr(ref),
		uintptr(unsafe.Pointer(wstr(mst))), 0, 0)
}

// langPackage copies base to path, embeds the transforms as substorages
// named by their language and sets the Template.
func langPackage(base, path, template string, storages map[string]string) error {
	data, err := os.ReadFile(base)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	db, err := openDB(path, dbTransact)
	if err != nil {
		return err
	}
	defer closeHandle(db)
	for name, mst := range storages {
		if err := insertStream(db, "_Storages", name, mst); err != nil {
			return fmt.Errorf("_Storages %s: %w", name, err)
		}
	}
	h, err := openSummary(db, "", 1)
	if err != nil {
		return err
	}
	defer closeHandle(h)
	if err := setSummary(h, sumProp{PID: 7, VT: vtLPSTR, Text: template}); err != nil {
		return err
	}
	if err := persistSummary(h); err != nil {
		return err
	}
	return commit(db)
}

func caseLANG01() {
	c := newCase("LANG-01", "embedded-language-transform")
	defer c.close()
	logLanguages(c)

	const code = "{6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D}"
	base := control(c, &pkg{Name: "lang01 base", Is64: true, ProductCode: code}, "base.msi")
	if base == "" {
		return
	}

	de, en := c.path("de.msi"), c.path("en.msi")
	mstDE, mstEN := c.path("1031.mst"), c.path("1033.mst")
	for _, step := range []struct {
		variant, mst, language, name string
	}{
		{de, mstDE, "1031", "MsiSpec Test lang01 DE"},
		{en, mstEN, "1033", "MsiSpec Test lang01 EN"},
	} {
		if err := langVariant(base, step.variant, step.language, step.name); err != nil {
			c.logf("variant %s: %v", step.language, err)
			return
		}
		if err := makeTransform(step.variant, base, step.mst); err != nil {
			c.logf("transform %s: %v", step.language, err)
			return
		}
		c.logf("transform %s: %s (ProductLanguage %s, ProductName %q) sha256 %s", step.language, step.mst, step.language, step.name, shaFile(step.mst))
	}

	both := map[string]string{"1031": mstDE, "1033": mstEN}
	for i, v := range []struct {
		template string
		storages map[string]string
		args     []string
	}{
		{"x64;1033,1031", both, nil},
		{"x64;1031,1033", both, nil},
		{"x64;1033,1031", map[string]string{"1031": mstDE}, nil},
		{"x64;1031,1033", map[string]string{"1031": mstDE}, nil},
		{"x64;1033,1031", nil, nil},
		{"x64;1033,1031", both, []string{"ProductLanguage=1033"}},
		{"x64;1033,1031", both, []string{"ProductLanguage=1031"}},
		{"x64;1033", both, []string{"TRANSFORMS=:1031"}},
		{"x64;1033,1031", both, []string{"TRANSFORMS=:1033"}},
		{"x64;1033", both, nil},
		// The user's language 1033 is not listed: a sublanguage of it,
		// and then none of its primary language.
		{"x64;1031,2057", map[string]string{"1031": mstDE, "2057": mstEN}, nil},
		{"x64;1031,1036", map[string]string{"1031": mstDE, "1036": mstEN}, nil},
		{"x64;1036,1031", map[string]string{"1031": mstDE, "1036": mstEN}, nil},
	} {
		names := make([]string, 0, len(v.storages))
		for name := range v.storages {
			names = append(names, name)
		}
		label := fmt.Sprintf("v%02d", i+1)
		c.logf("## %s: Template %q, storages %v, arguments %q", label, v.template, names, v.args)
		path := c.path(label + ".msi")
		if err := langPackage(base, path, v.template, v.storages); err != nil {
			c.logf("BUILD FAILED: %v", err)
			continue
		}
		args := append([]string{"/i", path}, v.args...)
		r := runMsiexec(c, label+"-install.log", args...)
		for _, line := range langLines.FindAllString(r.Log, 40) {
			c.logf("  lang: %s", strings.TrimSpace(line))
		}
		c.logf("RESULT %s: Template %q storages %v args %q -> install %d", label, v.template, names, v.args, r.Exit)
		if r.Exit == 0 || r.Exit == 3010 {
			c.logf("%s: installed ProductName %q, Language %q, Transforms %q", label,
				productInfo(code, "ProductName"), productInfo(code, "Language"), productInfo(code, "Transforms"))
			u := runMsiexec(c, label+"-uninstall.log", "/x", code)
			c.logf("%s: uninstall %d", label, u.Exit)
		}
	}
}
