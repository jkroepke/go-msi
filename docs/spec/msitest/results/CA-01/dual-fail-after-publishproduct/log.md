# CA-01 / dual-fail-after-publishproduct
user SID S-1-5-21-1635116080-4240938304-3303945664-500, elevated=true
built pkg.msi sha256 c832eff322ac930b448c64785fe779ee8ea2eccc02a06b4a69b4be542ca555a0
msiexec /i C:\ProgramData\msi-builder-tests\ca01\results\CA-01\dual-fail-after-publishproduct\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\2172b8.msi
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\S-1-5-21-1635116080-4240938304-3303945664-500\Software\Microsoft\Installer\Products\5AFB4D418EB48BF4795DC96D5F3F381E present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\S-1-5-21-1635116080-4240938304-3303945664-500\Products\5AFB4D418EB48BF4795DC96D5F3F381E present=false
msiexec /x {14D4BFA5-4BE8-4FB8-97D5-9CD6F5F383E1} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT dual-fail-after-publishproduct: install 1603, registration left after failed install=false
