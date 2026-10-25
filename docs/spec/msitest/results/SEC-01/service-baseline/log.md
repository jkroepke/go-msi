# SEC-01 / service-baseline
ServiceInstall row without MsiLockPermissionsEx: the default service SDDL
built pkg.msi sha256 6c258f4c52f94b0d6545d589a7df290a9f5b2f91ef467b92b263cdb0a7b26c02
msiexec /i results\SEC-01\service-baseline\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\1c43ee7.msi
  log: Product: MsiSpec Test sec01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
observed service SDDL: D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLOCRRC;;;IU)(A;;CCLCSWLOCRRC;;;SU)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;SO)S:(AU;FA;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;WD)
msiexec /x results\SEC-01\service-baseline\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test sec01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
RESULT service-baseline: install 0, uninstall 0, perm.txt left: false
