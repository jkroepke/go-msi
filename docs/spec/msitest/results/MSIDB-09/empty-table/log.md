# MSIDB-09 / empty-table
built native.msi sha256 191e88a79e0728d26e479e32a73fd946d791748248b3ac17791f038f9601186b
native: LaunchCondition in _Tables=true, stream present=false
variant empty-physical-stream.msi sha256 8fb14162bee7994d8a8de2807ea4044e852605ba9c71b60f07b1f104bc6266c2
empty physical stream: open ok; ProductCode [[{C2E98FCE-1B4C-44B2-94DC-01ABF03875FB}]] err=<nil>; 9 tables
variant empty-physical-stream-commit.msi sha256 8fb14162bee7994d8a8de2807ea4044e852605ba9c71b60f07b1f104bc6266c2
insert into empty-stream table + commit: <nil> / <nil>
msiexec /i results\MSIDB-09\empty-table\native.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3a9.msi
  log: Product: MsiSpec Test db09 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\MSIDB-09\empty-table\native.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db09 -- Removal completed successfully.
  log: MainEngineThread is returning 0
no-stream: install 0, uninstall 0
msiexec /i results\MSIDB-09\empty-table\empty-physical-stream.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3b0.msi
  log: Product: MsiSpec Test db09 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\MSIDB-09\empty-table\empty-physical-stream.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db09 -- Removal completed successfully.
  log: MainEngineThread is returning 0
empty-stream: install 0, uninstall 0
RESULT empty physical stream -> install 0
