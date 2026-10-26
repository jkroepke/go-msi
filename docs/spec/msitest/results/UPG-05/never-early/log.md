# UPG-05 / never-early
built v1.msi sha256 e41af24193a3af021abd76058b195c27b94b4ed9ce9f28b1bbe33d07d0583d46
built v2.msi sha256 1ef5714feb9dc5dc6ac955c08cb1378b1c6e97be9e8f365f2fc8ca7a6a78777a
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\never-early\v1.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bad.msi
  log: Product: MsiSpec Test upg-never-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0: config="1.0.0-default" registry="1.0.0-default"
after user edit: config="user-edit" registry="user-edit"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results-b\UPG-05\never-early\v2.msi -> exit 0
  log: Disallowing installation of component: C_reg since the registry keypath exists and the component is marked to never overwrite existing installations
  log: Disallowing installation of component: {5A650736-19D0-33BC-A632-0AA59271E672} since the keyfile exists and the component is marked to never overwrite existing installations
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1bb5.msi
  log: Product: MsiSpec Test upg-never-early -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT never-early upgrade: v2 exit 0, v1 registered=false, config="<absent>", registry="<absent>"
msiexec /x {ACC1BD06-E2FC-4989-AB03-F983CE1C9B0D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg-never-early -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT never-early uninstall: exit 0, v2 registered=false, readme present=false, config="<absent>", registry="<absent>"
