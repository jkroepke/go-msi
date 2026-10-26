# SHORTCUT-01 / file-component
built pkg.msi sha256 eb055daaf6281d0bc96b295d43898071cd51f3b5a2c6334941226e3c12156684
msiexec /i C:\Users\Administrator\GolandProjects\go-msi\.claude\worktrees\confident-antonelli-7dec4e\docs\spec\msitest\results\SHORTCUT-01\file-component\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\33c68ea.msi
  log: Product: MsiSpec Test sc -- Installation completed successfully.
  log: MainEngineThread is returning 0
install exit 0
  start menu C:\ProgramData\Microsoft\Windows\Start Menu\Programs\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args=/user wkdir=C:\Program Files\MsiSpecTest\ desc=MsiSpec shortcut test icon=,0 show=7
  desktop C:\Users\Public\Desktop\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args= wkdir= desc= icon=,0 show=1
  per-user start menu copy present=false
msiexec /x {2A338637-73AF-48B2-ABA9-B0DFDECEB1EB} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Config.Msi\33c68ed.rbf
  log: Note: 1: 2318 2: C:\Config.Msi\33c68ee.rbf
  log: Product: MsiSpec Test sc -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall exit 0; start menu lnk left=false, desktop lnk left=false, start menu folder left=false
RESULT file-component: installed=true removed=true
