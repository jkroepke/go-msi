# SVC-03 / two-steps
built pkg.msi sha256 ea22ef375e7ebae8bafcc2089ed73d20aabed51c952b20b874678ac802156e99
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SVC-03\two-steps\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1af1.msi
  log: Product: MsiSpec Test run27 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT two-steps: install 0; sc qfailure: [SC] QueryServiceConfig2 SUCCESS SERVICE_NAME: MsiSpecTestSvc RESET_PERIOD (in seconds) : 86400 REBOOT_MESSAGE : COMMAND_LINE : FAILURE_ACTIONS : RESTART -- Delay = 1000 milliseconds.
msiexec /x C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SVC-03\two-steps\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run27 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT two-steps: uninstall 0; service: not installed
