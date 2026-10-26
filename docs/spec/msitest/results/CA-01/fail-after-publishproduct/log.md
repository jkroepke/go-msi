# CA-01 / fail-after-publishproduct
built pkg.msi sha256 3975e5358b27482bb6ea6392dd1e9f90fd4d9fa048a70ea3c42be30e51ef65cf
msiexec /i C:\Users\Administrator\GolandProjects\windows_exporter\installer\msitest\results\CA-01\fail-after-publishproduct\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\163c76b.msi
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run13 -- Installation failed.
  log: MainEngineThread is returning 1603
install exit 1603; MsiQueryProductState -1
HKU\<SID>\Software\Microsoft\Installer\Products\DCE99060CBFF82448824C785D71E67E4 present=false
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData\<SID>\Products\DCE99060CBFF82448824C785D71E67E4 present=false
msiexec /x {06099ECD-FFBC-4428-8842-7C587DE1764E} -> exit 1605
  log: MainEngineThread is returning 1605
uninstall by ProductCode exit 1605; state after -1
RESULT fail-after-publishproduct: install 1603, registration left after failed install=false
