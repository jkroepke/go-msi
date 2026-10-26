# SVC-01 / no-delete-event
built pkg.msi sha256 965e9be1578114cc2dbb73fa7fa5450565e6ecac6c5a3c0270812df7c53c47ec
msiexec /i results\SVC-01\no-delete-event\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a68a.msi
  log: Product: MsiSpec Test run05 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0; service state 4 RUNNING
sc qc: [SC] QueryServiceConfig SUCCESS SERVICE_NAME: MsiSpecTestSvc TYPE : 10 WIN32_OWN_PROCESS START_TYPE : 3 DEMAND_START ERROR_CONTROL : 1 NORMAL BINARY_PATH_NAME : "C:\Program Files\MsiSpecTest\testsvc.exe" LOAD_ORDER_GROUP : TAG : 0 DISPLAY_NAME : MsiSpec Test Service DEPENDENCIES : SERVICE_START_NAME : LocalSystem
sc qfailure: [SC] QueryServiceConfig2 SUCCESS SERVICE_NAME: MsiSpecTestSvc RESET_PERIOD (in seconds) : 0 REBOOT_MESSAGE : COMMAND_LINE :
msiexec /fa results\SVC-01\no-delete-event\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test run05 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
repair 0; service state 4 RUNNING
msiexec /x results\SVC-01\no-delete-event\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run05 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT no-delete-event: install 0, uninstall 0, service after uninstall: 1 STOPPED
  cleanup: sc delete -> [SC] DeleteService SUCCESS
