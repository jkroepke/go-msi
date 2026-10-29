//go:build windows

package nativetest_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// fwAllProbe is a temporary probe (MSISPEC 17.3): how netsh treats the rule
// name all on add and on delete. Every rule it touches names the program
// C:\msi-builder-probe\probe.exe, and every delete but the last carries
// that program. Rules are found by display name, because per-rule filter
// queries over all rules took minutes. The last delete is unscoped and may
// remove every inbound rule: run this on a throwaway CI runner only, never
// on a dev machine.
const fwAllProbe = `$ErrorActionPreference = 'Continue'
$prog = 'C:\msi-builder-probe\probe.exe'
$names = @('msibuilder-probe-A', 'msibuilder-probe-B', ' all', 'all ')
function Mine() {
  (Get-NetFirewallRule -DisplayName $names -ErrorAction SilentlyContinue | ForEach-Object { "[" + $_.DisplayName + "]" }) -join ' '
}
function Probe([string]$label, [string]$cmdline) {
  $out = cmd /c "netsh advfirewall firewall $cmdline 2>&1"
  "== $label :: netsh advfirewall firewall $cmdline :: exit $LASTEXITCODE"
  ($out | Where-Object { $_ -ne '' }) -join ' | '
  "   probe rules now: " + (Mine)
}
function Reset() {
  Get-NetFirewallRule -DisplayName $names -ErrorAction SilentlyContinue | Remove-NetFirewallRule
  cmd /c "netsh advfirewall firewall add rule name=""msibuilder-probe-A"" dir=in action=allow program=""$prog"" enable=yes" | Out-Null
  cmd /c "netsh advfirewall firewall add rule name=""msibuilder-probe-B"" dir=in action=allow program=""$prog"" enable=yes" | Out-Null
}
$before = (Get-NetFirewallRule).Count
Reset
foreach ($n in @('all', '"all"', 'ALL', '"All"', '" all"', '"all "')) {
  Probe "add $n" "add rule name=$n dir=in action=allow program=""$prog"""
}
Probe "delete name="" all"" (rule ' all' exists)" "delete rule name="" all"" dir=in program=""$prog"""
Probe "delete name=""all "" (rule 'all ' exists)" "delete rule name=""all "" dir=in program=""$prog"""
foreach ($n in @('"all"', 'ALL', '\"all\"', "'all'")) {
  Reset
  Probe "delete $n" "delete rule name=$n dir=in program=""$prog"""
}
Reset
Probe "delete name=all no dir" "delete rule name=all program=""$prog"""
Reset
# The throwaway runner only: an unscoped delete, to see whether it
# removes every inbound rule.
$inBefore = (Get-NetFirewallRule -Direction Inbound).Count
$outBefore = (Get-NetFirewallRule -Direction Outbound).Count
Probe "UNSCOPED delete name=all dir=in" "delete rule name=all dir=in"
"inbound rules before the unscoped delete $inBefore, after " + (Get-NetFirewallRule -Direction Inbound).Count
"outbound rules before the unscoped delete $outBefore, after " + (Get-NetFirewallRule -Direction Outbound).Count
Get-NetFirewallRule -DisplayName $names -ErrorAction SilentlyContinue | Remove-NetFirewallRule
"rules before $before, after " + (Get-NetFirewallRule).Count
`

// TestFirewallAllProbe reports what netsh did; it fails on purpose so the
// log is read and the probe replaced by an assertion before merging.
func TestFirewallAllProbe(t *testing.T) { //nolint:paralleltest // changes firewall rules
	if testing.Short() {
		t.Skip("changes firewall rules")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", fwAllProbe).CombinedOutput()
	t.Errorf("FW-ALL probe (err %v):\n%s", err, out)
}
