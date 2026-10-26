# MSIDB-10 / validation-table
built missing.msi sha256 8f4b74885dc498168765ee68cbe5032816ab9e21e00fa08d6747693437427d13
msiexec /i results\MSIDB-10\validation-table\missing.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3b7.msi
  log: Product: MsiSpec Test db10 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\MSIDB-10\validation-table\missing.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db10 -- Removal completed successfully.
  log: MainEngineThread is returning 0
missing: install 0, uninstall 0
variant empty.msi sha256 8f4b74885dc498168765ee68cbe5032816ab9e21e00fa08d6747693437427d13
create empty _Validation: <nil>, commit <nil>
msiexec /i results\MSIDB-10\validation-table\empty.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3be.msi
  log: Product: MsiSpec Test db10 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\MSIDB-10\validation-table\empty.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test db10 -- Removal completed successfully.
  log: MainEngineThread is returning 0
empty: install 0, uninstall 0
RESULT missing _Validation installs (see missing-install.log); empty _Validation -> 0
