# CAB-01 / makecab-none
built pkg.msi sha256 af2bff8d01cb37cae2513cf766c07527842fe7d7a6f8f097805644a098557eca
independent reader: 1 files err=<nil>
  f_data 21200 bytes equal=true date 0x5d59 time 0x90fb attr 0x20
native expand.exe: Expanding Files Complete ...
  FDI extracted equal=true err=<nil>
msiexec /i results\CAB-01\makecab-none\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a38d.msi
  log: Product: MsiSpec Test cab01makecab-none -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecTest\data.txt: ok sha 1930767a5b82ad39c76fef8be756e031380abd7f0ffc741226866a1d2b575386
msiexec /x results\CAB-01\makecab-none\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab01makecab-none -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT makecab-none install ok=true
