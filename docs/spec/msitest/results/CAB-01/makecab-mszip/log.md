# CAB-01 / makecab-mszip
built pkg.msi sha256 f50482c7c78b57c3524b557f0b340255a292eba5f11e9a1ae7a269cacb8b33c4
independent reader: 1 files err=<nil>
  f_data 21200 bytes equal=true date 0x5d59 time 0x90fb attr 0x20
native expand.exe: Expanding Files Complete ...
  FDI extracted equal=true err=<nil>
msiexec /i results\CAB-01\makecab-mszip\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a394.msi
  log: Product: MsiSpec Test cab01makecab-mszip -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecTest\data.txt: ok sha 1930767a5b82ad39c76fef8be756e031380abd7f0ffc741226866a1d2b575386
msiexec /x results\CAB-01\makecab-mszip\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab01makecab-mszip -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT makecab-mszip install ok=true
