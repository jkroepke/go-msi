# SEC-01 / file-deny-ace
File row secured with D:PAI(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
built pkg.msi sha256 28ef7add1d75b63162c9241a0dae84b7c175390b7cd3ed29e05443eb3695daaa
msiexec /i results\SEC-01\file-deny-ace\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1b3bbaa.msi
  log: Product: MsiSpec Test run19 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
observed SDDL: O:SYG:SYD:P(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
msiexec /x results\SEC-01\file-deny-ace\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run19 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT file-deny-ace: install 0, uninstall 0, perm.txt left: true
  leftover perm.txt SDDL: O:SYG:SYD:P(D;;FA;;;BU)(A;;FA;;;SY)(A;;FA;;;BA)
  cleanup: remove install dir -> <nil>
