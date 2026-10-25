# CFB-03 / empty-stream-start
built base.msi sha256 2119a6ccc88c26b75f852445fa009a1bb5f69ab44f17a08537edc917488731ea
create Binary: <nil>
insert empty Binary row: <nil>
commit: <nil>
native writer: size-0 streams: [Binary.Empty start=0xfffffffe]
variant empty-start-fffffffe.msi sha256 af1cdb336285b6240daa42966788da87d999d64ae41fcfef90c25b49b7619309
empty start 0xfffffffe: open ok; ProductCode [[{87AE5AEC-CDCF-4D52-A1B6-7B0A7A4ECA70}]] err=<nil>; 9 tables
  native read Binary.Empty -> 0 bytes err=MsiRecordReadStream: 13
variant empty-start-fffffffe-commit.msi sha256 af1cdb336285b6240daa42966788da87d999d64ae41fcfef90c25b49b7619309
  native insert+commit: <nil> / <nil>
msiexec /i results\CFB-03\empty-stream-start\empty-start-fffffffe.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a4c8.msi
  log: Product: MsiSpec Test cfb03 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CFB-03\empty-stream-start\empty-start-fffffffe.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cfb03 -- Removal completed successfully.
  log: MainEngineThread is returning 0
empty-start-fffffffe: install 0, uninstall 0
RESULT empty stream start 0xfffffffe -> install 0
variant empty-start-ffffffff.msi sha256 b49bb9a2da2d23069273c1185250b81616c087fbeae0256e98818c95344f0348
empty start 0xffffffff: open ok; ProductCode [[{87AE5AEC-CDCF-4D52-A1B6-7B0A7A4ECA70}]] err=<nil>; 9 tables
  native read Binary.Empty -> 0 bytes err=MsiRecordReadStream: 13
variant empty-start-ffffffff-commit.msi sha256 b49bb9a2da2d23069273c1185250b81616c087fbeae0256e98818c95344f0348
  native insert+commit: <nil> / <nil>
msiexec /i results\CFB-03\empty-stream-start\empty-start-ffffffff.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a4cf.msi
  log: Product: MsiSpec Test cfb03 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CFB-03\empty-stream-start\empty-start-ffffffff.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cfb03 -- Removal completed successfully.
  log: MainEngineThread is returning 0
empty-start-ffffffff: install 0, uninstall 0
RESULT empty stream start 0xffffffff -> install 0
variant empty-start-0.msi sha256 6a5ff70448374778d4e3757376bf060d4818b40dfd9d0c7079502ac867cfab15
empty start 0x0: open ok; ProductCode [[{87AE5AEC-CDCF-4D52-A1B6-7B0A7A4ECA70}]] err=<nil>; 9 tables
  native read Binary.Empty -> 0 bytes err=MsiRecordReadStream: 13
variant empty-start-0-commit.msi sha256 6a5ff70448374778d4e3757376bf060d4818b40dfd9d0c7079502ac867cfab15
  native insert+commit: <nil> / <nil>
msiexec /i results\CFB-03\empty-stream-start\empty-start-0.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a4d6.msi
  log: Product: MsiSpec Test cfb03 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CFB-03\empty-stream-start\empty-start-0.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cfb03 -- Removal completed successfully.
  log: MainEngineThread is returning 0
empty-start-0: install 0, uninstall 0
RESULT empty stream start 0x0 -> install 0
