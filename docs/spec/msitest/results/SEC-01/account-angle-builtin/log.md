# SEC-01 / account-angle-builtin
SDDLText naming an account in angle brackets, <BUILTIN\Users> (docs: account lookup)
built pkg.msi sha256 abeb1d5ecbdfa6d0d70e142220c3cd770c012e59feb56b751ac5468a413a8184
msiexec /i results\SEC-01\account-angle-builtin\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43efd.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
observed SDDL: O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;BU)
msiexec /x results\SEC-01\account-angle-builtin\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT account-angle-builtin: install 0, uninstall 0, perm.txt left: false
