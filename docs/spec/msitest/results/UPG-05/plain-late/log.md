# UPG-05 / plain-late
built v1.msi sha256 6124c292ad4c9048344a42c92226412c85aee1538c387767f3f45281550b8359
built v2.msi sha256 3244b9332737982777256840b4403cbd8699ca88e638d03cef6fcdcd65170414
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\plain-late\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bd9.msi
  log: Product: MsiSpec Test upg-plain-late -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\plain-late\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bdf.msi
  log: Product: MsiSpec Test upg-plain-late -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT plain-late upgrade: v2 exit 0, v1 registered=false, config="2.0.0-default", registry="2.0.0-default"
msiexec /x {C1B0A873-D98B-4B97-A73F-70A253903D32} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-plain-late -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT plain-late uninstall: exit 0, v2 registered=false, readme present=false, config="<absent>", registry="<absent>"
