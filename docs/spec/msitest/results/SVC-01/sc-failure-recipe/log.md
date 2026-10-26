# SVC-01 / sc-failure-recipe
built pkg.msi sha256 10935abe810322929a644789619c32f6f0c3a714d12c2f28c8e355e145f06fc1
msiexec /i results\SVC-01\sc-failure-recipe\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a698.msi
  log: Product: MsiSpec Test run05 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0; service state 4 RUNNING
sc qc: [SC] QueryServiceConfig SUCCESS SERVICE_NAME: MsiSpecTestSvc TYPE : 10 WIN32_OWN_PROCESS START_TYPE : 3 DEMAND_START ERROR_CONTROL : 1 NORMAL BINARY_PATH_NAME : "C:\Program Files\MsiSpecTest\testsvc.exe" LOAD_ORDER_GROUP : TAG : 0 DISPLAY_NAME : MsiSpec Test Service DEPENDENCIES : SERVICE_START_NAME : LocalSystem
sc qfailure: [SC] QueryServiceConfig2 SUCCESS SERVICE_NAME: MsiSpecTestSvc RESET_PERIOD (in seconds) : 86400 REBOOT_MESSAGE : COMMAND_LINE : FAILURE_ACTIONS : RESTART -- Delay = 1000 milliseconds.
msiexec /fa results\SVC-01\sc-failure-recipe\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test run05 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
repair 0; service state 4 RUNNING
msiexec /x results\SVC-01\sc-failure-recipe\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run05 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT sc-failure-recipe: install 0, uninstall 0, service after uninstall: not installed
