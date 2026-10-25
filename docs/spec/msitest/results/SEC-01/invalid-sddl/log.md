# SEC-01 / invalid-sddl
SDDLText that is not SDDL (docs: error 1943)
built pkg.msi sha256 e51e51c0b187c233db90e48249506705c906c0b6ab28172423abc86a39ed0e6d
msiexec /i results\SEC-01\invalid-sddl\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1943. SDDL string 'D:garbage(' for object f_perm(in table File) could not be resolved into a valid Security Descriptor.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
install 1603
action before the first error: Action start 20:35:00: InstallFiles.
msiexec /x results\SEC-01\invalid-sddl\pkg.msi -> exit 1605
  log: Product: MsiSpec Test sec01 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT invalid-sddl: install 1603, uninstall 1605, perm.txt left: false
