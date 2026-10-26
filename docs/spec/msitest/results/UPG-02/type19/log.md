# UPG-02 / type19
built newer.msi sha256 b4d1f00b13da68960bab416b61b68d05eac80f9a16f7e919d5bb5e43c25ec31f
built older.msi sha256 532454c7f2dac1178b3a5a1de5d1e870b4db3e286932e5110b81cb86c38fcf96
msiexec /i results\UPG-02\type19\newer.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a5c1.msi
  log: Product: MsiSpec Test upg -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\UPG-02\type19\older.msi -> exit 1603
  log: Return value 3.
  log: Product: MsiSpec Test upg -- Installation failed.
  log: MainEngineThread is returning 1603
RESULT type19: downgrade exit 1603; newer still registered=true; older registered=false
msiexec /x {033169A5-7FD6-42CE-8E63-0908C882ADA6} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test upg -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {033169A5-7FD6-42CE-8E63-0908C882ADA6} -> 0
