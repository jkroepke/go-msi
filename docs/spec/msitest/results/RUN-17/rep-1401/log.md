# RUN-17 / rep-1401
built v1.msi sha256 a008ab1e8d190d1ea2d58c7458dd40f23a3e07fa2d73eee145334655c87f0f12
built v2.msi sha256 c09470711d4e784a8db208b524e4d8d3594135c60873d58b008ef5a622aa55f0
msiexec /i results\RUN-17\rep-1401\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5c9.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-17\rep-1401\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5d0.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT REP at 1401: v2 exit 0, v1 registered=false, v2 registered=true, file present=true
msiexec /x {074EADF1-E820-472A-BBD0-16B94BBCD534} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {074EADF1-E820-472A-BBD0-16B94BBCD534} -> 0
