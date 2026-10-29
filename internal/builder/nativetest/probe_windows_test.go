//go:build windows

package nativetest_test

// Temporary probe for the WiX issue triage: every observation is reported
// with t.Errorf so that CI shows it. Replaced by permanent tests before merge.

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/ice"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

var pMapFileAndCheckSumW = syscall.NewLazyDLL("imagehlp.dll").NewProc("MapFileAndCheckSumW") //nolint:gochecknoglobals // probe

type probeReport struct{ lines []string }

func (r *probeReport) add(format string, args ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func probeBuild(t *testing.T, dir, name, yaml string, inputs map[string][]byte) (string, string) {
	t.Helper()

	for file, data := range inputs {
		p := filepath.Join(dir, file)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)

		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(yaml), config.Options{})
	if err != nil {
		return "", "parse: " + err.Error()
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		return "", fmt.Sprintf("build: %v %v", err, res.Diagnostics)
	}

	path := filepath.Join(dir, name+".msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, fmt.Sprint(res.Diagnostics)
}

func probeMsiexec(log string, args ...string) int {
	//nolint:gosec // test paths
	err := exec.CommandContext(context.Background(), "msiexec", append(args, "/qn", "/l*v", log)...).Run()
	if exit, ok := err.(*exec.ExitError); ok { //nolint:errorlint // probe
		return exit.ExitCode()
	}

	if err != nil {
		return -1
	}

	return 0
}

func grepLog(log string, re *regexp.Regexp) []string {
	data := readLog(log)

	var out []string

	for line := range strings.Lines(data) {
		if re.MatchString(line) {
			out = append(out, strings.TrimSpace(line))
		}
	}

	if len(out) > 12 {
		out = out[:12]
	}

	return out
}

func TestProbeWixIssues(t *testing.T) { //nolint:paralleltest // msiexec
	if testing.Short() {
		t.Skip("probe")
	}

	r := &probeReport{}

	probeDowngradedFile(t, r)
	probeLeadingDot(t, r)
	probeChecksum(t, r)
	probeFirewallStopped(t, r)

	t.Errorf("PROBE REPORT\n%s", strings.Join(r.lines, "\n"))
}

const probeUpgradeYAML = `schema: msi-builder/v1
product: { name: MsiBuilderProbeUpg, manufacturer: msi-builder tests, version: %s, upgradeCode: 3c1d7e52-8a40-4f6b-9b2e-5d7a1c0e9f34 }
package: { platform: x64 }
installDir: { name: MsiBuilderProbeUpg }
upgrade: { removeOld: %s }
components:
  - files: [{ source: app.exe }]
  - files: [{ source: lib.dll }]
`

var reAction = regexp.MustCompile(`(?i)(Action \d+:\d+:\d+: InstallFiles|\[1\]|Won't Overwrite|Disallowing|lib\.dll)`)

func probeDowngradedFile(t *testing.T, r *probeReport) {
	t.Helper()

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	libNew := testpe.Build(testpe.Options{FileVersion: "2.0.0.0", Languages: []int{1033}, Padding: 1})
	libOld := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}, Padding: 2})
	installed := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderProbeUpg", "lib.dll")

	for _, variant := range []struct{ removeOld, extra string }{
		{"afterInstallValidate", ""},
		{"afterInstallInitialize", ""},
		{"afterInstallExecute", ""},
		{"afterInstallInitialize", "REINSTALLMODE=amus"},
	} {
		dir := t.TempDir()
		v1, d1 := probeBuild(t, filepath.Join(dir, "v1"), "v1", fmt.Sprintf(probeUpgradeYAML, "1.0.0", variant.removeOld), map[string][]byte{"app.exe": app, "lib.dll": libNew})
		v2, d2 := probeBuild(t, filepath.Join(dir, "v2"), "v2", fmt.Sprintf(probeUpgradeYAML, "1.1.0", variant.removeOld), map[string][]byte{"app.exe": app, "lib.dll": libOld})

		if v1 == "" || v2 == "" {
			r.add("UPG %s: build failed %s %s", variant.removeOld, d1, d2)

			continue
		}

		log1 := filepath.Join(dir, "i1.log")
		log2 := filepath.Join(dir, "i2.log")
		e1 := probeMsiexec(log1, "/i", v1)

		args := []string{"/i", v2}
		if variant.extra != "" {
			args = append(args, variant.extra)
		}

		e2 := probeMsiexec(log2, args...)
		data, err := os.ReadFile(installed)

		state := "missing"

		switch {
		case err == nil && bytes.Equal(data, libNew):
			state = "v1 file (2.0.0.0) kept"
		case err == nil && bytes.Equal(data, libOld):
			state = "v2 file (1.0.0.0) installed"
		case err == nil:
			state = "other bytes"
		}

		r.add("UPG removeOld=%s %s: install=%d upgrade=%d lib.dll %s", variant.removeOld, variant.extra, e1, e2, state)

		if variant.removeOld == "afterInstallValidate" {
			r.add("  install log lines: %q", grepLog(log1, regexp.MustCompile(`Action \d+:\d+:\d+: (InstallFiles|RemoveFiles|WriteRegistryValues)\.`)))
		}

		r.add("  upgrade log: %q", grepLog(log2, reAction))

		// Repair after the upgrade.
		e3 := probeMsiexec(filepath.Join(dir, "r.log"), "/fa", v2)
		_, err = os.Stat(installed)
		r.add("  repair /fa=%d lib.dll exists=%v", e3, err == nil)

		e4 := probeMsiexec(filepath.Join(dir, "x.log"), "/x", v2)
		r.add("  uninstall=%d", e4)
		_ = probeMsiexec(filepath.Join(dir, "x1.log"), "/x", v1)
		_ = os.RemoveAll(filepath.Dir(installed))
	}
}

const probeDotYAML = `schema: msi-builder/v1
product: { name: MsiBuilderProbeDot, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 6e2a9c41-0b7d-4e85-a1f3-2c9d8b7e5a16 }
package: { platform: x64 }
installDir: { name: MsiBuilderProbeDot }
components:
  - files: [{ source: app.exe }, { source: .gitignore }, { source: .a.b.c }]
  - directory: .git
    files: [{ source: config, name: config }]
  - directory: .config/.x
    files: [{ source: y.txt }]
`

func probeLeadingDot(t *testing.T, r *probeReport) {
	t.Helper()

	dir := t.TempDir()
	path, diags := probeBuild(t, dir, "dot", probeDotYAML, map[string][]byte{
		"app.exe":    testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
		".gitignore": []byte("x\n"),
		".a.b.c":     []byte("abc\n"),
		"config":     []byte("[core]\n"),
		"y.txt":      []byte("y\n"),
	})

	if path == "" {
		r.add("DOT: %s", diags)

		return
	}

	db := openDB(t, path)
	r.add("DOT Directory: %q", query(t, db, "SELECT `Directory`, `DefaultDir` FROM `Directory`"))
	r.add("DOT File: %q", query(t, db, "SELECT `File`, `FileName` FROM `File`"))
	closeHandle(db)

	if cub := os.Getenv("MSI_BUILDER_ICE_CUB"); cub != "" {
		findings, err := ice.Run(path, cub)
		for _, f := range findings {
			if f.Type == ice.Error || f.Type == ice.Failure || f.Type == ice.Warning {
				r.add("DOT ICE: %v", f)
			}
		}

		r.add("DOT ICE err=%v findings=%d", err, len(findings))
	} else {
		r.add("DOT ICE: no cub")
	}

	root := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderProbeDot")
	log := filepath.Join(dir, "i.log")
	e := probeMsiexec(log, "/i", path)

	var found []string

	_ = filepath.Walk(root, func(p string, info os.FileInfo, _ error) error {
		if info != nil {
			rel, _ := filepath.Rel(root, p)
			found = append(found, rel)
		}

		return nil
	})
	r.add("DOT install=%d tree=%q", e, found)

	if e != 0 {
		r.add("DOT log: %q", grepLog(log, regexp.MustCompile(`(?i)error|Note: 1: `)))
	}

	ex := probeMsiexec(filepath.Join(dir, "x.log"), "/x", path)
	_, err := os.Stat(root)
	r.add("DOT uninstall=%d root left=%v", ex, err == nil)
	_ = os.RemoveAll(root)
}

const probeChecksumYAML = `schema: msi-builder/v1
product: { name: MsiBuilderProbeCrc, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 9a4f1e27-6c3b-4d08-b5e9-7f2a0c8d1b63 }
package: { platform: x64 }
installDir: { name: MsiBuilderProbeCrc }
components:
  - files: [{ source: app.exe, checksum: true }]
`

// peChecksum patches the PE checksum of data: the correct one, or a wrong one.
func peChecksum(t *testing.T, dir string, data []byte, correct bool) []byte {
	t.Helper()

	off := int(binary.LittleEndian.Uint32(data[0x3C:])) + 24 + 64
	out := bytes.Clone(data)

	if !correct {
		binary.LittleEndian.PutUint32(out[off:], 0x12345678)

		return out
	}

	binary.LittleEndian.PutUint32(out[off:], 0)
	p := filepath.Join(dir, "crc.tmp")

	if err := os.WriteFile(p, out, 0o600); err != nil {
		t.Fatal(err)
	}

	var header, sum uint32

	name, _ := syscall.UTF16PtrFromString(p)
	rc, _, _ := pMapFileAndCheckSumW.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&header)), uintptr(unsafe.Pointer(&sum)))

	if rc != 0 {
		t.Fatalf("MapFileAndCheckSumW: %d", rc)
	}

	binary.LittleEndian.PutUint32(out[off:], sum)

	return out
}

func probeChecksum(t *testing.T, r *probeReport) {
	t.Helper()

	base := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}, Checksum: 1})
	installed := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderProbeCrc", "app.exe")

	for _, v := range []struct {
		correct bool
		prop    string
	}{
		{false, ""}, {false, "MSICHECKCRCS=1"}, {true, "MSICHECKCRCS=1"}, {true, ""},
	} {
		dir := t.TempDir()
		path, diags := probeBuild(t, dir, "crc", probeChecksumYAML, map[string][]byte{"app.exe": peChecksum(t, dir, base, v.correct)})

		if path == "" {
			r.add("CRC: %s", diags)

			return
		}

		db := openDB(t, path)
		attrs := query(t, db, "SELECT `Attributes` FROM `File`")
		closeHandle(db)

		log := filepath.Join(dir, "i.log")
		args := []string{"/i", path}

		if v.prop != "" {
			args = append(args, v.prop)
		}

		e := probeMsiexec(log, args...)
		_, err := os.Stat(installed)
		r.add("CRC correct=%v %q attrs=%v: install=%d exists=%v log=%q", v.correct, v.prop, attrs, e, err == nil,
			grepLog(log, regexp.MustCompile(`(?i)checksum|crc|Error 1|Note: 1: 13`)))

		// Repair with the c mode as well.
		if e == 0 {
			ec := probeMsiexec(filepath.Join(dir, "r.log"), "/fc", path)
			r.add("  repair /fc=%d", ec)
		}

		_ = probeMsiexec(filepath.Join(dir, "x.log"), "/x", path)
		_ = os.RemoveAll(filepath.Dir(installed))
	}
}

const probeFirewallYAML = `schema: msi-builder/v1
product: { name: MsiBuilderProbeFw, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 1f7c3a85-2d6e-4b19-8e40-a3c5b9d2e706 }
package: { platform: x64 }
installDir: { name: MsiBuilderProbeFw }
components:
  - id: App
    files: [{ source: app.exe }]
features:
  - id: Main
    title: Main
    components: [App]
    firewall:
      - { name: MsiBuilderProbeFw, protocol: tcp, port: "39182", program: "[#app.exe]" }
`

func probeFirewallStopped(t *testing.T, r *probeReport) {
	t.Helper()

	dir := t.TempDir()
	path, diags := probeBuild(t, dir, "fw", probeFirewallYAML, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	if path == "" {
		r.add("FW: %s", diags)

		return
	}

	out, err := exec.CommandContext(context.Background(), "sc", "stop", "mpssvc").CombinedOutput()
	r.add("FW sc stop mpssvc: %v %s", err, strings.Join(strings.Fields(string(out)), " "))

	defer func() {
		out, err := exec.CommandContext(context.Background(), "sc", "start", "mpssvc").CombinedOutput()
		r.add("FW sc start mpssvc: %v %s", err, strings.Join(strings.Fields(string(out)), " "))
	}()

	q, _ := exec.CommandContext(context.Background(), "sc", "query", "mpssvc").CombinedOutput()
	r.add("FW state: %s", strings.Join(strings.Fields(string(q)), " "))

	n, _ := exec.CommandContext(context.Background(), "netsh", "advfirewall", "firewall", "show", "rule", "name=MsiBuilderProbeFw").CombinedOutput()
	r.add("FW netsh show while stopped: %s", strings.Join(strings.Fields(string(n)), " "))

	log := filepath.Join(dir, "i.log")
	e := probeMsiexec(log, "/i", path)
	r.add("FW install while stopped=%d log=%q", e, grepLog(log, regexp.MustCompile(`(?i)FwAdd|FwClear|Return value 3|Error 17`)))

	x := probeMsiexec(filepath.Join(dir, "x.log"), "/x", path)
	r.add("FW uninstall=%d", x)
	_ = os.RemoveAll(filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderProbeFw"))
}
