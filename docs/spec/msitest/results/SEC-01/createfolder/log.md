# SEC-01 / createfolder
CreateFolder row with inheritable ACEs; a file created inside afterwards
built pkg.msi sha256 76c533af63a6e150bf01e870fcc36f9109e1600ba41dcb2f74feadbc3e7384d5
msiexec /i results\SEC-01\createfolder\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43ed9.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;BU)
observed folder SDDL: O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;BU)
file created afterwards inherits: O:BAG:DUD:(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1301bf;;;BU)
msiexec /x results\SEC-01\createfolder\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT createfolder: install 0, uninstall 0, perm.txt left: false
