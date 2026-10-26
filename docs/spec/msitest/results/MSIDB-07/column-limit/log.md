# MSIDB-07 / column-limit
31 columns: CREATE TABLE -> <nil>
  INSERT -> <nil>
  commit -> <nil>
  reopen SELECT -> 1 rows, 31 fields, err=<nil>
  _Columns rows for Wide: 31
  raw: 31 columns in catalog, stream present=true
32 columns: CREATE TABLE -> MsiViewExecute: 1627 (1: 2280 2: results\MSIDB-07\column-limit\cols32.msi 3: C32 4: Wide )
  reopen -> MsiOpenDatabaseW: 110
33 columns: CREATE TABLE -> MsiViewExecute: 1627 (1: 2280 2: results\MSIDB-07\column-limit\cols33.msi 3: C32 4: Wide )
  reopen -> MsiOpenDatabaseW: 110
31 columns via MsiDatabaseImportW -> <nil>; commit <nil>
32 columns via MsiDatabaseImportW -> MsiDatabaseImportW(Wide.idt): 1627 (1: 2216 2: results\MSIDB-07\column-limit\idt32.msi 3: Wide.idt 4: 4 ); commit <nil>
33 columns via MsiDatabaseImportW -> MsiDatabaseImportW(Wide.idt): 1627 (1: 2216 2: results\MSIDB-07\column-limit\idt33.msi 3: Wide.idt 4: 1 ); commit <nil>
ALTER path: create 31 -> <nil>
ALTER TABLE ADD 32nd column -> MsiViewExecute: 1627 (1: 2280 2: results\MSIDB-07\column-limit\alter.msi 3: C32 4: Wide )
