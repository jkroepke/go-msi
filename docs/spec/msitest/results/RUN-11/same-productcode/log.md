# RUN-11 / same-productcode
built a.msi sha256 79b865f33203cdd15df99d5a3ed278cef8cf8a035084eb8db1521ae711d4f474
built b-new-packagecode.msi sha256 393db5a3caa1764391beb0baefe337856b4aa0f2d2fb9fc62f603789759b9f24
variant a-copy.msi sha256 79b865f33203cdd15df99d5a3ed278cef8cf8a035084eb8db1521ae711d4f474
msiexec /i results\RUN-11\same-productcode\a.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a618.msi
  log: Product: MsiSpec Test run11 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-11\same-productcode\a-copy.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Product: MsiSpec Test run11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
msiexec /i results\RUN-11\same-productcode\b-new-packagecode.msi -> exit 1638
  log: Product: MsiSpec Test run11 -- Configuration failed.
  log: MainEngineThread is returning 1638
msiexec /i results\RUN-11\same-productcode\b-new-packagecode.msi REINSTALL=ALL REINSTALLMODE=vomus -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2318 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a621.msi
  log: Product: MsiSpec Test run11 -- Configuration completed successfully.
  log: MainEngineThread is returning 0
RESULT same package again -> 0; same ProductCode, new PackageCode -> 1638; with REINSTALL=ALL REINSTALLMODE=vomus -> 0
msiexec /x {7EFCBC76-BF56-44DF-B83B-7E61D0C4555F} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test run11 -- Removal completed successfully.
  log: MainEngineThread is returning 0
  cleanup {7EFCBC76-BF56-44DF-B83B-7E61D0C4555F} -> 0
