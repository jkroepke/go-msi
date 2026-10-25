# DB-08 / cp65001
build with database code page 65001: <nil>
raw pool code page 65001; TESTTEXT stored bytes 43 61 66 c3 a9 20 e2 82 ac 20 e6 97 a5 e6 9c ac (found id 94)
native read back "Café € 日本" equal=true err=<nil>
msiexec /i results\DB-08\cp65001\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a743.msi
  log: Product: MsiSpec Test db08cp65001 -- Installation completed successfully.
  log: MainEngineThread is returning 0
installed files in C:\Program Files\MsiSpeccp65001: ["日本.txt"]
msiexec /x results\DB-08\cp65001\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db08cp65001 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT cp65001: install 0
