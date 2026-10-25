# RUN-18 / exe-exit1-immediate-continue
built pkg.msi sha256 49a94f3ef739c1ec6faa3a6c847074fae3cead8ceacdbdaa59462798c4b6a7d7
msiexec /i results\RUN-18\exe-exit1-immediate-continue\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a6da.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT exe-exit1-immediate-continue: exit 0, file installed=true, registered=true, CA: Action ended 18:13:22: TestCA. Return value 1., last action: Action start 18:13:22: InstallFinalize.
msiexec /x {72146744-9A90-40C4-9FD2-10DB849824E5} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {72146744-9A90-40C4-9FD2-10DB849824E5} -> 0
