package main

// CA-01: per-user rollback of a failing deferred action.

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var pMsiQueryProductStateW = msiDLL.NewProc("MsiQueryProductStateW")

func init() { cases["CA-01"] = caseCA01 }

// productState returns INSTALLSTATE: 5 default (installed), -1 unknown, 1 advertised, 2 absent.
func productState(code string) int32 {
	r, _, _ := pMsiQueryProductStateW.Call(uintptr(unsafe.Pointer(wstr(code))))
	return int32(r)
}

// squash turns a braced GUID into the registry form used under Installer\Products.
func squash(g string) string {
	h := strings.ReplaceAll(strings.Trim(g, "{}"), "-", "")
	rev := func(s string) string {
		b := []byte(s)
		for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}
		return string(b)
	}
	out := rev(h[0:8]) + rev(h[8:12]) + rev(h[12:16])
	for i := 16; i < 32; i += 2 {
		out += rev(h[i : i+2])
	}
	return out
}

func currentSID() string {
	out, _ := exec.Command("powershell", "-NoProfile", "-Command", "[Security.Principal.WindowsIdentity]::GetCurrent().User.Value").Output()
	return strings.TrimSpace(string(out))
}

func caseCA01() {
	sid := currentSID()
	for _, v := range []struct{ name, seq string }{
		{"fail-after-publishproduct", "6450"},
		{"fail-after-installfiles", "4001"},
	} {
		c := newCase("CA-01", v.name)
		p := &pkg{Name: "run13", Is64: true, PerUser: true}
		// Deferred (1024), impersonated EXE action in INSTALLDIR that exits 1.
		p.Extra = map[string][][]string{"CustomAction": {{"FailDeferred", "1058", "INSTALLDIR", `"[System64Folder]cmd.exe" /c exit 1`, ""}}}
		p.ExtraSequence = [][]string{{"FailDeferred", "NOT REMOVE", v.seq}}
		m := control(c, p, "pkg.msi")
		r := runMsiexec(c, "install.log", "/i", m)
		sq := squash(p.ProductCode)
		hku := `HKU\` + sid + `\Software\Microsoft\Installer\Products\` + sq
		ud := `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\` + sid + `\Products\` + sq
		c.logf("install exit %d; MsiQueryProductState %d", r.Exit, productState(p.ProductCode))
		for _, h := range r.Hits {
			if strings.Contains(h, "1402") || strings.Contains(h, "1403") {
				c.logf("  rollback log: %s", h)
			}
		}
		pub := regQuery(hku)
		c.logf("%s present=%v", hku, !strings.Contains(pub, "ERROR"))
		if !strings.Contains(pub, "ERROR") {
			c.logf("  %s", oneLine(regQuery(hku+`\SourceList`)))
		}
		c.logf("%s present=%v", ud, !strings.Contains(regQuery(ud), "ERROR"))
		u := runMsiexec(c, "uninstall.log", "/x", p.ProductCode)
		c.logf("uninstall by ProductCode exit %d; state after %d", u.Exit, productState(p.ProductCode))
		left := !strings.Contains(regQuery(hku), "ERROR") || !strings.Contains(regQuery(ud), "ERROR")
		c.logf("RESULT %s: install %d, registration left after failed install=%v", v.name, r.Exit, !strings.Contains(pub, "ERROR"))
		if left {
			c.logf("  cleanup: deleting leftover registration keys")
			for _, k := range []string{hku, ud} {
				exec.Command("reg.exe", "delete", k, "/f").Run()
			}
			c.logf("  after cleanup state %d", productState(p.ProductCode))
		}
		c.close()
	}
	_ = syscall.EINVAL
}
