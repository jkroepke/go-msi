# FILE-03 / removefile-uninstall
built v1.msi sha256 0fefb9e419ef405ddb7a69282d6e378dc9e96e41f75d3f12ba9864d97af0620e
built v2.msi sha256 bb1c9e121da68025b165849955a0555b5dd82b0306bfacd79d390662dc8f733e
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\FILE-03\removefile-uninstall\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1aab.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0; data after fill: <folder>,app.log,state.db,sub,sub\wal.bin
msiexec /x {509D6C34-E114-42E9-A3DA-588A882A3F57} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT removefile-uninstall: uninstall [] exit 0, data=<folder>,sub,sub\wal.bin
