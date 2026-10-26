# RUN-24 / neverperm-retrofit-early
built v1.msi sha256 7040783eaf605ad430e5e46a5c475d8ee3a319f722b990e54cdb7d060371fb57
built v2.msi sha256 cbf5ba0bd93edf2072f26bf9dbbf46b9d2804c8e02674c22d13c5faf7164152c
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\RUN-24\neverperm-retrofit-early\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1c08.msi
  log: Product: MsiSpec Test upg-neverperm-retrofit-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\RUN-24\neverperm-retrofit-early\v2.msi -> exit 0
  log: Disallowing installation of component: C_reg since the registry keypath exists and the component is marked to never overwrite existing installations
  log: Disallowing installation of component: {174B614E-5A18-3AE5-8378-1EDF91A4B9DB} since the keyfile exists and the component is marked to never overwrite existing installations
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1c10.msi
  log: Product: MsiSpec Test upg-neverperm-retrofit-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT neverperm-retrofit-early upgrade: v2 exit 0, v1 registered=false, config="<absent>", registry="<absent>"
msiexec /x {22D9BC9B-239A-47E8-9F7D-5BB603A8896B} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-neverperm-retrofit-early -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT neverperm-retrofit-early uninstall: exit 0, v2 registered=false, readme present=false, config="<absent>", registry="<absent>"
  removing leftover component registration {174B614E-5A18-3AE5-8378-1EDF91A4B9DB}
  removing leftover component registration {3B6F8657-AA9C-3364-B2F6-FE0DBDFE9322}
