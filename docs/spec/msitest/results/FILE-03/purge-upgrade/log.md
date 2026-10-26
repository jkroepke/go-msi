# FILE-03 / purge-upgrade
built v1.msi sha256 1a10da74e225436fe5675e73ac78eee666a43985fd78659077f8a4e516779f3b
built v2.msi sha256 b50add38c642191626e650abc8181b529ef64b779e3d25837066e02ee40bd128
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\FILE-03\purge-upgrade\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1acc.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0; data after fill: <folder>,app.log,state.db,sub,sub\wal.bin
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\FILE-03\purge-upgrade\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1ad3.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT purge-upgrade: upgrade exit 0, v1 registered=false, data=<folder>,app.log,state.db,sub,sub\wal.bin
msiexec /x {D6AB7B32-5EE5-44A3-B1E5-3A466C297467} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT purge-upgrade: later uninstall exit 0, data=<gone>
