# RUN-18 / exe-exit1-immediate
built pkg.msi sha256 a9190c320363f36b5a33efdfcef25bf78a7530918ee88f66ea9b50ddb4c9df32
msiexec /i results\RUN-18\exe-exit1-immediate\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1721. There is a problem with this Windows Installer package. A program required for this install to complete could not be run. Contact your support personnel or pa
  log: Return value 3.
  log: MainEngineThread is returning 1603
RESULT exe-exit1-immediate: exit 1603, file installed=false, registered=false, CA: Action ended 18:13:21: TestCA. Return value 3., last action: Action start 18:13:21: TestCA.
