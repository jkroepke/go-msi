# RUN-25 / direct-cmdline
built v1.msi sha256 b5abb3d363833808b3c04d3822c27c8ea6086cb40dedb3b83f7da14a976e403f
built v2.msi sha256 1f39195e426ed6c2e7d64f3938daf9dff2e9bd983d083a5e15fec70a7a95d0e2
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-25\direct-cmdline\v1.msi INSTALLDIR=C:\MsiSpecCustom\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a81.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0 into C:\MsiSpecCustom\: readme present=true, registry="C:\\MsiSpecCustom\\"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-25\direct-cmdline\v2.msi INSTALLDIR=C:\MsiSpecOther\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a88.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
  v2 log: REMEMBERED="" INSTALLDIR="C:\\MsiSpecCustom\\"
RESULT direct-cmdline: v2 exit 0, v1 registered=false, readme in custom=true, in other=false, in Program Files=false, registry="C:\\MsiSpecCustom\\"
msiexec /x {FCA98A88-EC87-4F20-ACB4-F8E7B4E734DF} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {FCA98A88-EC87-4F20-ACB4-F8E7B4E734DF} -> 0
