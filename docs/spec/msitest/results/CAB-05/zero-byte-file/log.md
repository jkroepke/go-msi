# CAB-05 / zero-byte-file
built makecab-mszip.msi sha256 78745c6533b9e07e04627393f330deb31d0647dde7d822378e45ff3cb48f22c5
msiexec /i results\CAB-05\zero-byte-file\makecab-mszip.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7b8.msi
  log: Product: MsiSpec Test cab05 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT makecab-mszip embedded: install 0, empty.txt present=true size=0
msiexec /x results\CAB-05\zero-byte-file\makecab-mszip.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab05 -- Removal completed successfully.
  log: MainEngineThread is returning 0
built go-mszip.msi sha256 4c04eac65dec3f4aee399bdb16fab8fd7ab84acb9237a6559c9697eacded14ca
msiexec /i results\CAB-05\zero-byte-file\go-mszip.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7c0.msi
  log: Product: MsiSpec Test cab05 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT go-mszip embedded: install 0, empty.txt present=true size=0
msiexec /x results\CAB-05\zero-byte-file\go-mszip.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab05 -- Removal completed successfully.
  log: MainEngineThread is returning 0
built go-stored.msi sha256 2c0830e36b009f29ae79fa9194ebf1b139c1eac962d20908554bc6f7f09df524
msiexec /i results\CAB-05\zero-byte-file\go-stored.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7c8.msi
  log: Product: MsiSpec Test cab05 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT go-stored embedded: install 0, empty.txt present=true size=0
msiexec /x results\CAB-05\zero-byte-file\go-stored.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab05 -- Removal completed successfully.
  log: MainEngineThread is returning 0
built external.msi sha256 880db600fd5f906f447ccabde390ec4372e0368195e0ce5e6a9fef51452c2c4e
switch to external cab: <nil> <nil> commit <nil>
msiexec /i results\CAB-05\zero-byte-file\external.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a7d0.msi
  log: Product: MsiSpec Test cab05x -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT external cab: install 0, empty.txt present=true
msiexec /x results\CAB-05\zero-byte-file\external.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cab05x -- Removal completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\CAB-05\zero-byte-file\external.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1311. Source file not found(cabinet): <scratch>
  log: Return value 3.
  log: Product: MsiSpec Test cab05x -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT external cab missing: install 1603
