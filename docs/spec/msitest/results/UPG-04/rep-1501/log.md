# UPG-04 / rep-1501
built v1.msi sha256 0dceb344f14a1691e02f348e22915795b92316e97cf0bb5b8e17bdbffdd5d76e
built v2.msi sha256 5a3c839727a86f8f106ee7a65605b3463c2924acad3c25fbe8c09e705e44e6d8
msiexec /i results\UPG-04\rep-1501\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5d7.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\UPG-04\rep-1501\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5de.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT REP at 1501: v2 exit 0, v1 registered=false, v2 registered=true, file present=true
msiexec /x {F3600E36-2694-4FB7-8DF2-076348119CC7} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {F3600E36-2694-4FB7-8DF2-076348119CC7} -> 0
