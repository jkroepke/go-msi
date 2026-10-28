//go:build windows

package nativetest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// TestNetshProbe is a temporary probe: it logs how netsh treats remote
// address keywords and quotes in a description. It fails on purpose so that its output shows in CI.
func TestNetshProbe(t *testing.T) {
	t.Parallel()

	if !elevated() {
		t.Skip("needs elevation")
	}

	netsh := filepath.Join(os.Getenv("SystemRoot"), "System32", "netsh.exe")
	base := `"` + netsh + `" advfirewall firewall `
	probes := []string{
		`name="MBProbe1" dir=in action=allow protocol=tcp localport=65001 remoteip=any,10.0.0.1`,
		`name="MBProbe2" dir=in action=allow protocol=tcp localport=65001 remoteip=10.0.0.1,any`,
		`name="MBProbe3" dir=in action=allow protocol=tcp localport=65001 remoteip=*`,
		`name="MBProbe4" dir=in action=allow protocol=tcp localport=65001 remoteip=*,10.0.0.1`,
		`name="MBProbe5" dir=in action=allow protocol=tcp localport=65001 remoteip=localsubnet,10.0.0.1`,
		`name="MBProbe6" dir=in action=allow protocol=tcp localport=65001 remoteip=ANY`,
		`name="MBProbe7" dir=in action=allow protocol=tcp localport=65001 description="say "hi" now"`,
		`name="MBProbe8" dir=in action=allow protocol=tcp localport=65001 description="plain text"`,
		`name="MBProbe9" dir=in action=allow protocol=tcp localport=65001 remoteip=LocalSubnet,dns`,
	}

	run := func(args string) {
		cmd := exec.CommandContext(t.Context(), netsh)
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: base + args}
		out, err := cmd.CombinedOutput()
		t.Errorf("PROBE %s\nerr=%v\n%s", args, err, out)
	}

	for i, probe := range probes {
		run("add rule " + probe)
		name := "MBProbe" + string(rune('1'+i))
		run(`show rule name="` + name + `" verbose`)
		run(`delete rule name="` + name + `"`)
	}
}
