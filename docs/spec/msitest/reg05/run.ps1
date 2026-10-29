# REG-05: does anything tell the shell that file associations changed?
#
# Starts a listener window registered for SHCNE_ASSOCCHANGED in this session,
# checks it with an SHChangeNotify call from this process, then installs and
# uninstalls three packages and records which steps produced a notification:
#   probe    msi-builder fileTypes (BuilderAssocChanged action)
#   baseline msi-builder plain HKCR registry entries, no action
#   native   ProgId/Extension/Verb tables (WiX 5, black box), no action
#
#   powershell -ExecutionPolicy Bypass -File run.ps1
#
# Run it elevated, in an interactive session. Needs Go and the wix CLI on
# PATH. The report goes to ..\results\REG-05\report.md (not committed; log.md
# beside it is the committed summary), the msiexec logs to %TEMP%\reg05.
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$repo = Resolve-Path (Join-Path $here '..\..\..\..')
$work = Join-Path $env:TEMP 'reg05'
Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $work | Out-Null
$out = Join-Path $here '..\results\REG-05'
New-Item -ItemType Directory -Force $out | Out-Null

Push-Location $repo
go build -o "$work\msi-builder.exe" ./cmd/msi-builder
if ($LASTEXITCODE) { throw 'go build failed' }
Pop-Location
Copy-Item "$env:windir\System32\whoami.exe" "$work\hello.exe" -Force
foreach ($f in 'probe.yaml', 'baseline.yaml', 'native.wxs', 'listener.cs') { Copy-Item "$here\$f" "$work\$f" -Force }
foreach ($p in 'probe', 'baseline') {
  & "$work\msi-builder.exe" build -q -o "$work\$p.msi" "$work\$p.yaml"
  if ($LASTEXITCODE) { throw "build $p failed" }
}
Push-Location $work
wix build -arch x64 -o native.msi native.wxs
if ($LASTEXITCODE) { throw 'wix build failed' }
Pop-Location

$events = Join-Path $work 'events.log'
$stop = Join-Path $work 'stop'
$listener = @"
Add-Type -ReferencedAssemblies System.Windows.Forms -Path '$work\listener.cs'
[AssocListener]::Run('$events', '$stop')
"@
Set-Content -Encoding utf8 "$work\listener.ps1" $listener
$proc = Start-Process powershell.exe -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "$work\listener.ps1" -PassThru -WindowStyle Hidden
for ($i = 0; $i -lt 100 -and -not (Test-Path $events); $i++) { Start-Sleep -Milliseconds 200 }
if (-not (Test-Path $events)) { throw 'listener did not start' }
try {
  function Mark([string]$text) {
    for ($i = 0; $i -lt 20; $i++) {
      try { [IO.File]::AppendAllText($events, "$(Get-Date -Format 'HH:mm:ss.fff') --- $text`r`n"); return }
      catch { Start-Sleep -Milliseconds 50 }
    }
  }

  function Msiexec([string]$log, [string[]]$msiArgs) {
    $p = Start-Process msiexec.exe -ArgumentList ($msiArgs + @('/qn', '/l*vx', $log)) -PassThru
    if (-not $p.WaitForExit(240000)) { $p.Kill(); return -1 }
    $p.ExitCode
  }

  Add-Type -Namespace Reg05 -Name Shell -MemberDefinition '[DllImport("shell32.dll")] public static extern void SHChangeNotify(int e, uint f, IntPtr a, IntPtr b);'
  Mark 'control: SHChangeNotify(SHCNE_ASSOCCHANGED) from run.ps1'
  [Reg05.Shell]::SHChangeNotify(0x08000000, 0x1000, [IntPtr]::Zero, [IntPtr]::Zero)
  Start-Sleep -Seconds 3

  $report = New-Object System.Collections.Generic.List[string]
  $id = [Security.Principal.WindowsIdentity]::GetCurrent()
  $report.Add("# REG-05 run $(Get-Date -Format s) on $env:COMPUTERNAME as $($id.Name), session $((Get-Process -Id $PID).SessionId)")
  $report.Add("OS $([Environment]::OSVersion.VersionString); msi.dll $((Get-Item "$env:windir\System32\msi.dll").VersionInfo.FileVersion)")
  $keys = @{ probe = '.reg05p'; baseline = '.reg05b'; native = '.reg05n' }
  foreach ($p in 'probe', 'baseline', 'native') {
    $report.Add("$p before: HKCR\$($keys[$p]) exists=$(Test-Path "Registry::HKEY_CLASSES_ROOT\$($keys[$p])")")
    foreach ($step in 'install', 'uninstall') {
      $log = "$work\$p-$step.log"
      Mark "$p $step start"
      $rc = if ($step -eq 'install') { Msiexec $log @('/i', "$work\$p.msi") } else { Msiexec $log @('/x', "$work\$p.msi") }
      Start-Sleep -Seconds 3
      Mark "$p $step end (exit $rc)"
      $ext = Test-Path "Registry::HKEY_CLASSES_ROOT\$($keys[$p])"
      $report.Add("$p $step : exit $rc; HKCR\$($keys[$p]) exists=$ext")
      $text = Get-Content $log -Raw -ErrorAction SilentlyContinue
      if (-not $text) { $report.Add('  no log'); continue }
      foreach ($m in [regex]::Matches($text, '[^\r\n]*(SHChangeNotify|Doing action: (RegisterExtensionInfo|RegisterProgIdInfo|UnregisterExtensionInfo|UnregisterProgIdInfo|BuilderAssocChanged\w*))[^\r\n]*')) {
        $report.Add("  log: $($m.Value.Trim())")
      }
    }
  }
  # The probe keeps .reg05p on purpose (permanent component). Remove it and
  # the permanent component registration, so the next run starts clean.
  Remove-Item 'Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Classes\.reg05p' -Recurse -Force -ErrorAction SilentlyContinue
  $comps = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-18\Components'
  # Its key path is the .reg05p default, held by the null product GUID.
  Get-ChildItem $comps | Where-Object {
    ($_.GetValue('00000000000000000000000000000000') -as [string]) -like '*\.reg05p*'
  } | ForEach-Object {
    Remove-Item -Recurse -Force -ErrorAction Continue $_.PSPath
    $report.Add("cleanup: permanent component $($_.PSChildName) removed=$(-not (Test-Path $_.PSPath))")
  }
} finally {
  # Always stop the listener, also when a step above fails.
  New-Item -ItemType File -Force $stop | Out-Null
  $proc.WaitForExit(10000) | Out-Null
}
$report.Add('')
$report.Add('## listener')
foreach ($l in Get-Content $events) { $report.Add("    $l") }
$file = Join-Path $out 'report.md'
$report | Set-Content -Encoding utf8 $file
$report
