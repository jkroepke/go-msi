# RUN-07 / x64
built pkg.msi sha256 78af22905dc1d1b57a5ca610ef06c5a43906feda5a3b2a3eea5d29bd2ae70f50
msiexec /i results\RUN-07\x64\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a628.msi
  log: Product: MsiSpec Test run07x64 -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
64-bit view: Bit256=true Bit0=false
32-bit view: Bit256=false Bit0=true
msiexec /x results\RUN-07\x64\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run07x64 -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall 0; after: 64-bit has values=false, 32-bit has values=false
RESULT x64 template: bit 256 -> 64-bit view, no bit -> 32-bit view (see above)
