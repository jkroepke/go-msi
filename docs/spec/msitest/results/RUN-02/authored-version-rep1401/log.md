# RUN-02 / authored-version-rep1401
built v1.msi sha256 87194e3cbda7ed08ce6db1883452ba904594056f0f0e34e178ea20057763d96f
  set File.Version 1.0.0.0: <nil>, commit <nil>
built v2.msi sha256 fb02a955c197f56c77bd54a2108dfa0e57ed96c0a1fa85baa77bd8207ed9771e
  set File.Version 2.0.0.0: <nil>, commit <nil>
msiexec /i results\RUN-02\authored-version-rep1401\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a63d.msi
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-02\authored-version-rep1401\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a644.msi
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT authored-version-rep1401: upgrade exit 0, installed app.exe = v2 bytes, v1 registered=false
msiexec /fa results\RUN-02\authored-version-rep1401\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test run02 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
  repair /fa after deleting file: exit 0, file restored=true
msiexec /x {4FDD210A-8405-47C5-8649-E06ADD085ED3} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run02 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {4FDD210A-8405-47C5-8649-E06ADD085ED3} -> 0
