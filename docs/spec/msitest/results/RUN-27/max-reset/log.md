# RUN-27 / max-reset
built pkg.msi sha256 ababe1b4f4060e59baf970f779655e377e3d01472f8f3a9271d47c7cf4ffd136
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-27\max-reset\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1b06.msi
  log: Product: MsiSpec Test run27 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT max-reset: install 0; sc qfailure: [SC] QueryServiceConfig2 SUCCESS SERVICE_NAME: MsiSpecTestSvc RESET_PERIOD (in seconds) : 2147483647 REBOOT_MESSAGE : COMMAND_LINE : FAILURE_ACTIONS : RESTART -- Delay = 2147483647 milliseconds.
msiexec /x C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-27\max-reset\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run27 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT max-reset: uninstall 0; service: not installed
