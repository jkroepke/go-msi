# UPG-01 / lower
built first.msi sha256 8f51d935a4e383e302ec4fb97d2229f1416896aeb808480a180a0f9725386334
built second.msi sha256 b2685332365084c2705c7a5feef9643eb67d795881d6cc7dd9596b340b5fd0c0
msiexec /i results\UPG-01\lower\first.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a59d.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\UPG-01\lower\second.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5a2.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
first 2.0.0 exit 0; second 1.0.0 exit 0
  second install log: UPGRADEFOUND = ""
  second install log: NEWERFOUND = "{21975F2C-E794-4A51-A72C-0F7D750B60DA}"
RESULT lower: first still registered=true, second registered=true
msiexec /x {FB3377BE-A4ED-432A-997E-CF5F23051F50} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {FB3377BE-A4ED-432A-997E-CF5F23051F50} -> 0
msiexec /x {21975F2C-E794-4A51-A72C-0F7D750B60DA} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {21975F2C-E794-4A51-A72C-0F7D750B60DA} -> 0
