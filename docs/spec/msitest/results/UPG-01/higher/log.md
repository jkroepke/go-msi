# UPG-01 / higher
built first.msi sha256 4810112fbcea37494f7ab67123c9306c3562b061f609d7118c32b1b2ebf1c418
built second.msi sha256 9604d84e3262ba34c77b90e1c1591441f1a1542ee243aab77b6b74f5cecb0f52
msiexec /i results\UPG-01\higher\first.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a581.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\UPG-01\higher\second.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a588.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
first 1.0.0 exit 0; second 2.0.0 exit 0
  second install log: UPGRADEFOUND = "{A5FDAF13-FD6E-4EE7-A4B1-E15E246D861B}"
  second install log: NEWERFOUND = ""
RESULT higher: first still registered=false, second registered=true
msiexec /x {B4D9D6BD-2E49-464B-9CF2-0AAC6824AC29} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {B4D9D6BD-2E49-464B-9CF2-0AAC6824AC29} -> 0
