# RUN-18 / vbs-return-failure-3
built pkg.msi sha256 0022eeb2216955b0264f8a6e62d4ca4be01eea9cec6b3d66b32c1fd7aeae43c0
msiexec /i results\RUN-18\vbs-return-failure-3\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a6f9.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT vbs-return-failure-3: exit 0, file installed=true, registered=true, CA: Action ended 18:13:27: TestCA. Return value 0., last action: Action start 18:13:27: InstallFinalize.
msiexec /x {AB557882-671D-481F-B079-F5EC86532273} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {AB557882-671D-481F-B079-F5EC86532273} -> 0
