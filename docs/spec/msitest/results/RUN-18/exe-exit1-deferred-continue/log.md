# RUN-18 / exe-exit1-deferred-continue
built pkg.msi sha256 a9f97085e0e6c5c741161e39bae2eed6d358d55bf423ef15b839962f6ef8c95b
msiexec /i results\RUN-18\exe-exit1-deferred-continue\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a6e4.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT exe-exit1-deferred-continue: exit 0, file installed=true, registered=true, CA: Action ended 18:13:24: TestCA. Return value 1. [scheduled], last action: Action start 18:13:24: InstallFinalize.
msiexec /x {6DF6705B-FCEB-470C-9929-47BB9FA572B5} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {6DF6705B-FCEB-470C-9929-47BB9FA572B5} -> 0
