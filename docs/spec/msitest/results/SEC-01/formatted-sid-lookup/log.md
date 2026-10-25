# SEC-01 / formatted-sid-lookup
SDDLText with [LogonUser], an account name rather than a SID string or alias
built pkg.msi sha256 0e179e84866ae7dc64ac84f065ca322283c940bc7cf9474715ca2b07d0474044
msiexec /i results\SEC-01\formatted-sid-lookup\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1943. SDDL string 'D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;[LogonUser])' for object f_perm(in table File) could not be resolved into a valid Security Descriptor.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
install 1603
msiexec /x results\SEC-01\formatted-sid-lookup\pkg.msi -> exit 1605
  log: Product: MsiSpec Test run19 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT formatted-sid-lookup: install 1603, uninstall 1605, perm.txt left: false
