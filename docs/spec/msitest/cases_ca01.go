package main

// CA-01: per-user rollback of a failing deferred action.

import (
	"fmt"
	"os/exec"
	"regexp"
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

// Rollback lines the engine logs when a registry undo is denied (CA-01).
var (
	rollback1403 = regexp.MustCompile(`Info 1403\.[^\r\n]*`)
	denied1402   = regexp.MustCompile(`Note: 1: 1402 2: [^\r\n]* 3: 5`)
)

// elevated reports whether the process token has the high mandatory level.
func elevated() bool {
	out, err := exec.Command("whoami.exe", "/groups").Output()
	if err != nil {
		fmt.Println("whoami /groups:", err)
	}
	return strings.Contains(string(out), "S-1-16-12288")
}

func caseCA01() {
	sid := currentSID()
	admin := elevated()
	variants := []struct {
		name, seq string
		dual, lad bool
	}{
		{"fail-after-publishproduct", "6450", false, false},
		{"fail-after-installfiles", "4001", false, false},
		// The shape of WiX issue 8574: ALLUSERS=2 with MSIINSTALLPERUSER=1.
		{"dual-fail-after-publishproduct", "6450", true, false},
		{"dual-fail-after-registerproduct", "6150", true, false},
		{"dual-fail-after-installfiles", "4001", true, false},
		// msi-builder's perUser scope: no ALLUSERS, INSTALLDIR under
		// LocalAppDataFolder, so a non-administrator can run it.
		{"lad-fail-after-publishproduct", "6450", false, true},
		{"lad-fail-after-registerproduct", "6150", false, true},
		{"lad-fail-after-installfiles", "4001", false, true},
	}
	for _, v := range variants {
		name := v.name
		if !admin {
			name += "-nonadmin"
		}
		c := newCase("CA-01", name)
		c.logf("user SID %s, elevated=%v", sid, admin)
		p := &pkg{Name: "run13", Is64: true, PerUser: true}
		if v.dual {
			p.ExtraProps = [][]string{{"ALLUSERS", "2"}, {"MSIINSTALLPERUSER", "1"}}
		}
		if v.lad {
			p.ParentDir = "LocalAppDataFolder"
		}
		if v.dual || v.lad || !admin {
			// Word Count bit 3: no elevation needed, as a per-user package declares it.
			p.WordCount = 10
		}
		// Deferred (1024), impersonated EXE action in INSTALLDIR that exits 1.
		p.Extra = map[string][][]string{"CustomAction": {{"FailDeferred", "1058", "INSTALLDIR", `"[System64Folder]cmd.exe" /c exit 1`, ""}}}
		p.ExtraSequence = [][]string{{"FailDeferred", "NOT REMOVE", v.seq}}
		m := control(c, p, "pkg.msi")
		r := runMsiexec(c, "install.log", "/i", m)
		sq := squash(p.ProductCode)
		hku := `HKU\` + sid + `\Software\Microsoft\Installer\Products\` + sq
		ud := `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\` + sid + `\Products\` + sq
		state := productState(p.ProductCode)
		c.logf("install exit %d; MsiQueryProductState %d", r.Exit, state)
		// logPatterns leaves out 1402 and 1403, so count them in the log itself.
		infos := rollback1403.FindAllString(r.Log, -1)
		c.logf("rollback: %d x Info 1403, %d x Note 1402 with access denied", len(infos), len(denied1402.FindAllString(r.Log, -1)))
		for _, h := range infos[:min(len(infos), 3)] {
			c.logf("  %s", h)
		}
		pub := regQuery(hku)
		c.logf("%s present=%v", hku, !strings.Contains(pub, "ERROR"))
		if !strings.Contains(pub, "ERROR") {
			c.logf("  %s", oneLine(regQuery(hku+`\SourceList`)))
		}
		udLeft := !strings.Contains(regQuery(ud), "ERROR")
		c.logf("%s present=%v", ud, udLeft)
		// A failure between RegisterProduct and PublishProduct leaves only
		// the UserData registration (state 2), so count every key and state.
		registered := !strings.Contains(pub, "ERROR") || udLeft || state != -1
		u := runMsiexec(c, "uninstall.log", "/x", p.ProductCode)
		c.logf("uninstall by ProductCode exit %d; state after %d", u.Exit, productState(p.ProductCode))
		left := !strings.Contains(regQuery(hku), "ERROR") || !strings.Contains(regQuery(ud), "ERROR")
		c.logf("RESULT %s: install %d, registration left after failed install=%v", name, r.Exit, registered)
		if left {
			c.logf("  cleanup: deleting leftover registration keys")
			for _, k := range []string{hku, ud} {
				if err := exec.Command("reg.exe", "delete", k, "/f").Run(); err != nil {
					c.logf("  reg delete %s: %v", k, err)
				}
			}
			c.logf("  after cleanup state %d", productState(p.ProductCode))
		}
		c.close()
	}
	_ = syscall.EINVAL
}
