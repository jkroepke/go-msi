# SEC-01 / invalid-sddl
SDDLText that is not SDDL (docs: error 1943)
built pkg.msi sha256 faca65cd4abd3b140f22201d58b651b95a6a6bd6adbbe93467d3ce1d9980cca5
msiexec /i results\SEC-01\invalid-sddl\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1943. SDDL string 'D:garbage(' for object f_perm(in table File) could not be resolved into a valid Security Descriptor.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
install 1603
msiexec /x results\SEC-01\invalid-sddl\pkg.msi -> exit 1605
  log: Product: MsiSpec Test run19 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT invalid-sddl: install 1603, uninstall 1605, perm.txt left: false
