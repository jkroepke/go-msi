//go:build windows

package nativetest_test

import (
	"os/exec"
	"testing"
)

// fwAllProbe is a temporary probe (MSISPEC 17.3): how netsh treats the rule
// name all on add and on delete. Every rule it touches names the program
// C:\msi-builder-probe\probe.exe, and every delete but the last carries
// that program. The last one is unscoped and may remove every inbound rule:
// run this on a throwaway CI runner only, never on a dev machine.
const fwAllProbe = `$ErrorActionPreference = 'Continue'
$prog = 'C:\msi-builder-probe\probe.exe'
function Probe([string]$label, [string]$cmdline) {
  $out = cmd /c "netsh advfirewall firewall $cmdline 2>&1"
  "== $label :: netsh advfirewall firewall $cmdline :: exit $LASTEXITCODE"
  ($out | Where-Object { $_ -ne '' }) -join ' | '
  $mine = Get-NetFirewallRule -ErrorAction SilentlyContinue | Where-Object {
    (Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $_).Program -eq $prog
  } | ForEach-Object { "[" + $_.DisplayName + "]" }
  "   probe rules now: " + ($mine -join ' ')
}
function Reset() {
  Get-NetFirewallRule -ErrorAction SilentlyContinue | Where-Object {
    (Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $_).Program -eq $prog
  } | Remove-NetFirewallRule
  cmd /c "netsh advfirewall firewall add rule name=""msibuilder-probe-A"" dir=in action=allow program=""$prog"" enable=yes" | Out-Null
  cmd /c "netsh advfirewall firewall add rule name=""msibuilder-probe-B"" dir=in action=allow program=""$prog"" enable=yes" | Out-Null
}
$before = (Get-NetFirewallRule).Count
Reset
foreach ($n in @('all', '"all"', 'ALL', '"All"', '" all"', '"all "')) {
  Probe "add $n" "add rule name=$n dir=in action=allow program=""$prog"""
}
foreach ($n in @('"all"', 'ALL', '\"all\"', "'all'")) {
  Reset
  Probe "delete $n" "delete rule name=$n dir=in program=""$prog"""
}
Reset
Probe "delete name=all no dir" "delete rule name=all program=""$prog"""
Reset
Probe "show name=all" "show rule name=all dir=in program=""$prog"""
Reset
# The throwaway runner only: an unscoped delete, to see whether it
# removes every inbound rule.
$inBefore = (Get-NetFirewallRule -Direction Inbound).Count
Probe "UNSCOPED delete name=all dir=in" "delete rule name=all dir=in"
"inbound rules before the unscoped delete $inBefore, after " + (Get-NetFirewallRule -Direction Inbound).Count
Get-NetFirewallRule -ErrorAction SilentlyContinue | Where-Object {
  (Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $_).Program -eq $prog
} | Remove-NetFirewallRule
"rules before $before, after " + (Get-NetFirewallRule).Count
`

// TestFirewallAllProbe reports what netsh did; it fails on purpose so the
// log is read and the probe replaced by an assertion before merging.
func TestFirewallAllProbe(t *testing.T) { //nolint:paralleltest // changes firewall rules
	if testing.Short() {
		t.Skip("changes firewall rules")
	}

	out, err := exec.CommandContext(t.Context(), "powershell", "-NoProfile", "-NonInteractive", "-Command", fwAllProbe).CombinedOutput()
	t.Errorf("FW-ALL probe (err %v):\n%s", err, out)
}
