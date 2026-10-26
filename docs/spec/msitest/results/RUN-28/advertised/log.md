# RUN-28 / advertised
built pkg.msi sha256 ff604d2e70368ad91be7ed3fb3cc74051a197160e2168a92cd36aa7958897dd2
msiexec /i C:\Users\Administrator\GolandProjects\go-msi\.claude\worktrees\confident-antonelli-7dec4e\docs\spec\msitest\results\RUN-28\advertised\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\33c68fc.msi
  log: Product: MsiSpec Test sc -- Installation completed successfully.
  log: MainEngineThread is returning 0
install exit 0
  start menu C:\ProgramData\Microsoft\Windows\Start Menu\Programs\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args=/user wkdir=C:\Program Files\MsiSpecTest\ desc=MsiSpec shortcut test icon=C:\WINDOWS\Installer\{DA9E1A1B-CD4E-4244-B427-2EC9C26BE1D0}\app.ico,0 show=7
  desktop C:\Users\Public\Desktop\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args= wkdir= desc= icon=C:\WINDOWS\Installer\{DA9E1A1B-CD4E-4244-B427-2EC9C26BE1D0}\app.ico,0 show=1
  per-user start menu copy present=false
  icon file C:\WINDOWS\Installer\{DA9E1A1B-CD4E-4244-B427-2EC9C26BE1D0}\app.ico present=true
msiexec /x {DA9E1A1B-CD4E-4244-B427-2EC9C26BE1D0} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Config.Msi\33c68ff.rbf
  log: Note: 1: 2318 2: C:\Config.Msi\33c6900.rbf
  log: Product: MsiSpec Test sc -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall exit 0; start menu lnk left=false, desktop lnk left=false, start menu folder left=false
  icon file left=false
RESULT advertised: installed=true removed=true
