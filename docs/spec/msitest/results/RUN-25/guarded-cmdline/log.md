# RUN-25 / guarded-cmdline
built v1.msi sha256 631728633a469c72469abd8bfb19b9bc3da15fdc7c25373a88f89f084dee59a2
built v2.msi sha256 6bd6353c618aa1e0af0712ba4b36a415c31725f7cdadd2797d5e012e7f7105ef
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-25\guarded-cmdline\v1.msi INSTALLDIR=C:\MsiSpecCustom\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a9d.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0 into C:\MsiSpecCustom\: readme present=true, registry="C:\\MsiSpecCustom\\"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-25\guarded-cmdline\v2.msi INSTALLDIR=C:\MsiSpecOther\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1aa4.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
  v2 log: REMEMBERED="C:\\MsiSpecCustom\\" INSTALLDIR="C:\\MsiSpecOther\\"
RESULT guarded-cmdline: v2 exit 0, v1 registered=false, readme in custom=false, in other=true, in Program Files=false, registry="C:\\MsiSpecOther\\"
msiexec /x {E03F73AC-961F-425F-A8DB-AD6A64AFE404} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {E03F73AC-961F-425F-A8DB-AD6A64AFE404} -> 0
