# RUN-18 / js-return-failure-3
built pkg.msi sha256 e3703151f82001431f082287e5138ea388d028ed60115c6d039e7c8f07c8c34b
msiexec /i results\RUN-18\js-return-failure-3\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a707.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT js-return-failure-3: exit 0, file installed=true, registered=true, CA: Action ended 18:13:30: TestCA. Return value 0., last action: Action start 18:13:30: InstallFinalize.
msiexec /x {4D38DCB3-E472-438D-888F-556F97AFBEF7} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {4D38DCB3-E472-438D-888F-556F97AFBEF7} -> 0
