# SEC-01 / createfolder
CreateFolder row with inheritable ACEs; a file created inside afterwards
built pkg.msi sha256 de44ca19ff77bf4e25e0bf39d8f319fce30113685da0596a074f6ba5f9986755
msiexec /i results\SEC-01\createfolder\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1b3bb85.msi
  log: Product: MsiSpec Test run19 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;BU)
observed folder SDDL: O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;BU)
file created afterwards inherits: O:BAG:DUD:(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1301bf;;;BU)
msiexec /x results\SEC-01\createfolder\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run19 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT createfolder: install 0, uninstall 0, perm.txt left: false
