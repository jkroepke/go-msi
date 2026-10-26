# CA-02 / exe-exit1-deferred
built pkg.msi sha256 a772a49f3469c996143a1b2ecac15d29ea4ae5092604c8b811aea1ada50c393a
msiexec /i results\CA-02\exe-exit1-deferred\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Error 1722. There is a problem with this Windows Installer package. A program run as part of the setup did not finish as expected. Contact your support personnel or packa
  log: Return value 3.
  log: Product: MsiSpec Test run18 -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT exe-exit1-deferred: exit 1603, file installed=false, registered=false, CA: Action ended 18:13:23: TestCA. Return value 1. [scheduled], last action: Action start 18:13:23: InstallFinalize.
