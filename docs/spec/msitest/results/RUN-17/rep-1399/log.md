# RUN-17 / rep-1399
built v1.msi sha256 22213fd4bc6ea56bffabd394a348cffa256621017cd49aa456f36e4db8c0ffe7
built v2.msi sha256 1791c26d83e4bd8fc9a3580c45f11072af578d5b213290e62679bc5600b876e6
msiexec /i results\RUN-17\rep-1399\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a60a.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-17\rep-1399\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a611.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT REP at 1399: v2 exit 0, v1 registered=false, v2 registered=true, file present=true
msiexec /x {E589E428-9757-4B59-9769-1A6604ED01C4} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {E589E428-9757-4B59-9769-1A6604ED01C4} -> 0
