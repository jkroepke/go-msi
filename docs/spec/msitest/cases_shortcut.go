package main

// RUN-28: shortcuts. Which component layouts install and remove a
// non-advertised Start menu and desktop shortcut, what the .lnk holds, and
// whether an advertised shortcut and an icon work.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() { cases["RUN-28"] = caseRun28 }

const shortcutRegKey = `Software\MsiSpecTest\shortcut`

// testIcon is a 16x16, 32-bit ICO file with one image.
func testIcon() []byte {
	var b bytes.Buffer
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	const size = 40 + 16*16*4 + 16*4
	le([]uint16{0, 1, 1})                       // ICONDIR: reserved, type icon, one image
	le([]uint8{16, 16, 0, 0})                   // width, height, colours, reserved
	le([]uint16{1, 32})                         // planes, bit count
	le([]uint32{size, 22})                      // bytes in the image, offset
	le([]uint32{40, 16, 32})                    // BITMAPINFOHEADER: size, width, height (XOR + AND)
	le([]uint16{1, 32})                         // planes, bit count
	le([]uint32{0, 16*16*4 + 16*4, 0, 0, 0, 0}) // compression, image size, resolution, colours
	for i := 0; i < 16*16; i++ {
		le([]uint8{0x40, 0x80, 0xE0, 0xFF}) // BGRA
	}
	b.Write(make([]byte, 16*4)) // AND mask: all opaque
	return b.Bytes()
}

// lnkInfo reads a .lnk through the shell's own parser.
func lnkInfo(path string) string {
	if !exists(path) {
		return "<absent>"
	}
	ps := fmt.Sprintf(`$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%s'); "target=$($s.TargetPath) args=$($s.Arguments) wkdir=$($s.WorkingDirectory) desc=$($s.Description) icon=$($s.IconLocation) show=$($s.WindowStyle)"`, path)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		return "ERR " + err.Error() + " " + oneLine(string(out))
	}
	return strings.TrimSpace(string(out))
}

func hkcuValue(key, name string) string {
	out := cmdOut("reg.exe", "query", `HKCU\`+key, "/v", name)
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) >= 3 && f[0] == name {
			return strings.Join(f[2:], " ")
		}
	}
	return "<absent>"
}

// caseRun28 installs and removes shortcuts in four layouts:
//
//   - hkcu-folder: per-machine; a shortcut component of its own with an HKCU
//     registry key path (ICE38, ICE43), a Start menu subfolder removed by a
//     RemoveFile row (ICE64), a desktop shortcut, arguments, description,
//     working folder, minimized show command and an Icon row.
//   - file-component: per-machine; the shortcuts live in the file's own
//     component (file key path, HKLM), directly in ProgramMenuFolder and
//     DesktopFolder.
//   - file-component-folder: as file-component, but in a Start menu subfolder
//     with no RemoveFile row: is the folder removed at uninstall?
//   - advertised: Target is the feature, with an icon, in the file's component.
//   - per-user: ALLUSERS absent, the hkcu-folder layout; where does it land?
func caseRun28() {
	appData, err := os.ReadFile(filepath.Join(os.Getenv("SystemRoot"), "System32", "whoami.exe"))
	if err != nil {
		fmt.Println("RUN-28: cannot read whoami.exe:", err)
		return
	}
	icon := testIcon()
	commonPrograms := filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`)
	commonDesktop := filepath.Join(os.Getenv("PUBLIC"), "Desktop")
	userPrograms := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`)
	userDesktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")

	for _, v := range []struct {
		name       string
		perUser    bool
		ownComp    bool // shortcut component with an HKCU key path
		subfolder  bool
		removeFile bool
		advertised bool
		withIcon   bool
	}{
		{name: "hkcu-folder", ownComp: true, subfolder: true, removeFile: true, withIcon: true},
		{name: "file-component"},
		{name: "file-component-folder", subfolder: true},
		{name: "advertised", withIcon: true},
		{name: "per-user", perUser: true, ownComp: true, subfolder: true, removeFile: true, withIcon: true},
	} {
		c := newCase("RUN-28", v.name)
		p := &pkg{Name: "sc", Is64: true, PerUser: v.perUser}
		p.Files = []payload{{Key: "f_app", Name: "app.exe", Data: appData}}
		p.Extra = map[string][][]string{
			"Directory": {
				{"ProgramMenuFolder", "TARGETDIR", "."},
				{"DesktopFolder", "TARGETDIR", "."},
				{"SCDIR", "ProgramMenuFolder", "msispect|MsiSpecTest"},
			},
		}
		p.ExtraSequence = [][]string{{"RemoveShortcuts", "", "3200"}, {"CreateShortcuts", "", "4500"}}

		comp := "C_f_app"
		if v.ownComp {
			comp = "C_sc"
			p.Extra["Component"] = [][]string{{comp, guidFor("sc-comp" + v.name), "INSTALLDIR", fmt.Sprint(attr64 | attrRegKeyPath), "", "R_sc"}}
			p.Extra["Registry"] = [][]string{{"R_sc", "1", shortcutRegKey, "installed", "#1", comp}}
			p.Extra["FeatureComponents"] = [][]string{{"Main", comp}}
		}
		startDir := "ProgramMenuFolder"
		if v.subfolder {
			startDir = "SCDIR"
		}
		if v.removeFile {
			p.Extra["RemoveFile"] = [][]string{{"RF_scdir", comp, "", "SCDIR", "2"}}
		}
		target, iconRef, iconIndex := "[#f_app]", "", ""
		if v.advertised {
			target = "Main"
		}
		if v.withIcon {
			p.Icons = []payload{{Key: "app.ico", Data: icon}}
			iconRef, iconIndex = "app.ico", "0"
		}
		p.Extra["Shortcut"] = [][]string{
			{"S_start", startDir, "msispeca|MsiSpec App", comp, target, "/user", "MsiSpec shortcut test", "", iconRef, iconIndex, "7", "INSTALLDIR", "", "", "", ""},
			{"S_desk", "DesktopFolder", "msispeca|MsiSpec App", comp, target, "", "", "", iconRef, iconIndex, "", "", "", "", "", ""},
		}

		programs, desktop := commonPrograms, commonDesktop
		if v.perUser {
			programs, desktop = userPrograms, userDesktop
		}
		startLnk := filepath.Join(programs, "MsiSpec App.lnk")
		if v.subfolder {
			startLnk = filepath.Join(programs, "MsiSpecTest", "MsiSpec App.lnk")
		}
		deskLnk := filepath.Join(desktop, "MsiSpec App.lnk")
		// Any leftover from an earlier run would hide a missing shortcut.
		for _, l := range []string{startLnk, deskLnk, filepath.Join(userPrograms, "MsiSpecTest", "MsiSpec App.lnk"), filepath.Join(userDesktop, "MsiSpec App.lnk")} {
			os.Remove(l)
		}
		os.Remove(filepath.Join(programs, "MsiSpecTest"))

		m := control(c, p, "pkg.msi")
		if m == "" {
			c.close()
			continue
		}
		r := runMsiexec(c, "install.log", "/i", m)
		iconFile := filepath.Join(os.Getenv("SystemRoot"), "Installer", p.ProductCode, "app.ico")
		if v.perUser {
			// A per-user install keeps its icons in the user's profile.
			iconFile = filepath.Join(os.Getenv("APPDATA"), `Microsoft\Installer`, p.ProductCode, "app.ico")
		}
		c.logf("install exit %d", r.Exit)
		c.logf("  start menu %s: %s", startLnk, lnkInfo(startLnk))
		c.logf("  desktop %s: %s", deskLnk, lnkInfo(deskLnk))
		if v.perUser {
			c.logf("  common start menu copy present=%v", exists(filepath.Join(commonPrograms, "MsiSpecTest", "MsiSpec App.lnk")))
		} else {
			c.logf("  per-user start menu copy present=%v", exists(filepath.Join(userPrograms, "MsiSpecTest", "MsiSpec App.lnk")) || exists(filepath.Join(userPrograms, "MsiSpec App.lnk")))
		}
		if v.withIcon {
			c.logf("  icon file %s present=%v", iconFile, exists(iconFile))
		}
		if v.ownComp {
			c.logf("  HKCU key path value: %s", hkcuValue(shortcutRegKey, "installed"))
		}
		installed := r.Exit == 0 && exists(startLnk) && exists(deskLnk)

		u := runMsiexec(c, "uninstall.log", "/x", p.ProductCode)
		folderLeft := v.subfolder && exists(filepath.Join(programs, "MsiSpecTest"))
		c.logf("uninstall exit %d; start menu lnk left=%v, desktop lnk left=%v, start menu folder left=%v", u.Exit, exists(startLnk), exists(deskLnk), folderLeft)
		if v.withIcon {
			c.logf("  icon file left=%v", exists(iconFile))
		}
		if v.ownComp {
			c.logf("  HKCU key path value after uninstall: %s", hkcuValue(shortcutRegKey, "installed"))
		}
		removed := u.Exit == 0 && !exists(startLnk) && !exists(deskLnk) && !folderLeft
		c.logf("RESULT %s: installed=%v removed=%v", v.name, installed, removed)

		removeProduct(c, p.ProductCode)
		os.Remove(startLnk)
		os.Remove(deskLnk)
		os.Remove(filepath.Join(programs, "MsiSpecTest"))
		exec.Command("reg.exe", "delete", `HKCU\`+shortcutRegKey, "/f").Run()
		c.close()
	}
}
