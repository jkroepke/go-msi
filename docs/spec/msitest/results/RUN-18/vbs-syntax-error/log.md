# RUN-18 / vbs-syntax-error
built pkg.msi sha256 937ba7888ba3d1fa7584f1cf9327b5bb2b5ba8ccf17ec08c44d3f12c55a61dd3
msiexec /i results\RUN-18\vbs-syntax-error\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1720. There is a problem with this Windows Installer package. A script required for this install to complete could not be run. Contact your support personnel or pac
  log: Return value 3.
  log: Product: MsiSpec Test run18 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT vbs-syntax-error: exit 1603, file installed=false, registered=false, CA: Action ended 18:13:32: TestCA. Return value 3., last action: Action start 18:13:32: TestCA.
