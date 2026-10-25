# SEC-01 / with-lockpermissions
LockPermissions and MsiLockPermissionsEx in one package (spec: error 1941)
built pkg.msi sha256 acd27090ed5a27249792c059193c56c5498d518c482847c307e7b3619510d53a
msiexec /i results\SEC-01\with-lockpermissions\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1941. Both LockPermissions and MsiLockPermissionsEx tables were found in the package. Only one of them should be present. This is a problem with the package.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
install 1603
action before the first error: Action start 20:35:01: InstallFiles.
msiexec /x results\SEC-01\with-lockpermissions\pkg.msi -> exit 1605
  log: Product: MsiSpec Test sec01 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT with-lockpermissions: install 1603, uninstall 1605, perm.txt left: false
