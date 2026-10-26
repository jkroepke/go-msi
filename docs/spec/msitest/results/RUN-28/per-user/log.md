# RUN-28 / per-user
built pkg.msi sha256 735bfbea15f40863799ba4aca2309d3554a7fa5daf7e3c841c1cd7f94a74c69e
msiexec /i C:\Users\Administrator\GolandProjects\go-msi\.claude\worktrees\confident-antonelli-7dec4e\docs\spec\msitest\results\RUN-28\per-user\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\33c6905.msi
  log: Product: MsiSpec Test sc -- Installation completed successfully.
  log: MainEngineThread is returning 0
install exit 0
  start menu C:\Users\Administrator\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\MsiSpecTest\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args=/user wkdir=C:\Program Files\MsiSpecTest\ desc=MsiSpec shortcut test icon=C:\Users\Administrator\AppData\Roaming\Microsoft\Installer\{A1408B5E-6E97-4A7E-BB47-43F22E7D48AA}\app.ico,0 show=7
  desktop C:\Users\Administrator\Desktop\MsiSpec App.lnk: target=C:\Program Files\MsiSpecTest\app.exe args= wkdir= desc= icon=C:\Users\Administrator\AppData\Roaming\Microsoft\Installer\{A1408B5E-6E97-4A7E-BB47-43F22E7D48AA}\app.ico,0 show=1
  common start menu copy present=false
  icon file C:\Users\Administrator\AppData\Roaming\Microsoft\Installer\{A1408B5E-6E97-4A7E-BB47-43F22E7D48AA}\app.ico present=true
  HKCU key path value: 0x1
msiexec /x {A1408B5E-6E97-4A7E-BB47-43F22E7D48AA} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Config.Msi\33c6908.rbf
  log: Note: 1: 2318 2: C:\Config.Msi\33c6909.rbf
  log: Product: MsiSpec Test sc -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall exit 0; start menu lnk left=false, desktop lnk left=false, start menu folder left=false
  icon file left=false
  HKCU key path value after uninstall: <absent>
RESULT per-user: installed=true removed=true
