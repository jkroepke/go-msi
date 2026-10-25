# SEC-01 / two-true-conditions
two rows for the same object, both true (docs: error 1942)
built pkg.msi sha256 628a5d23129587175595bafb11d799e93639b6252be7ce2a759653c817927490
msiexec /i results\SEC-01\two-true-conditions\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1942. Multiple conditions ('1' and '1')have resolved to true while installing Object f_perm (from table File). This may be a problem with the package.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
install 1603
action before the first error: Action start 20:35:01: InstallFiles.
msiexec /x results\SEC-01\two-true-conditions\pkg.msi -> exit 1605
  log: Product: MsiSpec Test sec01 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT two-true-conditions: install 1603, uninstall 1605, perm.txt left: false
