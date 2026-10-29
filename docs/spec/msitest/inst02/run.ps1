# INST-02: install contexts of a dual-scope package (spec section 8.2).
#
# Builds the probe package with msi-builder from this checkout in both dual
# scopes and two versions, installs each variant with msiexec, records where
# files, registry values, the shortcut and the product registration land,
# and the probe properties the log shows, then uninstalls.
#
#   powershell -ExecutionPolicy Bypass -File run.ps1 [-Only name,...]
#
# Run it elevated, unless -NonAdmin is given: then it runs the context
# variants as the current (non-administrator) user. The report goes to
# ..\results\INST-02\<admin|nonadmin>\report.md (not committed; log.md
# beside it is the committed summary), the msiexec logs to %TEMP%\inst02.
param([string[]]$Only, [switch]$NonAdmin)
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$repo = Resolve-Path (Join-Path $here '..\..\..\..')
$work = Join-Path $env:TEMP 'inst02'
New-Item -ItemType Directory -Force $work | Out-Null
$out = Join-Path $here "..\results\INST-02\$(if ($NonAdmin) { 'nonadmin' } else { 'admin' })"
New-Item -ItemType Directory -Force $out | Out-Null

if (-not (Test-Path "$work\msi-builder.exe")) {
  Push-Location $repo
  go build -o "$work\msi-builder.exe" ./cmd/msi-builder
  if ($LASTEXITCODE) { throw 'go build failed' }
  Pop-Location
}
Copy-Item "$env:windir\System32\whoami.exe" "$work\hello.exe" -Force
Copy-Item "$here\probe.yaml" "$work\probe.yaml" -Force
foreach ($s in 'perUserOrMachine', 'perMachineOrUser') {
  foreach ($v in '1.0.0', '2.0.0') {
    & "$work\msi-builder.exe" build -q --var SCOPE=$s --var VERSION=$v -o "$work\$s-$v.msi" "$work\probe.yaml"
    if ($LASTEXITCODE) { throw "build $s $v failed" }
  }
}

function Squash([string]$g) {
  $h = $g.Trim('{}').Replace('-', '')
  $r = { param($x) -join ($x.ToCharArray()[($x.Length - 1)..0]) }
  $o = (& $r $h.Substring(0, 8)) + (& $r $h.Substring(8, 4)) + (& $r $h.Substring(12, 4))
  for ($i = 16; $i -lt 32; $i += 2) { $o += & $r $h.Substring($i, 2) }
  $o
}

function ProductCode([string]$msi) {
  $i = New-Object -ComObject WindowsInstaller.Installer
  $db = $i.GetType().InvokeMember('OpenDatabase', 'InvokeMethod', $null, $i, @($msi, 0))
  $view = $db.GetType().InvokeMember('OpenView', 'InvokeMethod', $null, $db, @("SELECT Value FROM Property WHERE Property='ProductCode'"))
  $view.GetType().InvokeMember('Execute', 'InvokeMethod', $null, $view, $null) | Out-Null
  $rec = $view.GetType().InvokeMember('Fetch', 'InvokeMethod', $null, $view, $null)
  $code = $rec.GetType().InvokeMember('StringData', 'GetProperty', $null, $rec, 1)
  $view.GetType().InvokeMember('Close', 'InvokeMethod', $null, $view, $null) | Out-Null
  $code
}

function Msiexec([string]$log, [string[]]$msiArgs) {
  $all = $msiArgs + @('/l*vx', $log)
  $p = Start-Process msiexec.exe -ArgumentList $all -PassThru
  if (-not $p.WaitForExit(240000)) { $p.Kill(); return -1 }
  $p.ExitCode
}

function State($lines, [string]$code) {
  $sq = Squash $code
  $user = Join-Path $env:LOCALAPPDATA 'Programs\ScopeProbe\hello.exe'
  $machine = Join-Path $env:ProgramFiles 'ScopeProbe\hello.exe'
  $lines.Add("  files: machine=$(Test-Path $machine) user=$(Test-Path $user)")
  $hkmuL = Test-Path 'HKLM:\Software\MsiSpecProbe\ScopeProbe'
  $hkmuU = Test-Path 'HKCU:\Software\MsiSpecProbe\ScopeProbe'
  $lines.Add("  HKMU value: HKLM=$hkmuL HKCU=$hkmuU")
  $crL = Test-Path 'HKLM:\Software\Classes\MsiSpecScopeProbe.File'
  $crU = Test-Path 'HKCU:\Software\Classes\MsiSpecScopeProbe.File'
  $lines.Add("  HKCR value: HKLM\Software\Classes=$crL HKCU\Software\Classes=$crU")
  $lnkM = Test-Path "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\ScopeProbe.lnk"
  $lnkU = Test-Path "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\ScopeProbe.lnk"
  $lines.Add("  shortcut: all users=$lnkM user=$lnkU")
  $pM = Test-Path "HKLM:\SOFTWARE\Classes\Installer\Products\$sq"
  $pU = Test-Path "HKCU:\Software\Microsoft\Installer\Products\$sq"
  $arpM = Test-Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\$code"
  $lines.Add("  registration: Installer\Products machine=$pM user=$pU; HKLM Uninstall key=$arpM")
}

function Probe($lines, [string]$log) {
  if (-not (Test-Path $log)) { $lines.Add('  no log'); return }
  $text = Get-Content $log -Raw
  foreach ($m in [regex]::Matches($text, 'MSI \((c|s)\) \([0-9A-F:]+\) \[[0-9:]+\]: PROPERTY CHANGE: [^\r\n]*?(PROBEBEFORE|PROBEAFTER|ALLUSERS|MSIINSTALLPERUSER|ProgramFiles64Folder|INSTALLDIR|ProgramMenuFolder) property[^\r\n]*')) {
    $lines.Add("  log: $($m.Value)")
  }
  foreach ($m in [regex]::Matches($text, 'MSI \((c|s)\) [^\r\n]*Doing action: (ProbeBefore|ProbeAfter|ProbeSwitch\w+)')) { $lines.Add("  ran: $($m.Value)") }
  foreach ($m in [regex]::Matches($text, '(?m)^Property\((C|S)\): (ALLUSERS|MSIINSTALLPERUSER|Privileged|PROBEBEFORE|PROBEAFTER|INSTALLDIR|ProgramFiles64Folder) = [^\r\n]*')) { $lines.Add("  end: $($m.Value)") }
  foreach ($m in [regex]::Matches($text, '(Product: [^\r\n]*|Windows Installer (re)?configured the product[^\r\n]*|Note: 1: 1925[^\r\n]*|Error (?!4:)\d+[^\r\n]*)')) { $lines.Add("  msg: $($m.Value)") }
}

$variants = @(
  @{ n = 'user-default';        pkg = 'perUserOrMachine-1.0.0'; args = @() },
  @{ n = 'user-to-machine';     pkg = 'perUserOrMachine-1.0.0'; args = @('MSIINSTALLPERUSER=""') },
  @{ n = 'user-allusers1';      pkg = 'perUserOrMachine-1.0.0'; args = @('ALLUSERS=1') },
  @{ n = 'machine-default';     pkg = 'perMachineOrUser-1.0.0'; args = @() },
  @{ n = 'machine-to-user';     pkg = 'perMachineOrUser-1.0.0'; args = @('MSIINSTALLPERUSER=1') },
  @{ n = 'qr-user';             pkg = 'perUserOrMachine-1.0.0'; args = @(); ui = '/qr' },
  @{ n = 'qr-user-switch-pu';   pkg = 'perUserOrMachine-1.0.0'; args = @('PROBE_SWITCH=1', 'PROBE_AU=2'); ui = '/qr' },
  @{ n = 'qr-user-switch-au1';  pkg = 'perUserOrMachine-1.0.0'; args = @('PROBE_SWITCH=1', 'PROBE_AU=1'); ui = '/qr' },
  @{ n = 'qr-machine-switch';   pkg = 'perMachineOrUser-1.0.0'; args = @('PROBE_SWITCH=1', 'PROBE_AU=2', 'PROBE_PU=1'); ui = '/qr' },
  @{ n = 'upgrade-same';        pkg = 'perUserOrMachine-1.0.0'; args = @(); then = @{ pkg = 'perUserOrMachine-2.0.0'; args = @() } },
  @{ n = 'upgrade-cross';       pkg = 'perUserOrMachine-1.0.0'; args = @(); then = @{ pkg = 'perUserOrMachine-2.0.0'; args = @('MSIINSTALLPERUSER=""') } }
)
if ($NonAdmin) {
  $variants = $variants | Where-Object { $_.n -in 'user-default', 'user-to-machine', 'machine-default', 'machine-to-user' }
}

$id = [Security.Principal.WindowsIdentity]::GetCurrent()
$admin = (New-Object Security.Principal.WindowsPrincipal $id).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
$report = New-Object System.Collections.Generic.List[string]
$report.Add("# INST-02 run $(Get-Date -Format s) on $env:COMPUTERNAME as $($id.Name), elevated admin=$admin")
$report.Add("OS $([Environment]::OSVersion.VersionString); msi.dll $((Get-Item "$env:windir\System32\msi.dll").VersionInfo.FileVersion)")
foreach ($v in $variants) {
  if ($Only -and $v.n -notin $Only) { continue }
  $tag = if ($NonAdmin) { "$($v.n)-nonadmin" } else { $v.n }
  $report.Add('')
  $report.Add("## $tag")
  $msi = "$work\$($v.pkg).msi"
  $code = ProductCode $msi
  $ui = if ($v.ui) { $v.ui } else { '/qn' }
  $log = "$work\$tag-install.log"
  $rc = Msiexec $log (@('/i', $msi, $ui) + $v.args)
  $report.Add("install $($v.pkg) $ui $($v.args -join ' '): exit $rc")
  Probe $report $log
  State $report $code
  $codes = @($code)
  if ($v.then) {
    $msi2 = "$work\$($v.then.pkg).msi"
    $code2 = ProductCode $msi2
    $log2 = "$work\$tag-upgrade.log"
    $rc2 = Msiexec $log2 (@('/i', $msi2, '/qn') + $v.then.args)
    $report.Add("then install $($v.then.pkg) /qn $($v.then.args -join ' '): exit $rc2")
    Probe $report $log2
    $report.Add('  old product:'); State $report $code
    $report.Add('  new product:'); State $report $code2
    $codes += $code2
  }
  foreach ($c in $codes) {
    $rcx = Msiexec "$work\$tag-uninstall-$c.log" @('/x', $c, '/qn')
    $report.Add("uninstall $c : exit $rcx")
  }
  State $report $code
}
$file = Join-Path $out 'report.md'
$report | Set-Content -Encoding utf8 $file
$report
