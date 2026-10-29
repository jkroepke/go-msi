# CA-01 / fail-after-installfiles
user SID S-1-5-21-1635116080-4240938304-3303945664-500, elevated=true
built pkg.msi sha256 b95a56fbadb56a036118b273d8fd9fee8f98c902df76893e23335b6c2442577c
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results\CA-01\fail-after-installfiles\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\S-1-5-21-1635116080-4240938304-3303945664-500\Software\Microsoft\Installer\Products\96051349DF319A6479796416BC2355CE present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-500\Products\96051349DF319A6479796416BC2355CE present=false
msiexec /x {94315069-13FD-46A9-9797-4661CB3255EC} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT fail-after-installfiles: install 1603, registration left after failed install=false
