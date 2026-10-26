# SVC-03 / per-step
built pkg.msi sha256 7fff90312bd3ae19f1e259e72256d26765d8e2729a93328d293baa3f1e88f6a5
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SVC-03\per-step\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1aea.msi
  log: Product: MsiSpec Test run27 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT per-step: install 0; sc qfailure: [SC] QueryServiceConfig2 SUCCESS SERVICE_NAME: MsiSpecTestSvc RESET_PERIOD (in seconds) : 3600 REBOOT_MESSAGE : COMMAND_LINE : FAILURE_ACTIONS : RESTART -- Delay = 60000 milliseconds. RESTART -- Delay = 300000 milliseconds. RESTART -- Delay = 300000 milliseconds.
msiexec /x C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SVC-03\per-step\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run27 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT per-step: uninstall 0; service: not installed
