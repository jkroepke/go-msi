# SEC-01 / account-angle-env
SDDLText with <[%USERDOMAIN]\[%USERNAME]> as owner and ACE trustee (docs example)
built pkg.msi sha256 abd790e27fc50abaa7d1a9a2e5e8c266a1a4c90a740c5aa23a3bb5c86da1e3dd
msiexec /i results\SEC-01\account-angle-env\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43f04.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
observed SDDL: O:LAG:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;0x1200a9;;;LA)
msiexec /x results\SEC-01\account-angle-env\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT account-angle-env: install 0, uninstall 0, perm.txt left: false
