# RUN-25 / none
built v1.msi sha256 8453aca997b2fd4db87d824a1dc3063ff08b534b291a4cbb8668226ecefceace
built v2.msi sha256 1d79b20a538a6cfae0052c0edd4766ceda2ebfc4f75adbe6831740eb9af6cffe
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-25\none\v1.msi INSTALLDIR=C:\MsiSpecCustom\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a5d.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0 into C:\MsiSpecCustom\: readme present=true, registry="C:\\MsiSpecCustom\\"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\RUN-25\none\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a64.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
  v2 log: REMEMBERED="" INSTALLDIR="C:\\Program Files\\MsiSpecTest\\"
RESULT none: v2 exit 0, v1 registered=false, readme in custom=false, in other=false, in Program Files=true, registry="C:\\Program Files\\MsiSpecTest\\"
msiexec /x {672BC6BC-93D3-45CE-9519-A2266F408878} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {672BC6BC-93D3-45CE-9519-A2266F408878} -> 0
