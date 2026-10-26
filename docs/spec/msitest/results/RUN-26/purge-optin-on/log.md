# RUN-26 / purge-optin-on
built v1.msi sha256 ac8b5750aac8b1d34d0910fb3b3abc4d354b3b3f53fcec4d24d28a2f4627c738
built v2.msi sha256 1caac2941053e5e1856ec24a4a54024c1989e1a71384b21754640d3fb0c25924
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-26\purge-optin-on\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1ae2.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0; data after fill: <folder>,app.log,state.db,sub,sub\wal.bin
msiexec /x {5A9B40B3-8411-426F-AB36-5B931E989FD2} DELETE_DATA=1 -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT purge-optin-on: uninstall [DELETE_DATA=1] exit 0, data=<gone>
