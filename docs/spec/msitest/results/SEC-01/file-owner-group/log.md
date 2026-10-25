# SEC-01 / file-owner-group
File row secured with O:BAG:BAD:PAI(A;;FA;;;SY)(A;;FA;;;BA)
built pkg.msi sha256 444dda9a3e645986df1a8af065db55969aabb4d7e2d9d2e4fd37bc5c305eb26b
msiexec /i results\SEC-01\file-owner-group\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43ecb.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: O:BAG:BAD:PAI(A;;FA;;;SY)(A;;FA;;;BA)
observed SDDL: O:BAG:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)
msiexec /x results\SEC-01\file-owner-group\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT file-owner-group: install 0, uninstall 0, perm.txt left: false
