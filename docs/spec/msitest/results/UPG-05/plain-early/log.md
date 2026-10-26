# UPG-05 / plain-early
built v1.msi sha256 54b798adec8d1a176e8526a8a80c0b2c8b5b940d6271c90f5a92a1beb14f8f52
built v2.msi sha256 63b29937ca51d17287ceea48942b844876d7ae5b4998c315ddd61e63ce9b03ca
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\plain-early\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1b9d.msi
  log: Product: MsiSpec Test upg-plain-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\plain-early\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1ba5.msi
  log: Product: MsiSpec Test upg-plain-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT plain-early upgrade: v2 exit 0, v1 registered=false, config="2.0.0-default", registry="2.0.0-default"
msiexec /x {17F867A0-283F-45D7-AA9B-EC2E8F70C3F6} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-plain-early -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT plain-early uninstall: exit 0, v2 registered=false, readme present=false, config="<absent>", registry="<absent>"
