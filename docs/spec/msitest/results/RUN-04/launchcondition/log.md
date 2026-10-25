# RUN-04 / launchcondition
built newer.msi sha256 e06b759274579581df9669bf2b020511ab921ee4af701437c2167fb8bfe3bb87
built older.msi sha256 da789cc76e736719079ed9694ed3b4c141ffb7989dcd76b48918dde0c49e3322
msiexec /i results\RUN-04\launchcondition\newer.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5b9.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-04\launchcondition\older.msi -> exit 1603
  log: Return value 3.
  log: Product: MsiSpec Test upg -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT launchcondition: downgrade exit 1603; newer still registered=true; older registered=false
msiexec /x {32FDBB15-EA4F-4E58-AC31-19BADEED3FB4} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {32FDBB15-EA4F-4E58-AC31-19BADEED3FB4} -> 0
