# RUN-18 / vbs-return-finished-5-deferred
built pkg.msi sha256 c83746dcce40627c2d2f206201c987504009173aeb32400259948d3d592a6fcb
msiexec /i results\RUN-18\vbs-return-finished-5-deferred\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a717.msi
  log: Product: MsiSpec Test run18 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT vbs-return-finished-5-deferred: exit 0, file installed=true, registered=true, CA: Action ended 18:13:33: TestCA. Return value 1. [scheduled], last action: Action start 18:13:33: InstallFinalize.
msiexec /x {BB283565-872B-4E8D-B782-1BDDDEBC6175} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run18 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {BB283565-872B-4E8D-B782-1BDDDEBC6175} -> 0
