# RUN-28 / hkcu-folder
built pkg.msi sha256 18fb8aa5dea4a2c59a925dc15013e72bf209ba03b03ec822fb9492ea8ee5100d
msiexec /i C:\Users\Administrator\GolandProjects\go-msi\.claude\worktrees\confident-antonelli-7dec4e\docs\spec\msitest\results\RUN-28\hkcu-folder\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\33c68e1.msi
  log: Product: MsiSpec Test sc -- Installation completed successfully.
  log: MainEngineThread is returning 0
install exit 0
  start menu C:\ProgramData\Microsoft\Windows\Start Menu\Programs\MsiSpecTest\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args=/user wkdir=C:\Program Files\MsiSpecTest\ desc=MsiSpec shortcut test icon=C:\WINDOWS\Installer\{0453A0AA-1165-4777-975D-D7FF1B78CCD8}\app.ico,0 show=7
  desktop C:\Users\Public\Desktop\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args= wkdir= desc= icon=C:\WINDOWS\Installer\{0453A0AA-1165-4777-975D-D7FF1B78CCD8}\app.ico,0 show=1
  per-user start menu copy present=false
  icon file C:\WINDOWS\Installer\{0453A0AA-1165-4777-975D-D7FF1B78CCD8}\app.ico present=true
  HKCU key path value: 0x1
msiexec /x {0453A0AA-1165-4777-975D-D7FF1B78CCD8} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Config.Msi\33c68e4.rbf
  log: Note: 1: 2318 2: C:\Config.Msi\33c68e5.rbf
  log: Product: MsiSpec Test sc -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall exit 0; start menu lnk left=false, desktop lnk left=false, start menu folder left=false
  icon file left=false
  HKCU key path value after uninstall: <absent>
RESULT hkcu-folder: installed=true removed=true
