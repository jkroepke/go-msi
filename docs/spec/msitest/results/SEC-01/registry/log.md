# SEC-01 / registry
Registry row secured; key permissions
built pkg.msi sha256 9b5babf8f90bfc4630d9bc14410f015a7e01b3c4c0472c2050440905be62ace3
msiexec /i results\SEC-01\registry\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1b3bb8c.msi
  log: Product: MsiSpec Test run19 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;BU)
observed key SDDL: O:SYG:SYD:P(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;BU)
msiexec /x results\SEC-01\registry\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run19 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT registry: install 0, uninstall 0, perm.txt left: false
