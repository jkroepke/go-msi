# SEC-01 / two-true-conditions
two rows for the same object, both true (docs: error 1942)
built pkg.msi sha256 9eca761c65b0def3236dfecd1f7461156bb5582d2cdf48878c26ad6bd91616d1
msiexec /i results\SEC-01\two-true-conditions\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1942. Multiple conditions ('1' and '1')have resolved to true while installing Object f_perm (from table File). This may be a problem with the package.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
install 1603
msiexec /x results\SEC-01\two-true-conditions\pkg.msi -> exit 1605
  log: Product: MsiSpec Test run19 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT two-true-conditions: install 1603, uninstall 1605, perm.txt left: false
