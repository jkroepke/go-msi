package main

// Runtime cases: upgrades, file versioning, registry views, environment.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	cases["UPG-01"] = caseUPG01
	cases["UPG-02"] = caseUPG02
	cases["UPG-04"] = caseUPG04
	cases["UPG-03"] = caseUPG03
	cases["REG-01"] = caseREG01
	cases["ENVVAR-01"] = caseENVVAR01
	cases["FILE-01"] = caseFILE01
}

func registered(code string) bool {
	out := cmdOut("reg.exe", "query", `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+code, "/reg:64")
	if strings.Contains(out, "DisplayName") {
		return true
	}
	out = cmdOut("reg.exe", "query", `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+code, "/reg:32")
	return strings.Contains(out, "DisplayName")
}

func removeProduct(c *caseLog, code string) {
	if registered(code) {
		r := runMsiexec(c, "cleanup-"+strings.Trim(code, "{}")+".log", "/x", code)
		c.logf("  cleanup %s -> %d", code, r.Exit)
	}
}

const upgradeUC = "{8C3B7A51-6E2D-4F1A-9B0C-2D4E6F8A1B3C}"

// upgradePkg builds a package of the "upg" family with major-upgrade rows.
func upgradePkg(c *caseLog, file, version string, rep string, extra func(p *pkg)) (*pkg, string) {
	p := &pkg{Name: "upg", Is64: true, Version: version, UpgradeCode: upgradeUC}
	p.Extra = map[string][][]string{
		"Upgrade": {
			{upgradeUC, "", version, "", "256", "", "UPGRADEFOUND"},
			{upgradeUC, version, "", "", "2", "", "NEWERFOUND"},
		},
	}
	p.ExtraProps = [][]string{{"SecureCustomProperties", "NEWERFOUND;UPGRADEFOUND"}}
	p.ExtraSequence = [][]string{{"InstallExecute", "", "6500"}}
	if rep != "" {
		p.ExtraSequence = append(p.ExtraSequence, []string{"RemoveExistingProducts", "", rep})
	}
	if extra != nil {
		extra(p)
	}
	return p, control(c, p, file)
}

func caseUPG01() {
	for _, v := range []struct{ name, first, second string }{
		{"higher", "1.0.0", "2.0.0"},
		{"equal", "1.0.0", "1.0.0"},
		{"lower", "2.0.0", "1.0.0"},
		{"fourth-field-only", "1.0.0.1", "1.0.0.2"},
	} {
		c := newCase("UPG-01", v.name)
		p1, m1 := upgradePkg(c, "first.msi", v.first, "1401", nil)
		p2, m2 := upgradePkg(c, "second.msi", v.second, "1401", nil)
		r1 := runMsiexec(c, "install-first.log", "/i", m1)
		r2 := runMsiexec(c, "install-second.log", "/i", m2)
		c.logf("first %s exit %d; second %s exit %d", v.first, r1.Exit, v.second, r2.Exit)
		for _, prop := range []string{"UPGRADEFOUND", "NEWERFOUND"} {
			c.logf("  second install log: %s = %q", prop, logProperty(r2.Log, prop))
		}
		c.logf("RESULT %s: first still registered=%v, second registered=%v", v.name, registered(p1.ProductCode), registered(p2.ProductCode))
		removeProduct(c, p2.ProductCode)
		removeProduct(c, p1.ProductCode)
		c.close()
	}
}

func logProperty(log, name string) string {
	for _, l := range strings.Split(log, "\n") {
		if strings.HasPrefix(l, "Property(S): "+name+" = ") {
			return strings.TrimSpace(strings.TrimPrefix(l, "Property(S): "+name+" = "))
		}
	}
	return ""
}

func caseUPG02() {
	for _, v := range []string{"launchcondition", "type19"} {
		c := newCase("UPG-02", v)
		p2, m2 := upgradePkg(c, "newer.msi", "2.0.0", "1401", nil)
		p1, m1 := upgradePkg(c, "older.msi", "1.0.0", "1401", func(p *pkg) {
			if v == "launchcondition" {
				p.Extra["LaunchCondition"] = [][]string{{"NOT NEWERFOUND", "A newer version is already installed."}}
				// LaunchConditions (100) must run after FindRelatedProducts (25): default order does that.
			} else {
				p.Extra["CustomAction"] = [][]string{{"PreventDowngrade", "19", "", "A newer version is already installed.", ""}}
				p.ExtraSequence = append(p.ExtraSequence, []string{"PreventDowngrade", "NEWERFOUND", "26"})
			}
		})
		runMsiexec(c, "install-newer.log", "/i", m2)
		r := runMsiexec(c, "install-older.log", "/i", m1)
		c.logf("RESULT %s: downgrade exit %d; newer still registered=%v; older registered=%v", v, r.Exit, registered(p2.ProductCode), registered(p1.ProductCode))
		removeProduct(c, p1.ProductCode)
		removeProduct(c, p2.ProductCode)
		c.close()
	}
}

func caseUPG04() {
	for _, pos := range []string{"1401", "1501", "6551", "6601", "5802", "1399"} {
		c := newCase("UPG-04", "rep-"+pos)
		p1, m1 := upgradePkg(c, "v1.msi", "1.0.0", "1401", nil)
		p2, m2 := upgradePkg(c, "v2.msi", "2.0.0", pos, nil)
		runMsiexec(c, "install-v1.log", "/i", m1)
		r := runMsiexec(c, "install-v2.log", "/i", m2)
		file := filepath.Join(pf64(), "MsiSpecTest", "readme.txt")
		c.logf("RESULT REP at %s: v2 exit %d, v1 registered=%v, v2 registered=%v, file present=%v", pos, r.Exit, registered(p1.ProductCode), registered(p2.ProductCode), exists(file))
		removeProduct(c, p2.ProductCode)
		removeProduct(c, p1.ProductCode)
		c.close()
	}
}

func caseUPG03() {
	c := newCase("UPG-03", "same-productcode")
	defer c.close()
	code := newGUID()
	pa := &pkg{Name: "run11", Is64: true, ProductCode: code}
	ma := control(c, pa, "a.msi")
	pb := &pkg{Name: "run11", Is64: true, ProductCode: code}
	mb := control(c, pb, "b-new-packagecode.msi")
	same := writeVariant(c, "a-copy.msi", mustRead(ma))
	runMsiexec(c, "install-a.log", "/i", ma)
	r1 := runMsiexec(c, "install-a-copy.log", "/i", same)
	r2 := runMsiexec(c, "install-b.log", "/i", mb)
	r3 := runMsiexec(c, "install-b-reinstall.log", "/i", mb, "REINSTALL=ALL", "REINSTALLMODE=vomus")
	c.logf("RESULT same package again -> %d; same ProductCode, new PackageCode -> %d; with REINSTALL=ALL REINSTALLMODE=vomus -> %d", r1.Exit, r2.Exit, r3.Exit)
	removeProduct(c, code)
}

func caseREG01() {
	for _, plat := range []string{"x64", "Intel"} {
		c := newCase("REG-01", plat)
		p := &pkg{Name: "run07" + plat, Is64: plat == "x64", Template: plat + ";1033"}
		p.Extra = map[string][][]string{
			"Component": {
				{"C_reg64", guidFor("run07-64" + plat), "INSTALLDIR", "260", "", "R64"},
				{"C_reg32", guidFor("run07-32" + plat), "INSTALLDIR", "4", "", "R32"},
			},
			"Registry": {
				{"R64", "2", `Software\MsiSpecTest`, "Bit256", "#1", "C_reg64"},
				{"R32", "2", `Software\MsiSpecTest`, "Bit0", "#1", "C_reg32"},
			},
			"FeatureComponents": {{"Main", "C_reg64"}, {"Main", "C_reg32"}},
		}
		m := control(c, p, "pkg.msi")
		r := runMsiexec(c, "install.log", "/i", m)
		v64 := cmdOut("reg.exe", "query", `HKLM\Software\MsiSpecTest`, "/reg:64")
		v32 := cmdOut("reg.exe", "query", `HKLM\Software\MsiSpecTest`, "/reg:32")
		c.logf("install %d", r.Exit)
		c.logf("64-bit view: Bit256=%v Bit0=%v", strings.Contains(v64, "Bit256"), strings.Contains(v64, "Bit0"))
		c.logf("32-bit view: Bit256=%v Bit0=%v", strings.Contains(v32, "Bit256"), strings.Contains(v32, "Bit0"))
		u := runMsiexec(c, "uninstall.log", "/x", m)
		v64 = cmdOut("reg.exe", "query", `HKLM\Software\MsiSpecTest`, "/reg:64")
		v32 = cmdOut("reg.exe", "query", `HKLM\Software\MsiSpecTest`, "/reg:32")
		c.logf("uninstall %d; after: 64-bit has values=%v, 32-bit has values=%v", u.Exit, strings.Contains(v64, "Bit"), strings.Contains(v32, "Bit"))
		uk := cmdOut("reg.exe", "query", `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+p.ProductCode, "/reg:64")
		c.logf("RESULT %s template: bit 256 -> 64-bit view, no bit -> 32-bit view (see above)", plat)
		_ = uk
		c.close()
	}
}

const envKey = `HKLM\System\CurrentControlSet\Control\Session Manager\Environment`

func envGet(name string) (string, bool) {
	out := cmdOut("reg.exe", "query", envKey, "/v", name)
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, name+" ") {
			f := strings.Fields(l)
			if len(f) >= 3 {
				return strings.Join(f[2:], " "), true
			}
			return "", true
		}
	}
	return "", false
}

func envSet(name, val string) {
	exec.Command("reg.exe", "add", envKey, "/v", name, "/t", "REG_SZ", "/d", val, "/f").Run()
}

func envDel(name string) { exec.Command("reg.exe", "delete", envKey, "/v", name, "/f").Run() }

func caseENVVAR01() {
	c := newCase("ENVVAR-01", "environment")
	defer c.close()
	type row struct{ key, name, value, pre string }
	rows := []row{
		{"E1", "=-*MSISPEC_SET", "one", ""},
		{"E2", "=*MSISPEC_PERM", "two", ""},
		{"E3", "+-*MSISPEC_PLUS", "new", "existing"},
		{"E4", "=-*MSISPEC_PATH", "[~];added", "base"},
		{"E5", "=-*MSISPEC_PRE", "added;[~]", "base"},
		{"E6", "-*MSISPEC_MINUS", "x", ""},
		{"E7", "*MSISPEC_NOPREFIX", "seven", ""},
		{"E8", "!-*MSISPEC_BANG", "", "gone"},
	}
	var envRows [][]string
	for _, r := range rows {
		envRows = append(envRows, []string{r.key, r.name, r.value, "C_f_readme"})
		v := strings.TrimLeft(r.name, "=+-!*")
		envDel(v)
		if r.pre != "" {
			envSet(v, r.pre)
		}
	}
	p := &pkg{Name: "run08", Is64: true, Extra: map[string][][]string{"Environment": envRows}}
	p.ExtraSequence = [][]string{{"RemoveEnvironmentStrings", "", "3300"}}
	m := control(c, p, "pkg.msi")
	r := runMsiexec(c, "install.log", "/i", m)
	c.logf("install %d", r.Exit)
	after := map[string]string{}
	for _, rw := range rows {
		v := strings.TrimLeft(rw.name, "=+-!*")
		val, ok := envGet(v)
		after[v] = fmt.Sprintf("%q present=%v", val, ok)
		c.logf("  %-22s value %-12q pre %-10q -> after install %q present=%v", rw.name, rw.value, rw.pre, val, ok)
	}
	u := runMsiexec(c, "uninstall.log", "/x", m)
	c.logf("uninstall %d", u.Exit)
	for _, rw := range rows {
		v := strings.TrimLeft(rw.name, "=+-!*")
		val, ok := envGet(v)
		c.logf("  %-22s -> after uninstall %q present=%v", rw.name, val, ok)
		envDel(v)
	}
}

// versionedCopy copies a system executable and rewrites its fixed file version.
func versionedCopy(dst string, ms, ls uint32) error {
	b, err := os.ReadFile(`C:\Windows\System32\whoami.exe`)
	if err != nil {
		return err
	}
	sig := []byte{0xBD, 0x04, 0xEF, 0xFE}
	i := bytes.Index(b, sig)
	if i < 0 {
		return fmt.Errorf("no VS_FIXEDFILEINFO")
	}
	le := binary.LittleEndian
	le.PutUint32(b[i+8:], ms)
	le.PutUint32(b[i+12:], ls)
	le.PutUint32(b[i+16:], ms)
	le.PutUint32(b[i+20:], ls)
	return os.WriteFile(dst, b, 0o644)
}

func caseFILE01() {
	for _, v := range []struct {
		name     string
		authored bool
		rep      string
	}{
		{"authored-version-rep1401", true, "1401"},
		{"authored-version-rep6601", true, "6601"},
		{"empty-version-rep6601", false, "6601"},
		{"empty-version-rep1401", false, "1401"},
	} {
		c := newCase("FILE-01", v.name)
		dir, _ := filepath.Abs(c.dir)
		e1, e2 := filepath.Join(dir, "app-1.exe"), filepath.Join(dir, "app-2.exe")
		versionedCopy(e1, 1<<16, 0)
		versionedCopy(e2, 2<<16, 0)
		b1, b2 := mustRead(e1), mustRead(e2)
		build := func(file, ver string, data []byte, fileVer string) (*pkg, string) {
			p := &pkg{Name: "run02", Is64: true, Version: ver, UpgradeCode: "{2B9F4C11-7A3D-4E5B-8C6F-1D2E3F4A5B6C}",
				Files: []payload{{Key: "f_app", Name: "app.exe", Data: data}}}
			p.Extra = map[string][][]string{"Upgrade": {{p.UpgradeCode, "", ver, "", "256", "", "UPGRADEFOUND"}}}
			p.ExtraProps = [][]string{{"SecureCustomProperties", "UPGRADEFOUND"}}
			p.ExtraSequence = [][]string{{"InstallExecute", "", "6500"}, {"RemoveExistingProducts", "", v.rep}}
			m := control(c, p, file)
			if fileVer != "" {
				db, _ := openDB(m, dbTransact)
				_, err := query(db, "UPDATE `File` SET `Version`=?, `Language`='0' WHERE `File`='f_app'", fileVer)
				c.logf("  set File.Version %s: %v, commit %v", fileVer, err, commit(db))
				closeHandle(db)
			}
			return p, m
		}
		fv1, fv2 := "", ""
		if v.authored {
			fv1, fv2 = "1.0.0.0", "2.0.0.0"
		}
		p1, m1 := build("v1.msi", "1.0.0", b1, fv1)
		p2, m2 := build("v2.msi", "2.0.0", b2, fv2)
		target := filepath.Join(pf64(), "MsiSpecTest", "app.exe")
		runMsiexec(c, "install-v1.log", "/i", m1)
		r := runMsiexec(c, "install-v2.log", "/i", m2)
		got, _ := os.ReadFile(target)
		which := "missing"
		switch {
		case bytes.Equal(got, b1):
			which = "v1 bytes"
		case bytes.Equal(got, b2):
			which = "v2 bytes"
		}
		c.logf("RESULT %s: upgrade exit %d, installed app.exe = %s, v1 registered=%v", v.name, r.Exit, which, registered(p1.ProductCode))
		for _, h := range r.Hits {
			if strings.Contains(h, "Disallowing") {
				c.logf("  log: %s", h)
			}
		}
		// repair keeps or restores
		os.Remove(target)
		rr := runMsiexec(c, "repair-v2.log", "/fa", m2)
		got, _ = os.ReadFile(target)
		c.logf("  repair /fa after deleting file: exit %d, file restored=%v", rr.Exit, bytes.Equal(got, b2))
		removeProduct(c, p2.ProductCode)
		removeProduct(c, p1.ProductCode)
		c.close()
	}
}
