# CA-01 / lad-fail-after-publishproduct-nonadmin
user SID S-1-5-21-1635116080-4240938304-3303945664-1109, elevated=false
built pkg.msi sha256 5a30125f74c6ce0a8b0261ec58e02a4f14b08e560d6928c06cd18f4e2281b164
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results-interactive3\CA-01\lad-fail-after-publishproduct-nonadmin\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\3fc8d0.msi
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState 5
HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\281E919614EC5394E832F2CBAA65313C present=true
  HKEY_USERS\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\281E919614EC5394E832F2CBAA65313C\SourceList PackageName REG_SZ pkg.msi LastUsedSource REG_EXPAND_SZ n;1;C:\ProgramData\msi-builder-tests\ca01\results-interactive3\CA-01\lad-fail-after-publishproduct-nonadmin\ HKEY_USERS\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\281E919614EC5394E832F2CBAA65313C\SourceList\Media HKEY_USERS\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\281E919614EC5394E832F2CBAA65313C\SourceList\Net
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\281E919614EC5394E832F2CBAA65313C present=true
msiexec /x {6919E182-CE41-4935-8E23-2FBCAA5613C3} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run13 -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall by ProductCode exit 0; state after -1
RESULT lad-fail-after-publishproduct-nonadmin: install 1603, registration left after failed install=true
