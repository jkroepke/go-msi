# SEC-01 / file-condition-false
File row secured with D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;BU)
built pkg.msi sha256 b658550ea069e631fbd1cf04dd83de38b5646d78e211bd571ebd48bcb4d50c48
msiexec /i results\SEC-01\file-condition-false\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43ed2.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;BU)
observed SDDL: O:SYG:SYD:AI(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1200a9;;;BU)(A;ID;0x1200a9;;;AC)(A;ID;0x1200a9;;;S-1-15-2-2)
msiexec /x results\SEC-01\file-condition-false\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT file-condition-false: install 0, uninstall 0, perm.txt left: false
