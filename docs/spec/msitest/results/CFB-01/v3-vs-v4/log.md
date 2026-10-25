# CFB-01 / v3-vs-v4
built native.msi sha256 872b855daea7c0c1180f46d57cc32f79d1e1f5f5c41d791b12bbb01c756cb37c
native output: CFB v4, 20480 bytes, root CLSID 84 10 0c 00 00 00 00 00 c0 00 00 00 00 00 00 46
variant repack-v3.msi sha256 421e925b9d190a41e836dc771396f63c621f5c7ccd66cc8643a00f40327ae8ad
v3: 6656 bytes, all streams byte-identical to native: true
repack v3: open ok; ProductCode [[{A148E6C3-2419-4331-8681-FD2C101127EA}]] err=<nil>; 8 tables
msiexec /i results\CFB-01\v3-vs-v4\repack-v3.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3c5.msi
  log: Product: MsiSpec Test cfb01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CFB-01\v3-vs-v4\repack-v3.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cfb01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
repack-v3: install 0, uninstall 0
variant repack-v4.msi sha256 e7cd38890fd1863cacbb79b8859229fa7a04ba93b481244dcccd999ddf5b7659
v4: 20480 bytes, all streams byte-identical to native: true
repack v4: open ok; ProductCode [[{A148E6C3-2419-4331-8681-FD2C101127EA}]] err=<nil>; 8 tables
msiexec /i results\CFB-01\v3-vs-v4\repack-v4.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3cc.msi
  log: Product: MsiSpec Test cfb01 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CFB-01\v3-vs-v4\repack-v4.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cfb01 -- Removal completed successfully.
  log: MainEngineThread is returning 0
repack-v4: install 0, uninstall 0
