# SEC-01 / formatted-property
SDDLText containing [PERMSID] with PERMSID=BU (docs: properties not supported)
built pkg.msi sha256 63f8487a77340474c75c1b62d1161cb7d43753d6b25720c243a1f407dc10f3a4
msiexec /i results\SEC-01\formatted-property\pkg.msi -> exit 1603
  log: Note: 1: 2727 2:
  log: Error 1943. SDDL string 'D:PAI(A;;FA;;;SY)(A;;FA;;;BA)(A;;FRFX;;;[PERMSID])' for object f_perm(in table File) could not be resolved into a valid Security Descriptor.
  log: Note: 1: 2265 2:  3: -2147287035
  log: Return value 3.
  log: MainEngineThread is returning 1603
install 1603
action before the first error: Action start 20:34:56: InstallFiles.
msiexec /x results\SEC-01\formatted-property\pkg.msi -> exit 1605
  log: Product: MsiSpec Test sec01 -- Installation failed.
  log: MainEngineThread is returning 1605
RESULT formatted-property: install 1603, uninstall 1605, perm.txt left: false
