# UPG-01 / equal
built first.msi sha256 cada10b0d36d93f71852c4e61975719287a8fd54418a09ccb5dd027d065d0b64
built second.msi sha256 22d61a8a07dcc79e99a16b34282b37a51c2ddf64c5865ea0b487a2583264fc91
msiexec /i results\UPG-01\equal\first.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a58f.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\UPG-01\equal\second.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a594.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
first 1.0.0 exit 0; second 1.0.0 exit 0
  second install log: UPGRADEFOUND = ""
  second install log: NEWERFOUND = ""
RESULT equal: first still registered=true, second registered=true
msiexec /x {8063C2CA-17CB-4A80-B21B-04DB50B8B912} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {8063C2CA-17CB-4A80-B21B-04DB50B8B912} -> 0
msiexec /x {397FF368-F185-4013-9F2E-7B8720C2EA7A} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {397FF368-F185-4013-9F2E-7B8720C2EA7A} -> 0
