# SVC-03 / zero-reset
built pkg.msi sha256 393fb82b8b218bb75c03859e9eb87dd2bf4b97ff3089677bfdbc7df85a615174
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SVC-03\zero-reset\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1aff.msi
  log: Product: MsiSpec Test run27 -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT zero-reset: install 0; sc qfailure: [SC] QueryServiceConfig2 SUCCESS SERVICE_NAME: MsiSpecTestSvc RESET_PERIOD (in seconds) : 0 REBOOT_MESSAGE : COMMAND_LINE : FAILURE_ACTIONS : RESTART -- Delay = 1000 milliseconds. RESTART -- Delay = 2000 milliseconds.
msiexec /x C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SVC-03\zero-reset\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run27 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT zero-reset: uninstall 0; service: not installed
