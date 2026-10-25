# CAB-01 / uncompressed
built pkg.msi sha256 747f778fa48d3e4947a9551aff091d18bd1b5e363e9ef7c73a31649e6484d841
msiexec /i results\CAB-01\uncompressed\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Program Files\MsiSpecTest\data.txt
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a378.msi
  log: Product: MsiSpec Test cab01uncompressed -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecTest\data.txt: ok sha 1930767a5b82ad39c76fef8be756e031380abd7f0ffc741226866a1d2b575386
msiexec /x results\CAB-01\uncompressed\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab01uncompressed -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT uncompressed install ok=true
