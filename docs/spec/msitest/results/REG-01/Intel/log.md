# REG-01 / Intel
built pkg.msi sha256 c7514dd04c942b6815e3da95ea49cffcf463ffec6a6723544ec173cfb78a2e6c
msiexec /i results\REG-01\Intel\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a62f.msi
  log: Product: MsiSpec Test run07Intel -- Installation completed successfully.
  log: MainEngineThread is returning 0
install 0
64-bit view: Bit256=true Bit0=false
32-bit view: Bit256=false Bit0=true
msiexec /x results\REG-01\Intel\pkg.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run07Intel -- Removal completed successfully.
  log: MainEngineThread is returning 0
uninstall 0; after: 64-bit has values=false, 32-bit has values=false
RESULT Intel template: bit 256 -> 64-bit view, no bit -> 32-bit view (see above)
