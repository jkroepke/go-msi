# CA-01 / dual-fail-after-installfiles-nonadmin
user SID S-1-5-21-1635116080-4240938304-3303945664-1109, elevated=false
built pkg.msi sha256 de047583d03d439d74b29509776402fba2ced84805fb7f57317594e79576a18b
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results-interactive3\CA-01\dual-fail-after-installfiles-nonadmin\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\S-1-5-21-1635116080-4240938304-3303945664-1109\Software\Microsoft\Installer\Products\B56126AB09EECEE48B71EBD04B00E57B present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-1109\Products\B56126AB09EECEE48B71EBD04B00E57B present=false
msiexec /x {BA62165B-EE90-4EEC-B817-BE0DB4005EB7} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT dual-fail-after-installfiles-nonadmin: install 1603, registration left after failed install=false
