# SEC-01 / file
File row secured with D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;BU)
built pkg.msi sha256 e4c9606f8bacbef2428ea2dcc3543114d1ee0d39a096b407732d296e862a05c9
msiexec /i results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1b3bb67.msi
  log: Product: MsiSpec Test run19 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;BU)
observed SDDL: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;BU)
after icacls /reset: O:SYG:SYD:AI(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1200a9;;;BU)(A;ID;0x1200a9;;;AC)(A;ID;0x1200a9;;;S-1-15-2-2)
msiexec /fa results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test run19 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
repair /fa 0: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;BU)
msiexec /fvomus results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1b3bb70.msi
  log: Product: MsiSpec Test run19 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
repair /fvomus 0: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;BU)
msiexec /x results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run19 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT file: install 0, uninstall 0, perm.txt left: false
