# CAB-01 / uncompressed-longonly-dir
built pkg.msi sha256 1ef34a2600218a20be77d3767cf2321e11f6c1253b3d4b215ccbdc482e145e64
msiexec /i results\CAB-01\uncompressed-longonly-dir\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Program Files\MsiSpecTest\data.txt
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a386.msi
  log: Product: MsiSpec Test cab01uncompressed-longonly-dir -- Installation completed successfully.
  log: MainEngineThread is returning 0
  installed file C:\Program Files\MsiSpecTest\data.txt: ok sha 1930767a5b82ad39c76fef8be756e031380abd7f0ffc741226866a1d2b575386
msiexec /x results\CAB-01\uncompressed-longonly-dir\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab01uncompressed-longonly-dir -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT uncompressed-longonly-dir install ok=true
