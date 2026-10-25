# RUN-24 / neverperm-early
built v1.msi sha256 b46640d583571c9f613d399b0710febaf8623eff4c2396b80b0f54edbd77e782
built v2.msi sha256 e5f3df816864887509c1f657d5994ec03664a4f8aa55edd7901c546ac488c02f
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\RUN-24\neverperm-early\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bcb.msi
  log: Product: MsiSpec Test upg-neverperm-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\RUN-24\neverperm-early\v2.msi -> exit 0
  log: Disallowing installation of component: C_reg since the registry keypath exists and the component is marked to never overwrite existing installations
  log: Disallowing installation of component: {D2F59933-2FD7-3220-A824-65B3CB611D9B} since the keyfile exists and the component is marked to never overwrite existing installations
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bd2.msi
  log: Product: MsiSpec Test upg-neverperm-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT neverperm-early upgrade: v2 exit 0, v1 registered=false, config="user-edit", registry="user-edit"
msiexec /x {D293C81E-BFCF-44D8-B4E7-9368692C8C34} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-neverperm-early -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT neverperm-early uninstall: exit 0, v2 registered=false, readme present=false, config="user-edit", registry="user-edit"
  removing leftover component registration {D2F59933-2FD7-3220-A824-65B3CB611D9B}
  removing leftover component registration {10D43F3A-26FC-3564-9144-EBD64F9C989F}
