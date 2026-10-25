# CAB-01 / go-mszip
built pkg.msi sha256 ec6cc8283f5b7c72b77a13c86a8b18257dec28761535e5b15bdb7b662c02037b
independent reader: 1 files err=<nil>
  f_data 21200 bytes equal=true date 0x0021 time 0x0000 attr 0x20
native expand.exe: Expanding Files Complete ...
  FDI extracted equal=true err=<nil>
msiexec /i results\CAB-01\go-mszip\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3a2.msi
  log: Product: MsiSpec Test cab01go-mszip -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecTest\data.txt: ok sha 1930767a5b82ad39c76fef8be756e031380abd7f0ffc741226866a1d2b575386
msiexec /x results\CAB-01\go-mszip\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab01go-mszip -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT go-mszip install ok=true
