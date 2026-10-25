# RUN-03 / fourth-field-only
built first.msi sha256 cbd210cd902fa4ccf4b5d19f88016dea71414ab0a427df065f8ed9018d60aaf1
built second.msi sha256 82df9315dcfdbc91876d7ae4a7d3a36dc39ea2dccc703a67238c9fca35a1cb61
msiexec /i results\RUN-03\fourth-field-only\first.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5ab.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-03\fourth-field-only\second.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5b0.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
first 1.0.0.1 exit 0; second 1.0.0.2 exit 0
  second install log: UPGRADEFOUND = ""
  second install log: NEWERFOUND = ""
RESULT fourth-field-only: first still registered=true, second registered=true
msiexec /x {B3579717-BC82-4381-B43E-597271722090} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {B3579717-BC82-4381-B43E-597271722090} -> 0
msiexec /x {46CA0EC3-09CC-4726-853E-700BF48731DC} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {46CA0EC3-09CC-4726-853E-700BF48731DC} -> 0
