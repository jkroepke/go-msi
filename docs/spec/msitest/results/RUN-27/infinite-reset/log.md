# RUN-27 / infinite-reset
built pkg.msi sha256 b9fb6b7901533b152784c7f7cd564c7dc39d1745b63ee9e1cd36008d4e107210
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-27\infinite-reset\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1af8.msi
  log: Product: MsiSpec Test run27 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT infinite-reset: install 0; sc qfailure: [SC] QueryServiceConfig2 SUCCESS SERVICE_NAME: MsiSpecTestSvc RESET_PERIOD (in seconds) : INFINITE REBOOT_MESSAGE : COMMAND_LINE : FAILURE_ACTIONS : RESTART -- Delay = 5000 milliseconds.
msiexec /x C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-27\infinite-reset\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run27 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT infinite-reset: uninstall 0; service: not installed
