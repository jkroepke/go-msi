# SEARCH-03 / direct-nosignature
built v1.msi sha256 ef14657ae8228d5ab9b8e9714032724563b042efdcc5c2de53782a98632008f0
built v2.msi sha256 833b30857fb1029d01aef3ba9fd87dc19f15d5963b8b4bc0a8934e9b8b862386
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SEARCH-03\direct-nosignature\v1.msi INSTALLDIR=C:\MsiSpecCustom\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a79.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0 into C:\MsiSpecCustom\: readme present=true, registry="C:\\MsiSpecCustom\\"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SEARCH-03\direct-nosignature\v2.msi -> exit 1603
  log: Error 2228:  Database: . Unknown table 'Signature' in SQL query: SELECT `FileName`, `MinVersion`, `MaxVersion`, `MinSize` , `MaxSize`, `MinDate`, `MaxDate`, `Languages` F
  log: Return value 3.
  log: MainEngineThread is returning 1603
  v2 log: REMEMBERED="" INSTALLDIR=""
RESULT direct-nosignature: v2 exit 1603, v1 registered=true, readme in custom=true, in other=false, in Program Files=false, registry="C:\\MsiSpecCustom\\"
msiexec /x {5F7E7AA3-E7B1-47A4-92AC-C02FD118759A} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {5F7E7AA3-E7B1-47A4-92AC-C02FD118759A} -> 0
