# MSIDB-08 / cp1251
build with database code page 1251: <nil>
raw pool code page 1251; TESTTEXT stored bytes cf f0 e8 e2 e5 f2 (found id 94)
native read back "Привет" equal=true err=<nil>
msiexec /i results\MSIDB-08\cp1251\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a74a.msi
  log: Product: MsiSpec Test db08cp1251 -- Installation completed successfully.
  log: MainEngineThread is returning 0
installed files in C:\Program Files\MsiSpeccp1251: ["Пр.txt"]
msiexec /x results\MSIDB-08\cp1251\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db08cp1251 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT cp1251: install 0
