# SEC-01 / registry
Registry row secured; key permissions
built pkg.msi sha256 710a59202071850617ea746daa7f5b583b5b50cc2fd2f989e083302558e5060a
msiexec /i results\SEC-01\registry\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43ee0.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;BU)
observed key SDDL: O:SYG:SYD:P(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;BU)
msiexec /x results\SEC-01\registry\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT registry: install 0, uninstall 0, perm.txt left: false
