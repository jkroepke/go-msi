# RUN-24 / plain-late-nohash
built v1.msi sha256 116a4bc0395e795ff47517984709646a25d96553fc2feb0404a6bc2a3b8f67d0
built v2.msi sha256 f2c2f6dc1965e79d9eb06378ffd45b09d8a2276fdbc3e99477ca0042311aaa42
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\RUN-24\plain-late-nohash\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1be9.msi
  log: Product: MsiSpec Test upg-plain-late-nohash -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\RUN-24\plain-late-nohash\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bef.msi
  log: Product: MsiSpec Test upg-plain-late-nohash -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT plain-late-nohash upgrade: v2 exit 0, v1 registered=false, config="2.0.0-default", registry="2.0.0-default"
msiexec /x {C91FE0BC-19CE-4CAE-9AF1-9C5BD601BA01} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-plain-late-nohash -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT plain-late-nohash uninstall: exit 0, v2 registered=false, readme present=false, config="<absent>", registry="<absent>"
