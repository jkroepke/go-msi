# RUN-18 / vbs-return-finished-5-immediate
built pkg.msi sha256 d0100e340fdda96c858a75668f9806f280ce7e8ee378643c6838a0665295177d
msiexec /i results\RUN-18\vbs-return-finished-5-immediate\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a700.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT vbs-return-finished-5-immediate: exit 0, file installed=true, registered=true, CA: Action ended 18:13:29: TestCA. Return value 0., last action: Action start 18:13:29: InstallFinalize.
msiexec /x {1DB4168F-81EF-4E2B-868C-A1765DBEE1FC} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {1DB4168F-81EF-4E2B-868C-A1765DBEE1FC} -> 0
