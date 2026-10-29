# LANG-01 / embedded-language-transform
GetUserDefaultUILanguage = 1033 (0x0409)
GetSystemDefaultUILanguage = 1033 (0x0409)
GetUserDefaultLangID = 1033 (0x0409)
GetSystemDefaultLangID = 1033 (0x0409)
PowerShell (system locale, culture, UI culture, user languages, UI override):
  en-US
  en-US
  en-US
  en-US
built base.msi sha256 0cd4772f7857613a3bb7126e234606e4818ef983bb0bde83dcdb9ca08b32ac84
transform 1031: 1031.mst (ProductLanguage 1031, ProductName "MsiSpec Test lang01 DE") sha256 c7af49d3fe325acbf4255a7e2f3495396a75556ebea633fc0a10ca7efe48dcf8
transform 1033: 1033.mst (ProductLanguage 1033, ProductName "MsiSpec Test lang01 EN") sha256 c663f8148e39cefcbd1ddf58e27e221df98e3ba47972e26f05a3075baa81178e
## v01: Template "x64;1033,1031", storages [1031 1033], arguments []
msiexec /i v01.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28b74.msi
  log: Product: MsiSpec Test lang01 base -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT v01: Template "x64;1033,1031" storages [1031 1033] args [] -> install 0
v01: installed ProductName "MsiSpec Test lang01 base", Language "1033", Transforms ""
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test lang01 base -- Removal completed successfully.
  log: MainEngineThread is returning 0
v01: uninstall 0
## v02: Template "x64;1031,1033", storages [1031 1033], arguments []
msiexec /i v02.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28b7d.msi
  log: Product: MsiSpec Test lang01 EN -- Installation completed successfully.
  log: MainEngineThread is returning 0
  lang: MSI (s) (48:E8) [13:30:41:328]: Transforming table Property.
  lang: MSI (s) (48:E8) [13:30:41:421]: Transforming table Property.
RESULT v02: Template "x64;1031,1033" storages [1031 1033] args [] -> install 0
v02: installed ProductName "MsiSpec Test lang01 EN", Language "1033", Transforms ""
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test lang01 EN -- Removal completed successfully.
  log: MainEngineThread is returning 0
v02: uninstall 0
## v03: Template "x64;1033,1031", storages [1031], arguments []
msiexec /i v03.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28b86.msi
  log: Product: MsiSpec Test lang01 base -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT v03: Template "x64;1033,1031" storages [1031] args [] -> install 0
v03: installed ProductName "MsiSpec Test lang01 base", Language "1033", Transforms ""
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test lang01 base -- Removal completed successfully.
  log: MainEngineThread is returning 0
v03: uninstall 0
## v04: Template "x64;1031,1033", storages [1031], arguments []
msiexec /i v04.msi -> exit 1624
  log: Note: 1: 2203 2: 1033 3: -2147287038
  log: Error 2203:  Database: 1033. Cannot open database file. System error -2147287038
  log: Product: MsiSpec Test lang01 base -- Installation failed.
  log: MainEngineThread is returning 1624
RESULT v04: Template "x64;1031,1033" storages [1031] args [] -> install 1624
## v05: Template "x64;1033,1031", storages [], arguments []
msiexec /i v05.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28b90.msi
  log: Product: MsiSpec Test lang01 base -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT v05: Template "x64;1033,1031" storages [] args [] -> install 0
v05: installed ProductName "MsiSpec Test lang01 base", Language "1033", Transforms ""
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test lang01 base -- Removal completed successfully.
  log: MainEngineThread is returning 0
v05: uninstall 0
## v06: Template "x64;1033,1031", storages [1031 1033], arguments ["ProductLanguage=1033"]
msiexec /i v06.msi ProductLanguage=1033 -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28b99.msi
  log: Product: MsiSpec Test lang01 base -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT v06: Template "x64;1033,1031" storages [1031 1033] args ["ProductLanguage=1033"] -> install 0
v06: installed ProductName "MsiSpec Test lang01 base", Language "1033", Transforms ""
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test lang01 base -- Removal completed successfully.
  log: MainEngineThread is returning 0
v06: uninstall 0
## v07: Template "x64;1033,1031", storages [1033 1031], arguments ["ProductLanguage=1031"]
msiexec /i v07.msi ProductLanguage=1031 -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28ba2.msi
  log: MainEngineThread is returning 0
  lang: MSI (s) (48:28) [13:30:45:074]: Transforming table Property.
  lang: MSI (s) (48:28) [13:30:45:180]: Transforming table Property.
RESULT v07: Template "x64;1033,1031" storages [1033 1031] args ["ProductLanguage=1031"] -> install 0
v07: installed ProductName "MsiSpec Test lang01 DE", Language "1031", Transforms ""
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: MainEngineThread is returning 0
v07: uninstall 0
## v08: Template "x64;1033", storages [1031 1033], arguments ["TRANSFORMS=:1031"]
msiexec /i v08.msi TRANSFORMS=:1031 -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28bab.msi
  log: MainEngineThread is returning 0
  lang: MSI (s) (48:B8) [13:30:45:946]: Transforming table Property.
  lang: MSI (s) (48:B8) [13:30:46:033]: Transforming table Property.
RESULT v08: Template "x64;1033" storages [1031 1033] args ["TRANSFORMS=:1031"] -> install 0
v08: installed ProductName "MsiSpec Test lang01 DE", Language "1033", Transforms ":1031"
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: MainEngineThread is returning 0
v08: uninstall 0
## v09: Template "x64;1033,1031", storages [1031 1033], arguments ["TRANSFORMS=:1033"]
msiexec /i v09.msi TRANSFORMS=:1033 -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28bb4.msi
  log: Product: MsiSpec Test lang01 EN -- Installation completed successfully.
  log: MainEngineThread is returning 0
  lang: MSI (s) (48:FC) [13:30:46:847]: Transforming table Property.
  lang: MSI (s) (48:FC) [13:30:46:938]: Transforming table Property.
RESULT v09: Template "x64;1033,1031" storages [1031 1033] args ["TRANSFORMS=:1033"] -> install 0
v09: installed ProductName "MsiSpec Test lang01 EN", Language "1033", Transforms ":1033"
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test lang01 EN -- Removal completed successfully.
  log: MainEngineThread is returning 0
v09: uninstall 0
## v10: Template "x64;1033", storages [1031 1033], arguments []
msiexec /i v10.msi -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Note: 1: 2318 2: C:\Windows\Installer\28bbd.msi
  log: Product: MsiSpec Test lang01 base -- Installation completed successfully.
  log: MainEngineThread is returning 0
RESULT v10: Template "x64;1033" storages [1031 1033] args [] -> install 0
v10: installed ProductName "MsiSpec Test lang01 base", Language "1033", Transforms ""
msiexec /x {6A1B2C3D-4E5F-4A7B-8C9D-0E1F2A3B4C5D} -> exit 0
  log: Note: 1: 2727 2:
  log: Note: 1: 2265 2:  3: -2147287035
  log: Note: 1: 2318 2:
  log: Product: MsiSpec Test lang01 base -- Removal completed successfully.
  log: MainEngineThread is returning 0
v10: uninstall 0
