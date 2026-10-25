# SEC-01 / service
ServiceInstall row secured; BU may start and stop the service
built pkg.msi sha256 9a939709cf4b72ec67b68f55c5e50adf390878e82be908b5917552c7da071b61
msiexec /i results\SEC-01\service\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1b3bb93.msi
  log: Product: MsiSpec Test run19 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
authored SDDL: D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLORPWPRC;;;BU)
observed service SDDL: D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWRPWPLORC;;;BU)S:(AU;FA;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;WD)
msiexec /x results\SEC-01\service\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run19 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT service: install 0, uninstall 0, perm.txt left: false
