# CA-01 / lad-fail-after-installfiles-nonadmin
user SID S-1-5-21-1635116080-4240938304-3303945664-1109, elevated=false
built pkg.msi sha256 854456806960d36b812eb5a82e86d50a51ab4ab1e93337f82c731113dc851b2f
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results-interactive3\CA-01\lad-fail-after-installfiles-nonadmin\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\673C2F8A2DEB65F49B005EE303795762 present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\673C2F8A2DEB65F49B005EE303795762 present=false
msiexec /x {A8F2C376-BED2-4F56-B900-E53E30977526} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT lad-fail-after-installfiles-nonadmin: install 1603, registration left after failed install=false
