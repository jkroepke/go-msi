# CA-01 / fail-after-publishproduct-nonadmin
user SID S-1-5-21-1635116080-4240938304-3303945664-1109, elevated=false
built pkg.msi sha256 bdd6f66962974fcaef826f40fb54105f01b90bfd82d314bd3abf40fb32f473b9
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results-interactive2\CA-01\fail-after-publishproduct-nonadmin\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1303. The installer has insufficient privileges to access this directory: C:\Program Files\MsiSpecTest.  The installation cannot continue.  Log on as administrator
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\4281CCAEACEC5854AB264795328E3639 present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\4281CCAEACEC5854AB264795328E3639 present=false
msiexec /x {EACC1824-CECA-4585-BA62-745923E86393} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT fail-after-publishproduct-nonadmin: install 1603, registration left after failed install=false
