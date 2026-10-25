# RUN-13 / fail-after-installfiles
built pkg.msi sha256 2f8f8bfd76b55024a613b292aec0ddd88c18ba24ea9d945b2db2de1bec089450
msiexec /i C:\Users\Administrator\GolandProjects\windows_exporter\installer\msitest\results\RUN-13\fail-after-installfiles\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\<SID>\Software\Microsoft\Installer\Products\BAE293AF0DE79CA45A2EF21B5A4CD54D present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\<SID>\Products\BAE293AF0DE79CA45A2EF21B5A4CD54D present=false
msiexec /x {FA392EAB-7ED0-4AC9-A5E2-2FB1A5C45DD4} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT fail-after-installfiles: install 1603, registration left after failed install=false
