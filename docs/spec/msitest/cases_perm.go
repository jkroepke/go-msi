package main

// MsiLockPermissionsEx cases (Windows Installer 5.0 SDDL permissions).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	cases["SEC-01"] = caseSec01
}

const permRegKey = `SOFTWARE\MsiSpecPerm`

func psOut(script string) string {
	return strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-Command", script))
}

func fileSDDL(path string) string {
	return psOut("(Get-Acl -LiteralPath '" + path + "').Sddl")
}

func regSDDL(key string) string {
	return psOut("(Get-Acl -LiteralPath 'Registry::HKEY_LOCAL_MACHINE\\" + key + "').Sddl")
}

func caseSec01() {
	schema["MsiLockPermissionsEx"] = tbl{Cols: []string{"MsiLockPermissionsEx s72", "LockObject s72", "Table s32", "SDDLText l0", "Condition S255"}, Keys: []string{"MsiLockPermissionsEx"}}
	schema["LockPermissions"] = tbl{Cols: []string{"LockObject s72", "Table s32", "Domain S255", "User s255", "Permission I4"}, Keys: []string{"LockObject", "Table", "Domain", "User"}}

	const (
		fileSD  = "D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;BU)"
		dirSD   = "D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;BU)"
		regSD   = "D:PAI(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;BU)"
		svcSD   = "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLORPWPRC;;;BU)"
		denySD  = "D:PAI(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)"
		ownerSD = "O:SYG:SYD:PAI(A;;FA;;;SY)(A;;FA;;;BA)"
	)

	type variant struct {
		name  string
		desc  string
		svc   bool
		extra func(p *pkg)
		check func(c *caseLog, p *pkg)
	}
	instDir := func(p *pkg) string { return filepath.Join(pf64(), p.InstallDirName) }
	fileVariant := func(name, sd string, cond string) variant {
		return variant{name: name, desc: "File row secured with " + sd,
			extra: func(p *pkg) {
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "f_perm", "File", sd, cond}}
			},
			check: func(c *caseLog, p *pkg) {
				c.logf("authored SDDL: %s", sd)
				c.logf("observed SDDL: %s", fileSDDL(filepath.Join(instDir(p), "perm.txt")))
			}}
	}
	vs := []variant{
		fileVariant("file", fileSD, ""),
		fileVariant("file-deny-ace", denySD, ""),
		fileVariant("file-owner-group", ownerSD, ""),
		fileVariant("file-condition-false", fileSD, "0"),
		{name: "createfolder", desc: "CreateFolder row with inheritable ACEs; a file created inside afterwards",
			extra: func(p *pkg) {
				p.Extra["Directory"] = [][]string{{"DATADIR", "INSTALLDIR", "data"}}
				p.Extra["Component"] = [][]string{{"C_data", guidFor("run19-data"), "DATADIR", "256", "", ""}}
				p.Extra["CreateFolder"] = [][]string{{"DATADIR", "C_data"}}
				p.Extra["FeatureComponents"] = [][]string{{"Main", "C_data"}}
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "DATADIR", "CreateFolder", dirSD, ""}}
			},
			check: func(c *caseLog, p *pkg) {
				d := filepath.Join(instDir(p), "data")
				c.logf("authored SDDL: %s", dirSD)
				c.logf("observed folder SDDL: %s", fileSDDL(d))
				child := filepath.Join(d, "child.txt")
				os.WriteFile(child, []byte("x"), 0o644)
				c.logf("file created afterwards inherits: %s", fileSDDL(child))
				os.Remove(child)
			}},
		{name: "registry", desc: "Registry row secured; key permissions",
			extra: func(p *pkg) {
				p.Extra["Component"] = [][]string{{"C_reg", guidFor("run19-reg"), "INSTALLDIR", "260", "", "R1"}}
				p.Extra["Registry"] = [][]string{{"R1", "2", permRegKey, "Value", "#1", "C_reg"}}
				p.Extra["FeatureComponents"] = [][]string{{"Main", "C_reg"}}
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "R1", "Registry", regSD, ""}}
			},
			check: func(c *caseLog, p *pkg) {
				c.logf("authored SDDL: %s", regSD)
				c.logf("observed key SDDL: %s", regSDDL(permRegKey))
			}},
		{name: "service", desc: "ServiceInstall row secured; BU may start and stop the service", svc: true,
			extra: func(p *pkg) {
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "SI", "ServiceInstall", svcSD, ""}}
			},
			check: func(c *caseLog, p *pkg) {
				c.logf("authored SDDL: %s", svcSD)
				c.logf("observed service SDDL: %s", oneLine(scq("sdshow", svcName)))
			}},
		{name: "formatted-property", desc: "SDDLText containing [PERMSID] with PERMSID=BU (docs: properties not supported)",
			extra: func(p *pkg) {
				p.ExtraProps = append(p.ExtraProps, []string{"PERMSID", "BU"})
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "f_perm", "File", "D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;[PERMSID])", ""}}
			},
			check: func(c *caseLog, p *pkg) {
				c.logf("observed SDDL: %s", fileSDDL(filepath.Join(instDir(p), "perm.txt")))
			}},
		{name: "formatted-sid-lookup", desc: "SDDLText with [LogonUser], an account name rather than a SID string or alias",
			extra: func(p *pkg) {
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "f_perm", "File", "D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;[LogonUser])", ""}}
			},
			check: func(c *caseLog, p *pkg) {
				c.logf("observed SDDL: %s", fileSDDL(filepath.Join(instDir(p), "perm.txt")))
			}},
		{name: "invalid-sddl", desc: "SDDLText that is not SDDL (docs: error 1943)",
			extra: func(p *pkg) {
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "f_perm", "File", "D:garbage(", ""}}
			}},
		{name: "two-true-conditions", desc: "two rows for the same object, both true (docs: error 1942)",
			extra: func(p *pkg) {
				p.Extra["MsiLockPermissionsEx"] = [][]string{
					{"LP1", "f_perm", "File", fileSD, "1"},
					{"LP2", "f_perm", "File", denySD, "1"},
				}
			}},
		{name: "with-lockpermissions", desc: "LockPermissions and MsiLockPermissionsEx in one package (spec: error 1941)",
			extra: func(p *pkg) {
				p.Extra["MsiLockPermissionsEx"] = [][]string{{"LP1", "f_perm", "File", fileSD, ""}}
				p.Extra["LockPermissions"] = [][]string{{"f_perm", "File", "", "Administrators", "268435456"}}
			}},
	}

	only := os.Getenv("ONLY")
	for _, v := range vs {
		if only != "" && !strings.Contains(v.name, only) {
			continue
		}
		c := newCase("SEC-01", v.name)
		c.logf("%s", v.desc)
		p := &pkg{Name: "run19", Is64: true, InstallDirName: "MsiSpecPerm",
			Files: []payload{{Key: "f_perm", Name: "perm.txt", Data: []byte("MsiLockPermissionsEx test\r\n")}}}
		if v.svc {
			p.Files = append(p.Files, svcPayload()...)
			p.Extra = map[string][][]string{
				"ServiceInstall": {{"SI", svcName, "MsiSpec Test Service", "16", "3", "32769", "", "", "LocalSystem", "", "", "C_f_svc", "Disposable MSI spec test service"}},
				"ServiceControl": {{"SC", svcName, "162", "", "1", "C_f_svc"}},
			}
		} else {
			p.Extra = map[string][][]string{}
		}
		v.extra(p)
		m := control(c, p, "pkg.msi")
		if m == "" {
			c.close()
			continue
		}
		r := runMsiexec(c, "install.log", "/i", m)
		c.logf("install %d", r.Exit)
		if r.Exit == 0 && v.check != nil {
			v.check(c, p)
			if v.name == "file" {
				// Reset the ACL by hand, then see whether a repair puts the authored one back.
				f := filepath.Join(instDir(p), "perm.txt")
				exec.Command("icacls.exe", f, "/reset").Run()
				c.logf("after icacls /reset: %s", fileSDDL(f))
				rr := runMsiexec(c, "repair.log", "/fa", m)
				c.logf("repair /fa %d: %s", rr.Exit, fileSDDL(f))
				rr = runMsiexec(c, "repair-vomus.log", "/fvomus", m)
				c.logf("repair /fvomus %d: %s", rr.Exit, fileSDDL(f))
			}
		}
		u := runMsiexec(c, "uninstall.log", "/x", m)
		c.logf("RESULT %s: install %d, uninstall %d, install dir left: %v", v.name, r.Exit, u.Exit, exists(instDir(p)))
		if v.svc && svcState() != "not installed" {
			exec.Command("sc.exe", "stop", svcName).Run()
			c.logf("  cleanup: sc delete -> %s", oneLine(scq("delete", svcName)))
		}
		removeProduct(c, p.ProductCode)
		c.close()
	}
}
