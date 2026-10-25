# DB-08 / cp1252
build with database code page 1252: <nil>
raw pool code page 1252; TESTTEXT stored bytes 43 61 66 e9 20 80 (found id 94)
native read back "Café €" equal=true err=<nil>
msiexec /i results\DB-08\cp1252\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a73c.msi
  log: Product: MsiSpec Test db08cp1252 -- Installation completed successfully.
  log: MainEngineThread is returning 0
installed files in C:\Program Files\MsiSpeccp1252: ["Résumé.txt"]
msiexec /x results\DB-08\cp1252\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db08cp1252 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT cp1252: install 0
