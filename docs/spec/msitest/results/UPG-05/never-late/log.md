# UPG-05 / never-late
built v1.msi sha256 4e56daad3a4d900ef2521cdd6082657c0e956c25c453dc3eb1f163f48dcef4bc
built v2.msi sha256 c624215afac1587234e7e5c21f9f1ec11030b429f40d7f49bafe52390818b3ce
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\never-late\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bf9.msi
  log: Product: MsiSpec Test upg-never-late -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\never-late\v2.msi -> exit 0
  log: Disallowing installation of component: C_reg since the registry keypath exists and the component is marked to never overwrite existing installations
  log: Disallowing installation of component: {EEF12704-AD2C-336F-8BE5-31BD36BA372C} since the keyfile exists and the component is marked to never overwrite existing installations
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bfe.msi
  log: Product: MsiSpec Test upg-never-late -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT never-late upgrade: v2 exit 0, v1 registered=false, config="user-edit", registry="user-edit"
msiexec /x {C3906D7E-7549-46EE-AFDC-EC3A97EBCC99} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-never-late -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT never-late uninstall: exit 0, v2 registered=false, readme present=false, config="<absent>", registry="<absent>"
