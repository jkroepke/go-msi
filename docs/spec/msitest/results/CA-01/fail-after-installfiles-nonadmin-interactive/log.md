# CA-01 / fail-after-installfiles-nonadmin
user SID S-1-5-21-1635116080-4240938304-3303945664-1109, elevated=false
built pkg.msi sha256 80e8f0e3a26210eada530a89dfbc15d27c2f8feca64cf489da06f4a1510a1ba9
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results-interactive2\CA-01\fail-after-installfiles-nonadmin\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1303. The installer has insufficient privileges to access this directory: C:\Program Files\MsiSpecTest.  The installation cannot continue.  Log on as administrator
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\9581CAC27E104E94F94EA2364468D8BD present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\9581CAC27E104E94F94EA2364468D8BD present=false
msiexec /x {2CAC1859-01E7-49E4-9FE4-2A6344868DDB} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT fail-after-installfiles-nonadmin: install 1603, registration left after failed install=false
