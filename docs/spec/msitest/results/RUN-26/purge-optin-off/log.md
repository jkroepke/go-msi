# RUN-26 / purge-optin-off
built v1.msi sha256 785e1deb57a7862f1db5b7d00bcc092820ead95a7509aaba17d43ccc33fe280e
built v2.msi sha256 c5e3315dc5f98bb9651a196a7611dee3a507a70985102e0a5d9097dd48ca592f
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-26\purge-optin-off\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1adb.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0; data after fill: <folder>,app.log,state.db,sub,sub\wal.bin
msiexec /x {68F0BD6F-A2FC-4083-B2D9-A42CE76A16CD} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT purge-optin-off: uninstall [] exit 0, data=<folder>,app.log,state.db,sub,sub\wal.bin
