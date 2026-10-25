# RUN-17 / rep-5802
built v1.msi sha256 c7b44da2c053323fd37410aa08cb10b396f2f30730a5cef789da745486c186d6
built v2.msi sha256 f3461c9db5270de2879c863b118d13a732a915f8d54f75d8b832f11d4ae46439
msiexec /i results\RUN-17\rep-5802\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a601.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-17\rep-5802\v2.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2613
  log: Error 2613:  RemoveExistingProducts action sequenced incorrectly.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
RESULT REP at 5802: v2 exit 1603, v1 registered=true, v2 registered=false, file present=true
msiexec /x {1F2AA44F-4991-4036-8FEF-2A8A48F7707C} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {1F2AA44F-4991-4036-8FEF-2A8A48F7707C} -> 0
