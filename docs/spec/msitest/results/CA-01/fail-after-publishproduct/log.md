# CA-01 / fail-after-publishproduct
user SID S-1-5-21-1635116080-4240938304-3303945664-500, elevated=true
built pkg.msi sha256 c1c914aea49c4323faf9186c23b24993e9c6915c5b70214b7405d2c1022b3fd7
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results\CA-01\fail-after-publishproduct\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\2172b1.msi
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\S-1-5-21-1635116080-4240938304-3303945664-500\Software\Microsoft\Installer\Products\DFEEE62656EC1CB48B504076BC97509F present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-500\Products\DFEEE62656EC1CB48B504076BC97509F present=false
msiexec /x {626EEEFD-CE65-4BC1-B805-0467CB7905F9} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT fail-after-publishproduct: install 1603, registration left after failed install=false
