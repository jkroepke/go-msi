# SEARCH-03 / direct
built v1.msi sha256 888f8237fb1f4529776b4ae818c74da66d035260dcd0a0abe8c995cb10a948b3
built v2.msi sha256 504b9bd8c5764e16b92d8d53e6263a80e2b81a24750808c19c9d273151266d99
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SEARCH-03\direct\v1.msi INSTALLDIR=C:\MsiSpecCustom\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a6b.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0 into C:\MsiSpecCustom\: readme present=true, registry="C:\\MsiSpecCustom\\"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SEARCH-03\direct\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a72.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
  v2 log: REMEMBERED="" INSTALLDIR="C:\\MsiSpecCustom\\"
RESULT direct: v2 exit 0, v1 registered=false, readme in custom=true, in other=false, in Program Files=false, registry="C:\\MsiSpecCustom\\"
msiexec /x {016CB4D0-DCD5-4CEC-BE1B-F25C0FABCC0D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {016CB4D0-DCD5-4CEC-BE1B-F25C0FABCC0D} -> 0
