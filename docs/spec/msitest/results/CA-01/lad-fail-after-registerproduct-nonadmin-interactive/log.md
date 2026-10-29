# CA-01 / lad-fail-after-registerproduct-nonadmin
user SID S-1-5-21-1635116080-4240938304-3303945664-1109, elevated=false
built pkg.msi sha256 d4c47c25b3d6cc43db91ba8999ed6bba6c858636ba04d07aebbb132a520c6554
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results-interactive3\CA-01\lad-fail-after-registerproduct-nonadmin\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\3fc8d7.msi
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState 2
HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\97986949A0C1838459DB73070B54BA99 present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\97986949A0C1838459DB73070B54BA99 present=true
msiexec /x {94968979-1C0A-4838-95BD-3770B045AB99} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after 2
RESULT lad-fail-after-registerproduct-nonadmin: install 1603, registration left after failed install=false
  cleanup: deleting leftover registration keys
  reg delete HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\97986949A0C1838459DB73070B54BA99: exit status 1
  reg delete HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\97986949A0C1838459DB73070B54BA99: exit status 1
  after cleanup state 2
