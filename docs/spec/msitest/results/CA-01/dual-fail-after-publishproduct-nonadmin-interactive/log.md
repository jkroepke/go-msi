# CA-01 / dual-fail-after-publishproduct-nonadmin
user SID S-1-5-21-1635116080-4240938304-3303945664-1109, elevated=false
built pkg.msi sha256 e6687e424252fa101c948b329b3f36134cbb7013751c387c7403300b5b280042
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results-interactive2\CA-01\dual-fail-after-publishproduct-nonadmin\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\3fc8b0.msi
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState 5
HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\CEE359962D1BA0D4FA14B28C1D955932 present=true
  HKEY_USERS\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\CEE359962D1BA0D4FA14B28C1D955932\SourceList PackageName REG_SZ pkg.msi LastUsedSource REG_EXPAND_SZ n;1;C:\ProgramData\msi-builder-tests\ca01\results-interactive2\CA-01\dual-fail-after-publishproduct-nonadmin\ HKEY_USERS\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\CEE359962D1BA0D4FA14B28C1D955932\SourceList\Media HKEY_USERS\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\CEE359962D1BA0D4FA14B28C1D955932\SourceList\Net
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\CEE359962D1BA0D4FA14B28C1D955932 present=true
msiexec /x {69953EEC-B1D2-4D0A-AF41-2BC8D1599523} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run13 -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall by ProductCode exit 0; state after -1
RESULT dual-fail-after-publishproduct-nonadmin: install 1603, registration left after failed install=true
