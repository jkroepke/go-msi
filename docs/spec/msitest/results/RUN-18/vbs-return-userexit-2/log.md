# RUN-18 / vbs-return-userexit-2
built pkg.msi sha256 3646bb32d2cf8cad4b18263bf1a9c2932ebdc8a8cb1e7822752d1d8c64994614
msiexec /i results\RUN-18\vbs-return-userexit-2\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a6f2.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT vbs-return-userexit-2: exit 0, file installed=true, registered=true, CA: Action ended 18:13:26: TestCA. Return value 0., last action: Action start 18:13:26: InstallFinalize.
msiexec /x {3DFEC2C7-18DA-4606-A385-40CF221A8B35} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {3DFEC2C7-18DA-4606-A385-40CF221A8B35} -> 0
