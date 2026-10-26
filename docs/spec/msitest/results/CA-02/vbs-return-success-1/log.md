# CA-02 / vbs-return-success-1
built pkg.msi sha256 d45ba1b82830e41edbee3b7c1ff426b4bda0ee1e5967d562ef8e54781ba53d3e
msiexec /i results\CA-02\vbs-return-success-1\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a6eb.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT vbs-return-success-1: exit 0, file installed=true, registered=true, CA: Action ended 18:13:25: TestCA. Return value 0., last action: Action start 18:13:25: InstallFinalize.
msiexec /x {4D5D71BB-8EE7-4E33-A639-27B5A26F8470} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {4D5D71BB-8EE7-4E33-A639-27B5A26F8470} -> 0
