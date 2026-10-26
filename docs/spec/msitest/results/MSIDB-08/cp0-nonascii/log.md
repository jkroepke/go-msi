# MSIDB-08 / cp0-nonascii
build with database code page 0: <nil>
raw pool code page 0; TESTTEXT stored bytes 43 61 66 e9 (found id 0)
native read back "CafÃ©" equal=false err=<nil>
msiexec /i results\MSIDB-08\cp0-nonascii\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a751.msi
  log: Product: MsiSpec Test db08cp0-nonascii -- Installation completed successfully.
  log: MainEngineThread is returning 0
installed files in C:\Program Files\MsiSpeccp0-nonascii: ["RÃ©sumÃ©.txt"]
msiexec /x results\MSIDB-08\cp0-nonascii\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db08cp0-nonascii -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT cp0-nonascii: install 0
