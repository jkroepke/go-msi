# CA-02 / vbs-binary-return-1
built pkg.msi sha256 54ea1c5d4be41c5c5349c5829e091085076a0f61cfcba444c5924bd693b6f5bc
Binary script return 1: <nil> <nil> commit <nil>
msiexec /i results\CA-02\vbs-binary-return-1\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a71e.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT vbs-binary-return-1: exit 0, file installed=true, registered=true, CA: Action ended 18:13:51: TestCA. Return value 1., last action: Action start 18:13:51: InstallFinalize.
msiexec /x {0E3E13F5-2C28-4532-B786-3199893925A2} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {0E3E13F5-2C28-4532-B786-3199893925A2} -> 0
