# FILE-01 / empty-version-rep6601
built v1.msi sha256 aeab8904e872501ba12615ce52baed5bd83f92b8fb5690c77ccff76731fa14eb
built v2.msi sha256 b0ec149c243fc611313e2112b75d3159419ba32ff887c0bbe419828703962dbf
msiexec /i results\FILE-01\empty-version-rep6601\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a65f.msi
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\FILE-01\empty-version-rep6601\v2.msi -> exit 0
  log: Disallowing installation of component: {135D4D3C-5F51-3062-927C-5C69A4A3FC91} since the same component with higher versioned keyfile exists
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a663.msi
  log: Product: MsiSpec Test run02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT empty-version-rep6601: upgrade exit 0, installed app.exe = v1 bytes, v1 registered=false
  log: Disallowing installation of component: {135D4D3C-5F51-3062-927C-5C69A4A3FC91} since the same component with higher versioned keyfile exists
msiexec /fa results\FILE-01\empty-version-rep6601\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2: C:\Program Files\MsiSpecTest\app.exe
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\$PatchCache$\Managed\BADEE5A3F00E6004290C4BD384BDB428\CacheSize.txt
  log: Product: MsiSpec Test run02 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
  repair /fa after deleting file: exit 0, file restored=true
msiexec /x {3A5EEDAB-E00F-4006-92C0-B43D48DB4B82} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run02 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {3A5EEDAB-E00F-4006-92C0-B43D48DB4B82} -> 0
