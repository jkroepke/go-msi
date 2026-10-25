# CFB-02 / root-clsid
built native.msi sha256 13dc56c2f08e98c07579bf34ed40792de53489643eeba783847cf27845a7ff92
variant correct.msi sha256 13dc56c2f08e98c07579bf34ed40792de53489643eeba783847cf27845a7ff92
correct: generic CFB reader ok
correct: open ok; ProductCode [[{00374327-A66F-4E34-B660-1BA84A5E88E6}]] err=<nil>; 8 tables
msiexec /i results\CFB-02\root-clsid\correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\WINDOWS\Installer\138a3d3.msi
  log: Product: MsiSpec Test cfb02 -- Installation completed successfully.
  log: MainEngineThread is returning 0
msiexec /x results\CFB-02\root-clsid\correct.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test cfb02 -- Removal completed successfully.
  log: MainEngineThread is returning 0
correct: install 0, uninstall 0
RESULT root CLSID correct -> install 0
variant zero.msi sha256 77dc887f74a9bff085368a3f0a1a5bd777138ac0c24362b4ec6676ea20cef869
zero: generic CFB reader ok
zero: MsiOpenDatabaseW -> MsiOpenDatabaseW: 110
msiexec /i results\CFB-02\root-clsid\zero.msi -> exit 1620
  log: Note: 1: 2766 2: results\CFB-02\root-clsid\zero.msi
  log: MainEngineThread is returning 1620
zero: install 1620
RESULT root CLSID zero -> install 1620
variant one-byte.msi sha256 b6cadaf845dae76b42ebf371ecbf361da311efd9a783d1152660e35eb4868849
one-byte: generic CFB reader ok
one-byte: MsiOpenDatabaseW -> MsiOpenDatabaseW: 110
msiexec /i results\CFB-02\root-clsid\one-byte.msi -> exit 1620
  log: Note: 1: 2766 2: results\CFB-02\root-clsid\one-byte.msi
  log: MainEngineThread is returning 1620
one-byte: install 1620
RESULT root CLSID one-byte -> install 1620
variant mst-clsid.msi sha256 37a92f87fae5f60287730e30e0044e134f7db501501f3295fa53df4c8fe79d70
mst-clsid: generic CFB reader ok
mst-clsid: MsiOpenDatabaseW -> MsiOpenDatabaseW: 110
msiexec /i results\CFB-02\root-clsid\mst-clsid.msi -> exit 1620
  log: Note: 1: 2766 2: results\CFB-02\root-clsid\mst-clsid.msi
  log: MainEngineThread is returning 1620
mst-clsid: install 1620
RESULT root CLSID mst-clsid -> install 1620
