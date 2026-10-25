package main

// Service and custom-action cases.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	cases["RUN-05"] = caseRun05
	cases["RUN-18"] = caseRun18
}

const svcName = "MsiSpecTestSvc"

func svcPayload() []payload {
	exe := filepath.Join("testsvc", "testsvc.exe") // go build -o testsvc/testsvc.exe ./testsvc
	return []payload{{Key: "f_svc", Name: "testsvc.exe", Data: mustRead(exe)}}
}

func mustAbs(p string) string { a, _ := filepath.Abs(p); return a }

func scq(args ...string) string { return strings.TrimSpace(cmdOut("sc.exe", args...)) }

func svcState() string {
	out := scq("query", svcName)
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "STATE") {
			return strings.Join(strings.Fields(l)[2:], " ")
		}
	}
	if strings.Contains(out, "1060") {
		return "not installed"
	}
	return out
}

func caseRun05() {
	type variant struct {
		name  string
		event string
		extra func(p *pkg)
	}
	vs := []variant{
		{"install-start-stop-delete", "163", nil},
		{"no-delete-event", "35", nil},
		{"failureactions-table-restart", "163", func(p *pkg) {
			p.Extra["MsiServiceConfigFailureActions"] = [][]string{{"FA", svcName, "1", "86400", "", "", "1", "1000", "C_f_svc"}}
			p.ExtraSequence = append(p.ExtraSequence, []string{"MsiConfigureServices", "VersionMsi >= \"5.00\"", "5850"})
		}},
		{"sc-failure-recipe", "163", func(p *pkg) {
			p.Extra["CustomAction"] = [][]string{{"ScFailure", "3106", "INSTALLDIR", `"[System64Folder]sc.exe" failure ` + svcName + ` reset= 86400 actions= restart/1000`, ""}}
			p.ExtraSequence = append(p.ExtraSequence, []string{"ScFailure", "NOT REMOVE", "5850"})
		}},
	}
	schema["MsiServiceConfigFailureActions"] = tbl{Cols: []string{"MsiServiceConfigFailureActions s72", "Name s255", "Event i2", "ResetPeriod I4", "RebootMessage L255", "Command L255", "Actions S255", "DelayActions S255", "Component_ s72"}, Keys: []string{"MsiServiceConfigFailureActions"}}
	for _, v := range vs {
		c := newCase("RUN-05", v.name)
		p := &pkg{Name: "run05", Is64: true, Files: svcPayload()}
		p.Extra = map[string][][]string{
			"ServiceInstall": {{"SI", svcName, "MsiSpec Test Service", "16", "3", "32769", "", "", "LocalSystem", "", "", "C_f_svc", "Disposable MSI spec test service"}},
			"ServiceControl": {{"SC", svcName, v.event, "", "1", "C_f_svc"}},
		}
		if v.extra != nil {
			v.extra(p)
		}
		m := control(c, p, "pkg.msi")
		r := runMsiexec(c, "install.log", "/i", m)
		c.logf("install %d; service state %s", r.Exit, svcState())
		c.logf("sc qc: %s", oneLine(scq("qc", svcName)))
		c.logf("sc qfailure: %s", oneLine(scq("qfailure", svcName)))
		if r.Exit == 0 {
			rr := runMsiexec(c, "repair.log", "/fa", m)
			c.logf("repair %d; service state %s", rr.Exit, svcState())
		}
		u := runMsiexec(c, "uninstall.log", "/x", m)
		c.logf("RESULT %s: install %d, uninstall %d, service after uninstall: %s", v.name, r.Exit, u.Exit, svcState())
		if svcState() != "not installed" {
			exec.Command("sc.exe", "stop", svcName).Run()
			c.logf("  cleanup: sc delete -> %s", oneLine(scq("delete", svcName)))
		}
		removeProduct(c, p.ProductCode)
		c.close()
	}
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\r", "")), " ")
}

func caseRun18() {
	type variant struct {
		name, typ, source, target string
		seq                       string
	}
	cmd := `"[System64Folder]cmd.exe" /c exit `
	vs := []variant{
		{"exe-exit1-immediate", "34", "INSTALLDIR", cmd + "1", "1450"},
		{"exe-exit1-immediate-continue", "98", "INSTALLDIR", cmd + "1", "1450"},
		{"exe-exit1-deferred", "3106", "INSTALLDIR", cmd + "1", "4001"},
		{"exe-exit1-deferred-continue", "3170", "INSTALLDIR", cmd + "1", "4001"},
		{"vbs-return-success-1", "38", "Main", "Function Main()\r\nMain = 1\r\nEnd Function", "1450"},
		{"vbs-return-userexit-2", "38", "Main", "Function Main()\r\nMain = 2\r\nEnd Function", "1450"},
		{"vbs-return-failure-3", "38", "Main", "Function Main()\r\nMain = 3\r\nEnd Function", "1450"},
		{"vbs-return-finished-5-immediate", "38", "Main", "Function Main()\r\nMain = 5\r\nEnd Function", "1450"},
		{"js-return-failure-3", "37", "Main", "function Main() { return 3; }", "1450"},
		{"js-return-finished-5-immediate", "37", "Main", "function Main() { return 5; }", "1450"},
		{"vbs-syntax-error", "38", "", "this is not vbscript (", "1450"},
		{"vbs-binary-return-1", "6", "Script", "Main", "1450"},
		{"vbs-binary-return-2", "6", "Script", "Main", "1450"},
		{"vbs-binary-return-3", "6", "Script", "Main", "1450"},
		{"vbs-binary-return-5-immediate", "6", "Script", "Main", "1450"},
		{"vbs-binary-return-5-deferred", "1030", "Script", "Main", "4001"},
		{"vbs-return-finished-5-deferred", "1062", "Main", "Function Main()\r\nMain = 5\r\nEnd Function", "4001"},
	}
	only := os.Getenv("ONLY")
	for _, v := range vs {
		if only != "" && !strings.Contains(v.name, only) {
			continue
		}
		c := newCase("RUN-18", v.name)
		p := &pkg{Name: "run18", Is64: true}
		p.Extra = map[string][][]string{"CustomAction": {{"TestCA", v.typ, v.source, v.target, ""}}}
		p.ExtraSequence = [][]string{{"TestCA", "NOT REMOVE", v.seq}}
		m := control(c, p, "pkg.msi")
		if v.source == "Script" {
			ret := v.name[len("vbs-binary-return-")]
			sp := c.path("script.vbs")
			os.WriteFile(sp, []byte("Function Main()\r\nMain = "+string(ret)+"\r\nEnd Function\r\n"), 0o644)
			sp, _ = filepath.Abs(sp)
			db, _ := openDB(m, dbTransact)
			_, err := query(db, "CREATE TABLE `Binary` (`Name` CHAR(72) NOT NULL, `Data` OBJECT NOT NULL PRIMARY KEY `Name`)")
			err2 := insertStream(db, "Binary", "Script", sp)
			c.logf("Binary script return %c: %v %v commit %v", ret, err, err2, commit(db))
			closeHandle(db)
		}
		r := runMsiexec(c, "install.log", "/i", m)
		file := filepath.Join(pf64(), "MsiSpecTest", "readme.txt")
		ret := ""
		for _, l := range strings.Split(r.Log, "\n") {
			if strings.Contains(l, "Action ended") && strings.Contains(l, "TestCA") {
				ret = strings.TrimSpace(l)
			}
			if strings.Contains(l, "Executing op: CustomActionSchedule(Action=TestCA") {
				ret += " [scheduled]"
			}
		}
		lastAction := ""
		for _, l := range strings.Split(r.Log, "\n") {
			if strings.HasPrefix(l, "Action start") {
				lastAction = strings.TrimSpace(l)
			}
		}
		c.logf("RESULT %s: exit %d, file installed=%v, registered=%v, CA: %s, last action: %s", v.name, r.Exit, exists(file), registered(p.ProductCode), ret, lastAction)
		removeProduct(c, p.ProductCode)
		if exists(file) {
			c.logf("  file left behind without registration")
			os.Remove(file)
		}
		c.close()
	}
}
