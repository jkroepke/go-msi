# SEC-01 / file-owner-group
File row secured with O:SYG:SYD:PAI(A;;FA;;;SY)(A;;FA;;;BA)
built pkg.msi sha256 8d648c612c90b0d631897d0d5532c179343ad6ef5e10dbc1af22b9219fdb01f5
msiexec /i results\SEC-01\file-owner-group\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1b3bb77.msi
  log: Product: MsiSpec Test run19 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: O:SYG:SYD:PAI(A;;FA;;;SY)(A;;FA;;;BA)
observed SDDL: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)
msiexec /x results\SEC-01\file-owner-group\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run19 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT file-owner-group: install 0, uninstall 0, perm.txt left: false
