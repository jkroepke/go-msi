# SEARCH-03 / guarded
built v1.msi sha256 b74fee335c9ab96b1a33d45c7f61b7870c73f03f90b30cd3263734d1990837f6
built v2.msi sha256 07825766ce44ac15c1160575afdc4e27edaf302ce9f05d2ac80b889780a12173
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SEARCH-03\guarded\v1.msi INSTALLDIR=C:\MsiSpecCustom\ -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a8f.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
v1 install 0 into C:\MsiSpecCustom\: readme present=true, registry="C:\\MsiSpecCustom\\"
msiexec /i C:\Users\Administrator\msitest-gaps\docs\spec\msitest\results\SEARCH-03\guarded\v2.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\27b1a96.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
  v2 log: REMEMBERED="C:\\MsiSpecCustom\\" INSTALLDIR="C:\\MsiSpecCustom\\"
RESULT guarded: v2 exit 0, v1 registered=false, readme in custom=true, in other=false, in Program Files=false, registry="C:\\MsiSpecCustom\\"
msiexec /x {A880BDF5-DE93-4DF4-967B-FB3D3C44E37F} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {A880BDF5-DE93-4DF4-967B-FB3D3C44E37F} -> 0
