# SEC-01 / file
File row secured with D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;BU)
built pkg.msi sha256 6caa0d4717ab31600691d132d4c7a1f342bb28c37682a869928c91ab351e3163
msiexec /i results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43ebb.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;BU)
observed SDDL: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;BU)
after icacls /reset: O:SYG:SYD:AI(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1200a9;;;BU)(A;ID;0x1200a9;;;AC)(A;ID;0x1200a9;;;S-1-15-2-2)
msiexec /fa results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test sec01 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
repair /fa 0: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;BU)
after second icacls /reset: O:SYG:SYD:AI(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1200a9;;;BU)(A;ID;0x1200a9;;;AC)(A;ID;0x1200a9;;;S-1-15-2-2)
msiexec /fvomus results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43ec4.msi
  log: Product: MsiSpec Test sec01 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
repair /fvomus 0: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;BU)
msiexec /x results\SEC-01\file\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT file: install 0, uninstall 0, perm.txt left: false
