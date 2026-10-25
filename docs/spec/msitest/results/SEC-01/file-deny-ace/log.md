# SEC-01 / file-deny-ace
File row secured with D:PAI(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
built pkg.msi sha256 dea111a6dc68074013432e4916ef4ebd5c783f9e6b9a540b2fae9e1202918b68
msiexec /i results\SEC-01\file-deny-ace\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43f14.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
observed SDDL: O:SYG:SYD:P(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
msiexec /x results\SEC-01\file-deny-ace\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  uninstall log: Info 1321.The Installer has insufficient privileges to modify this file: C:\Program Files\MsiSpecPerm\perm.txt. System Error 5.
RESULT file-deny-ace: install 0, uninstall 0, perm.txt left: true
  leftover perm.txt SDDL: O:SYG:SYD:P(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
  cleanup: remove install dir -> <nil>
