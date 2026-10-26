package main

// Cases for the use-case gaps: preserved configuration, a remembered install
// folder, data removal on uninstall and per-step service recovery.

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	cases["UPG-05"] = caseUpg05
	cases["SEARCH-03"] = caseSearch03
	cases["FILE-03"] = caseFile03
	cases["SVC-03"] = caseSvc03
}

const (
	attrPermanent      = 0x10
	attrNeverOverwrite = 0x80
	attrRegKeyPath     = 0x4
	attr64             = 0x100
)

// hashRow is the MsiFileHash row of data (MD5 as four little-endian int32,
// spec A.6), as the go-msi builder writes it for every unversioned file.
func hashRow(key string, data []byte) []string {
	m := md5.Sum(data)
	row := []string{key, "0"}
	for i := 0; i < 4; i++ {
		row = append(row, fmt.Sprint(int32(binary.LittleEndian.Uint32(m[4*i:]))))
	}
	return row
}

func regValue(key, name string) string {
	out := cmdOut("reg.exe", "query", `HKLM\`+key, "/v", name, "/reg:64")
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) >= 3 && f[0] == name {
			return strings.Join(f[2:], " ")
		}
	}
	return "<absent>"
}

func fileState(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "<absent>"
	}
	return strings.TrimSpace(string(b))
}

const preserveKey = `SOFTWARE\MsiSpecTest\preserve`

// squashGUID is the form the engine uses for registry key names: each of the
// first three groups reversed, then the two hex digits of each later byte
// swapped.
func squashGUID(guid string) string {
	g := strings.Trim(strings.ReplaceAll(guid, "-", ""), "{}")
	rev := func(s string) string {
		b := []byte(s)
		for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}
		return string(b)
	}
	out := rev(g[0:8]) + rev(g[8:12]) + rev(g[12:16])
	for i := 16; i < 32; i += 2 {
		out += string(g[i+1]) + string(g[i])
	}
	return out
}

// forgetComponents deletes the machine registration of components a
// permanent variant left behind. Without it the next variant that reuses
// the GUID finds the component still installed (its registration is never
// removed) and behaves differently.
func forgetComponents(c *caseLog, guids ...string) {
	for _, g := range guids {
		key := `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Components\` + squashGUID(g)
		if strings.Contains(cmdOut("reg.exe", "query", key, "/reg:64"), "REG_") {
			c.logf("  removing leftover component registration %s", g)
			exec.Command("reg.exe", "delete", key, "/f", "/reg:64").Run()
		}
	}
}

// caseUpg05: does a component with neverOverwrite and/or permanent keep a
// file and a registry value the user edited across a major upgrade, for an
// early (1401) and a late (6551) RemoveExistingProducts?
func caseUpg05() {
	for _, v := range []struct {
		name         string
		bits1, bits2 int
		rep          string
		hash         bool
	}{
		{"plain-early", 0, 0, "1401", true},
		{"never-early", attrNeverOverwrite, attrNeverOverwrite, "1401", true},
		{"perm-early", attrPermanent, attrPermanent, "1401", true},
		{"neverperm-early", attrNeverOverwrite | attrPermanent, attrNeverOverwrite | attrPermanent, "1401", true},
		{"plain-late", 0, 0, "6551", true},
		{"plain-late-nohash", 0, 0, "6551", false},
		{"never-late", attrNeverOverwrite, attrNeverOverwrite, "6551", true},
		{"neverperm-retrofit-early", 0, attrNeverOverwrite | attrPermanent, "1401", true},
		{"never-retrofit-late", 0, attrNeverOverwrite, "6551", true},
	} {
		c := newCase("UPG-05", v.name)
		// Each variant has its own component GUIDs, so that a permanent
		// registration of one variant cannot change the next (the GUIDs of
		// the first harness version are cleaned up too).
		name := "upg-" + v.name
		cfgGUID := guidFor(name + "f_cfg" + upgradeUC)
		regGUID := guidFor(name + "-preserve-reg" + upgradeUC)
		legacy := []string{guidFor("upgf_cfg" + upgradeUC), guidFor("upg-preserve-reg" + upgradeUC)}
		forgetComponents(c, append(legacy, cfgGUID, regGUID)...)
		dir := filepath.Join(pf64(), "MsiSpecTest")
		cfg := filepath.Join(dir, "config.txt")
		os.Remove(cfg)
		exec.Command("reg.exe", "delete", `HKLM\`+preserveKey, "/f", "/reg:64").Run()
		mk := func(file, version string, bits int) (*pkg, string) {
			data := []byte(version + "-default\r\n")
			return upgradePkg(c, file, version, v.rep, func(p *pkg) {
				p.Name = name
				p.Files = []payload{
					{Key: "f_readme", Name: "readme.txt", Data: []byte("readme " + version + "\r\n")},
					{Key: "f_cfg", Name: "config.txt", Data: data},
				}
				p.CompAttr = map[string]int{"f_cfg": bits}
				p.Extra["Component"] = [][]string{{"C_reg", regGUID, "INSTALLDIR", fmt.Sprint(attr64 | attrRegKeyPath | bits), "", "R_args"}}
				p.Extra["Registry"] = [][]string{{"R_args", "2", preserveKey, "Arguments", version + "-default", "C_reg"}}
				p.Extra["FeatureComponents"] = [][]string{{"Main", "C_reg"}}
				if v.hash {
					p.Extra["MsiFileHash"] = [][]string{hashRow("f_cfg", data)}
				}
			})
		}
		p1, m1 := mk("v1.msi", "1.0.0", v.bits1)
		p2, m2 := mk("v2.msi", "2.0.0", v.bits2)
		r1 := runMsiexec(c, "install-v1.log", "/i", m1)
		c.logf("v1 install %d: config=%q registry=%q", r1.Exit, fileState(cfg), regValue(preserveKey, "Arguments"))
		os.WriteFile(cfg, []byte("user-edit\r\n"), 0o644)
		exec.Command("reg.exe", "add", `HKLM\`+preserveKey, "/v", "Arguments", "/d", "user-edit", "/f", "/reg:64").Run()
		c.logf("after user edit: config=%q registry=%q", fileState(cfg), regValue(preserveKey, "Arguments"))
		r2 := runMsiexec(c, "install-v2.log", "/i", m2)
		c.logf("RESULT %s upgrade: v2 exit %d, v1 registered=%v, config=%q, registry=%q", v.name, r2.Exit, registered(p1.ProductCode), fileState(cfg), regValue(preserveKey, "Arguments"))
		r3 := runMsiexec(c, "uninstall-v2.log", "/x", p2.ProductCode)
		c.logf("RESULT %s uninstall: exit %d, v2 registered=%v, readme present=%v, config=%q, registry=%q", v.name, r3.Exit,
			registered(p2.ProductCode), exists(filepath.Join(dir, "readme.txt")), fileState(cfg), regValue(preserveKey, "Arguments"))
		removeProduct(c, p2.ProductCode)
		removeProduct(c, p1.ProductCode)
		forgetComponents(c, cfgGUID, regGUID)
		os.Remove(cfg)
		os.Remove(filepath.Join(dir, "readme.txt"))
		os.Remove(dir)
		exec.Command("reg.exe", "delete", `HKLM\SOFTWARE\MsiSpecTest`, "/f", "/reg:64").Run()
		c.close()
	}
}

const rememberKey = `SOFTWARE\MsiSpecTest\remember`

// caseSearch03: a registry value written at install and read back by AppSearch
// keeps a custom install folder across a major upgrade. Checks whether
// AppSearch overwrites INSTALLDIR given on the command line, and a guarded
// type-51 action that only applies the remembered value when INSTALLDIR is
// empty before CostFinalize.
func caseSearch03() {
	custom := `C:\MsiSpecCustom\`
	other := `C:\MsiSpecOther\`
	for _, v := range []struct {
		name, mode, cmdline string
	}{
		{"none", "none", ""},
		{"direct", "direct", ""},
		{"direct-nosignature", "direct-nosignature", ""},
		{"direct-cmdline", "direct", other},
		{"guarded", "guarded", ""},
		{"guarded-cmdline", "guarded", other},
	} {
		c := newCase("SEARCH-03", v.name)
		mk := func(file, version string, search bool) (*pkg, string) {
			return upgradePkg(c, file, version, "1401", func(p *pkg) {
				p.Extra["Component"] = [][]string{{"C_rem", guidFor("upg-remember" + upgradeUC), "INSTALLDIR", fmt.Sprint(attr64 | attrRegKeyPath), "", "R_rem"}}
				p.Extra["Registry"] = [][]string{{"R_rem", "2", rememberKey, "InstallDir", "[INSTALLDIR]", "C_rem"}}
				p.Extra["FeatureComponents"] = [][]string{{"Main", "C_rem"}}
				p.ExtraProps = [][]string{{"SecureCustomProperties", "NEWERFOUND;UPGRADEFOUND;INSTALLDIR;REMEMBERED"}}
				if !search || v.mode == "none" {
					return
				}
				// 2 = raw value, 16 = 64-bit view.
				p.Extra["RegLocator"] = [][]string{{"S_rem", "2", rememberKey, "InstallDir", "18"}}
				p.ExtraSequence = append(p.ExtraSequence, []string{"AppSearch", "", "50"})
				if v.mode == "direct-nosignature" {
					p.NoSignature = true
				}
				if v.mode == "direct" || v.mode == "direct-nosignature" {
					p.Extra["AppSearch"] = [][]string{{"INSTALLDIR", "S_rem"}}
					return
				}
				p.Extra["AppSearch"] = [][]string{{"REMEMBERED", "S_rem"}}
				p.Extra["CustomAction"] = [][]string{{"ApplyRemembered", "51", "INSTALLDIR", "[REMEMBERED]", ""}}
				p.ExtraSequence = append(p.ExtraSequence, []string{"ApplyRemembered", "REMEMBERED AND NOT INSTALLDIR", "60"})
			})
		}
		p1, m1 := mk("v1.msi", "1.0.0", false)
		p2, m2 := mk("v2.msi", "2.0.0", true)
		r1 := runMsiexec(c, "install-v1.log", "/i", m1, "INSTALLDIR="+custom)
		c.logf("v1 install %d into %s: readme present=%v, registry=%q", r1.Exit, custom, exists(custom+"readme.txt"), regValue(rememberKey, "InstallDir"))
		args := []string{"/i", m2}
		if v.cmdline != "" {
			args = append(args, "INSTALLDIR="+v.cmdline)
		}
		r2 := runMsiexec(c, "install-v2.log", args...)
		c.logf("  v2 log: REMEMBERED=%q INSTALLDIR=%q", logProperty(r2.Log, "REMEMBERED"), logProperty(r2.Log, "INSTALLDIR"))
		c.logf("RESULT %s: v2 exit %d, v1 registered=%v, readme in custom=%v, in other=%v, in Program Files=%v, registry=%q",
			v.name, r2.Exit, registered(p1.ProductCode), exists(custom+"readme.txt"), exists(other+"readme.txt"),
			exists(filepath.Join(pf64(), "MsiSpecTest", "readme.txt")), regValue(rememberKey, "InstallDir"))
		removeProduct(c, p2.ProductCode)
		removeProduct(c, p1.ProductCode)
		for _, d := range []string{custom, other} {
			os.RemoveAll(d)
		}
		exec.Command("reg.exe", "delete", `HKLM\SOFTWARE\MsiSpecTest`, "/f", "/reg:64").Run()
		c.close()
	}
}

// caseFile03: RemoveFile rows and a deferred rmdir action on a data folder the
// application filled at run time, on a real uninstall and on the uninstall
// half of a major upgrade with an early RemoveExistingProducts.
func caseFile03() {
	data := filepath.Join(os.Getenv("ProgramData"), "MsiSpecData")
	fill := func() {
		os.MkdirAll(filepath.Join(data, "sub"), 0o755)
		os.WriteFile(filepath.Join(data, "app.log"), []byte("log\r\n"), 0o644)
		os.WriteFile(filepath.Join(data, "state.db"), []byte("db\r\n"), 0o644)
		os.WriteFile(filepath.Join(data, "sub", "wal.bin"), []byte("wal\r\n"), 0o644)
	}
	state := func() string {
		var have []string
		for _, f := range []string{"", "app.log", "state.db", "sub", `sub\wal.bin`} {
			if exists(filepath.Join(data, f)) {
				if f == "" {
					f = "<folder>"
				}
				have = append(have, f)
			}
		}
		if len(have) == 0 {
			return "<gone>"
		}
		return strings.Join(have, ",")
	}
	for _, v := range []struct {
		name, mode string
		upgrade    bool
		uninstArgs []string
	}{
		{"removefile-uninstall", "removefile", false, nil},
		{"removefile-upgrade", "removefile", true, nil},
		{"purge-uninstall", "purge", false, nil},
		{"purge-upgrade", "purge", true, nil},
		{"purge-optin-off", "purge-optin", false, nil},
		{"purge-optin-on", "purge-optin", false, []string{"DELETE_DATA=1"}},
	} {
		c := newCase("FILE-03", v.name)
		os.RemoveAll(data)
		mk := func(file, version string) (*pkg, string) {
			return upgradePkg(c, file, version, "1401", func(p *pkg) {
				p.Extra["Directory"] = [][]string{
					{"CommonAppDataFolder", "TARGETDIR", "."},
					{"DATADIR", "CommonAppDataFolder", "msispecd|MsiSpecData"},
					{"System64Folder", "TARGETDIR", "."},
				}
				p.Extra["Component"] = [][]string{{"C_data", guidFor("upg-data" + upgradeUC), "DATADIR", fmt.Sprint(attr64), "", ""}}
				p.Extra["CreateFolder"] = [][]string{{"DATADIR", "C_data"}}
				p.Extra["FeatureComponents"] = [][]string{{"Main", "C_data"}}
				switch v.mode {
				case "removefile":
					p.Extra["RemoveFile"] = [][]string{
						{"RF_files", "C_data", "*", "DATADIR", "2"},
						{"RF_folder", "C_data", "", "DATADIR", "2"},
					}
				case "purge", "purge-optin":
					cond := `REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE`
					if v.mode == "purge-optin" {
						cond += ` AND DELETE_DATA="1"`
						p.ExtraProps = [][]string{{"SecureCustomProperties", "NEWERFOUND;UPGRADEFOUND;DELETE_DATA"}}
					}
					// 34 exe with working folder + 1024 deferred + 2048 no impersonate + 64 continue.
					p.Extra["CustomAction"] = [][]string{{"PurgeData", "3170", "System64Folder", `"[System64Folder]cmd.exe" /c rmdir /s /q "[DATADIR]."`, ""}}
					p.ExtraSequence = append(p.ExtraSequence, []string{"PurgeData", cond, "6590"})
				}
			})
		}
		p1, m1 := mk("v1.msi", "1.0.0")
		p2, m2 := mk("v2.msi", "2.0.0")
		r1 := runMsiexec(c, "install-v1.log", "/i", m1)
		fill()
		c.logf("v1 install %d; data after fill: %s", r1.Exit, state())
		if v.upgrade {
			r := runMsiexec(c, "install-v2.log", "/i", m2)
			c.logf("RESULT %s: upgrade exit %d, v1 registered=%v, data=%s", v.name, r.Exit, registered(p1.ProductCode), state())
			r = runMsiexec(c, "uninstall-v2.log", "/x", p2.ProductCode)
			c.logf("RESULT %s: later uninstall exit %d, data=%s", v.name, r.Exit, state())
		} else {
			r := runMsiexec(c, "uninstall-v1.log", append([]string{"/x", p1.ProductCode}, v.uninstArgs...)...)
			c.logf("RESULT %s: uninstall %v exit %d, data=%s", v.name, v.uninstArgs, r.Exit, state())
		}
		removeProduct(c, p2.ProductCode)
		removeProduct(c, p1.ProductCode)
		os.RemoveAll(data)
		c.close()
	}
}

// caseSvc03: sc.exe failure with a different delay per step and a reset
// period that is not a whole number of days.
func caseSvc03() {
	for _, v := range []struct{ name, args string }{
		{"per-step", "reset= 3600 actions= restart/60000/restart/300000/restart/300000"},
		{"two-steps", "reset= 86400 actions= restart/1000/none/0"},
		{"infinite-reset", "reset= INFINITE actions= restart/5000"},
		{"zero-reset", "reset= 0 actions= restart/1000/restart/2000"},
		{"max-reset", "reset= 2147483647 actions= restart/2147483647"},
	} {
		c := newCase("SVC-03", v.name)
		p := &pkg{Name: "run27", Is64: true, Files: svcPayload()}
		p.Extra = map[string][][]string{
			"Directory":      {{"System64Folder", "TARGETDIR", "."}},
			"ServiceInstall": {{"SI", svcName, "MsiSpec Test Service", "16", "3", "32769", "", "", "LocalSystem", "", "", "C_f_svc", "Disposable MSI spec test service"}},
			"ServiceControl": {{"SC", svcName, "163", "", "1", "C_f_svc"}},
			"CustomAction":   {{"ScFailure", "3170", "System64Folder", `"[System64Folder]sc.exe" failure "` + svcName + `" ` + v.args, ""}},
		}
		p.ExtraSequence = [][]string{{"ScFailure", "$C_f_svc=3", "5850"}}
		m := control(c, p, "pkg.msi")
		r := runMsiexec(c, "install.log", "/i", m)
		c.logf("RESULT %s: install %d; sc qfailure: %s", v.name, r.Exit, oneLine(scq("qfailure", svcName)))
		u := runMsiexec(c, "uninstall.log", "/x", m)
		c.logf("RESULT %s: uninstall %d; service: %s", v.name, u.Exit, svcState())
		if svcState() != "not installed" {
			exec.Command("sc.exe", "stop", svcName).Run()
			scq("delete", svcName)
		}
		removeProduct(c, p.ProductCode)
		c.close()
	}
}
