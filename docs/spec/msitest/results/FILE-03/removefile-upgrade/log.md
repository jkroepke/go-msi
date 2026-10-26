# FILE-03 / removefile-upgrade
built v1.msi sha256 1b7eb4527f494207b0fc53391de50920299bd0cf398ae770ae0db3f44a5ddd3b
built v2.msi sha256 f3d4606d9c08e048890cbb43cdb6f6511e162644f5e7101fb007fe522a0f3cc3
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\FILE-03\removefile-upgrade\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1ab4.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0; data after fill: <folder>,app.log,state.db,sub,sub\wal.bin
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\FILE-03\removefile-upgrade\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1abd.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT removefile-upgrade: upgrade exit 0, v1 registered=false, data=<folder>,sub,sub\wal.bin
msiexec /x {0336861D-E20F-4976-996A-94A120F614B0} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT removefile-upgrade: later uninstall exit 0, data=<folder>,sub,sub\wal.bin
