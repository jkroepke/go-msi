# SEARCH-07: a remembered install folder that does not exist (probe)

- Runner: GitHub `windows-latest`, elevated, 2026-10-29
- CI run 40962873234, commit fd57ff1
- Test: the probe version of `TestMsiexecRememberMissingFolder` in
  `internal/builder/nativetest/remember_windows_test.go`

The package (perMachine x64, `installDir.remember`) is installed fresh with
`/qn` after the test writes `InstallDir` under
`HKLM\Software\msi-builder tests\MsiBuilderRemember` (64-bit view). Each
folder ran with the raw search msi-builder wrote until then
(`RegLocator.Type` 18) and with a directory search (`Type` 16, the same
package edited with `msidb`).

| Remembered folder | Search | Exit | `INSTALLDIR_REMEMBERED` | Installed to |
| --- | --- | --- | --- | --- |
| `C:\MsiBuilderRemember\existing\` (exists) | raw | 0 | the folder | the folder |
| same | directory | 0 | the folder (with trailing `\`) | the folder |
| `C:\MsiBuilderRemember\missing\` (does not exist) | raw | 0 | the folder | the folder, created |
| same | directory | 0 | empty | `C:\Program Files\MsiBuilderRemember\` |
| `Z:\MsiBuilderRemember\` (drive not mounted) | raw | **1603** | the folder | nothing |
| same | directory | 0 | empty | `C:\Program Files\MsiBuilderRemember\` |

Raw probe lines:

```text
existing folder (C:\MsiBuilderRemember\existing\), raw search: exit 0, INSTALLDIR_REMEMBERED="C:\\MsiBuilderRemember\\existing\\", INSTALLDIR="C:\\MsiBuilderRemember\\existing\\", in remembered=true, in default=false
existing folder (C:\MsiBuilderRemember\existing\), directory search: exit 0, INSTALLDIR_REMEMBERED="C:\\MsiBuilderRemember\\existing\\", INSTALLDIR="C:\\MsiBuilderRemember\\existing\\", in remembered=true, in default=false
missing folder (C:\MsiBuilderRemember\missing\), raw search: exit 0, INSTALLDIR_REMEMBERED="C:\\MsiBuilderRemember\\missing\\", INSTALLDIR="C:\\MsiBuilderRemember\\missing\\", in remembered=true, in default=false
missing folder (C:\MsiBuilderRemember\missing\), directory search: exit 0, INSTALLDIR_REMEMBERED="", INSTALLDIR="C:\\Program Files\\MsiBuilderRemember\\", in remembered=false, in default=true
missing drive folder (Z:\MsiBuilderRemember\), raw search: exit 1603, INSTALLDIR_REMEMBERED="Z:\\MsiBuilderRemember\\", INSTALLDIR="Z:\\MsiBuilderRemember\\", in remembered=false, in default=false
missing drive folder (Z:\MsiBuilderRemember\), directory search: exit 0, INSTALLDIR_REMEMBERED="", INSTALLDIR="C:\\Program Files\\MsiBuilderRemember\\", in remembered=false, in default=true
```

The raw search on the missing drive failed in costing:

```text
MSI (s) (C0:80) [22:15:44:060]: Note: 1: 1327 2: Z:\
MSI (s) (C0:80) [22:15:44:060]: Product: MsiBuilderRemember -- Error 1327. Invalid Drive: Z:\
Error 1327. Invalid Drive: Z:\
```

msi-builder switched to the directory search; `TestMsiexecRememberMissingFolder`
now asserts the directory rows.

With the directory search (CI run 40966065131, commit 99dddc0),
`TestMsiexecRememberMissingFolder` passed:

```text
SEARCH-07 existing folder C:\MsiBuilderRemember\existing\: INSTALLDIR_REMEMBERED="C:\\MsiBuilderRemember\\existing\\", INSTALLDIR="C:\\MsiBuilderRemember\\existing\\"
SEARCH-07 missing folder C:\MsiBuilderRemember\missing\: INSTALLDIR_REMEMBERED="", INSTALLDIR="C:\\Program Files\\MsiBuilderRemember\\"
SEARCH-07 missing drive folder Z:\MsiBuilderRemember\: INSTALLDIR_REMEMBERED="", INSTALLDIR="C:\\Program Files\\MsiBuilderRemember\\"
```
