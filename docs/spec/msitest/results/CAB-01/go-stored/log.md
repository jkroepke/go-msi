# CAB-01 / go-stored
built pkg.msi sha256 3bb1de1c3919993572b65ca385f7253d16b0dca293a4b215b8a4b1bf207e3e4f
independent reader: 1 files err=<nil>
  f_data 21200 bytes equal=true date 0x0021 time 0x0000 attr 0x20
native expand.exe: Expanding Files Complete ...
  FDI extracted equal=true err=<nil>
msiexec /i results\CAB-01\go-stored\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a39b.msi
  log: Product: MsiSpec Test cab01go-stored -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecTest\data.txt: ok sha 1930767a5b82ad39c76fef8be756e031380abd7f0ffc741226866a1d2b575386
msiexec /x results\CAB-01\go-stored\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab01go-stored -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT go-stored install ok=true
