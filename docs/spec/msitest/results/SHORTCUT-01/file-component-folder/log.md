# SHORTCUT-01 / file-component-folder
built pkg.msi sha256 87f7202e8fe6331b47fbe003bc960cbe3b20405690fb40fa95274beae3dafbf1
msiexec /i C:\Users\Administrator\GolandProjects\go-msi\.claude\worktrees\confident-antonelli-7dec4e\docs\spec\msitest\results\SHORTCUT-01\file-component-folder\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\33c68f3.msi
  log: Product: MsiSpec Test sc -- Installation completed successfully.
  log: MainEngineThread is returning 0
install exit 0
  start menu C:\ProgramData\Microsoft\Windows\Start Menu\Programs\MsiSpecTest\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args=/user wkdir=C:\Program Files\MsiSpecTest\ desc=MsiSpec shortcut test icon=,0 show=7
  desktop C:\Users\Public\Desktop\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args= wkdir= desc= icon=,0 show=1
  per-user start menu copy present=false
msiexec /x {DD545D21-1B42-48EF-BA18-7FF8F449B487} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Config.Msi\33c68f6.rbf
  log: Note: 1: 2318 2: C:\Config.Msi\33c68f7.rbf
  log: Product: MsiSpec Test sc -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall exit 0; start menu lnk left=false, desktop lnk left=false, start menu folder left=false
RESULT file-component-folder: installed=true removed=true
