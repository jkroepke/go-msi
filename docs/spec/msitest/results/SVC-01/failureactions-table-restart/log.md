# SVC-01 / failureactions-table-restart
built pkg.msi sha256 0ed25a8ffc0a576f1f863471b3e22f9de0d031be3b4612c8f8d66ff57bad1481
msiexec /i results\SVC-01\failureactions-table-restart\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1939. Service 'MsiSpec Test Service' (MsiSpecTestSvc) could not be configured.  This could be a problem with the package or your permissions. Verify that you have s
  log: Return value 3.
  log: Product: MsiSpec Test run05 -- Installation failed.
  log: MainEngineThread is returning 1603
install 1603; service state not installed
sc qc: [SC] OpenService FAILED 1060: The specified service does not exist as an installed service.
sc qfailure: [SC] OpenService FAILED 1060: The specified service does not exist as an installed service.
msiexec /x results\SVC-01\failureactions-table-restart\pkg.msi -> exit 1605
  log: Product: MsiSpec Test run05 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT failureactions-table-restart: install 1603, uninstall 1605, service after uninstall: not installed
