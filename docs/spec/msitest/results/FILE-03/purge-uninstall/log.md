# FILE-03 / purge-uninstall
built v1.msi sha256 6c3f0dc8f9136a055a9750c6a27fbe976fd271ed797231a1bafb296b6d374a81
built v2.msi sha256 af3818218dcc2e3e4eca734fef2bd1b7b85a4e2489e09ec75b1efc2b10278587
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\FILE-03\purge-uninstall\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1ac4.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0; data after fill: <folder>,app.log,state.db,sub,sub\wal.bin
msiexec /x {4F80583E-D3D8-49E6-B342-EECB4D9C9CB1} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT purge-uninstall: uninstall [] exit 0, data=<gone>
