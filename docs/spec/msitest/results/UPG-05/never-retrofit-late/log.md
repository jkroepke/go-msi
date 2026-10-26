# UPG-05 / never-retrofit-late
built v1.msi sha256 93b72930b725a4c2b40fb9093e133a97a8cb681ecafa9bfe66adceb4d3ccc105
built v2.msi sha256 dedcb660e256cebaf2f9ace3a71326f015a6170713b012227d49bfe755b45cd9
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\never-retrofit-late\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1c17.msi
  log: Product: MsiSpec Test upg-never-retrofit-late -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\never-retrofit-late\v2.msi -> exit 0
  log: Disallowing installation of component: C_reg since the registry keypath exists and the component is marked to never overwrite existing installations
  log: Disallowing installation of component: {8BE23393-61E4-36A3-970A-AA0E702532F8} since the keyfile exists and the component is marked to never overwrite existing installations
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1c1c.msi
  log: Product: MsiSpec Test upg-never-retrofit-late -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT never-retrofit-late upgrade: v2 exit 0, v1 registered=false, config="user-edit", registry="user-edit"
msiexec /x {4FC93806-C751-4526-8568-B543F6CC4D9B} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-never-retrofit-late -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT never-retrofit-late uninstall: exit 0, v2 registered=false, readme present=false, config="<absent>", registry="<absent>"
