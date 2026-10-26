# FILE-01 / empty-version-rep1401
built v1.msi sha256 f71c22dad827282297943c4db94d03170df0fe376ca58ae95f3ca3313a7012ee
built v2.msi sha256 a7e8d7418c8ff417cf9f60cc2cb6054ae0bb7483fefa130bf69902d2799e65dc
msiexec /i results\FILE-01\empty-version-rep1401\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a670.msi
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\FILE-01\empty-version-rep1401\v2.msi -> exit 0
  log: Disallowing installation of component: {135D4D3C-5F51-3062-927C-5C69A4A3FC91} since the same component with higher versioned keyfile exists
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a677.msi
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT empty-version-rep1401: upgrade exit 0, installed app.exe = missing, v1 registered=false
  log: Disallowing installation of component: {135D4D3C-5F51-3062-927C-5C69A4A3FC91} since the same component with higher versioned keyfile exists
msiexec /fa results\FILE-01\empty-version-rep1401\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test run02 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
  repair /fa after deleting file: exit 0, file restored=true
msiexec /x {2603B311-2EB5-4BFC-B3CF-F51B1EABDCFD} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run02 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {2603B311-2EB5-4BFC-B3CF-F51B1EABDCFD} -> 0
