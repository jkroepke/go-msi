# UI-10 / windows-latest

GitHub Actions `windows-latest` runner (interactive desktop, elevated), CI
run 40967817046, 2026-10-29, `TestFullUIRemoveDialog` at commit 3a4cb01
(step "go test (remove page results)"). The run logged UI-09 as UI-08 and
UI-10 as UI-09; the cases were renumbered afterwards because UI-08 was
taken by another case.

The Add or Remove Programs entry of the package (`ARPNOMODIFY` unset), then
`msiexec /x` on the package with the full UI, the driver pressing Yes:

```text
HKEY_LOCAL_MACHINE\Software\Microsoft\Windows\CurrentVersion\Uninstall\{454CA58B-0EF2-55DC-B792-811FF5DD9ECF}
    AuthorizedCDFPrefix    REG_SZ    
    Comments    REG_SZ    
    Contact    REG_SZ    
    DisplayVersion    REG_SZ    1.0.0
    HelpLink    REG_SZ    
    HelpTelephone    REG_SZ    
    InstallDate    REG_SZ    20261029
    InstallLocation    REG_SZ    
    InstallSource    REG_SZ    C:\Users\RUNNER~1\AppData\Local\Temp\TestFullUIRemoveDialog3064779439\001\
    ModifyPath    REG_EXPAND_SZ    MsiExec.exe /I{454CA58B-0EF2-55DC-B792-811FF5DD9ECF}
    Publisher    REG_SZ    msi-builder tests
    Readme    REG_SZ    
    Size    REG_SZ    
    EstimatedSize    REG_DWORD    0x1c
    UninstallString    REG_EXPAND_SZ    MsiExec.exe /I{454CA58B-0EF2-55DC-B792-811FF5DD9ECF}
    URLInfoAbout    REG_SZ    
    URLUpdateInfo    REG_SZ    
    VersionMajor    REG_DWORD    0x1
    VersionMinor    REG_DWORD    0x0
    WindowsInstaller    REG_DWORD    0x1
    Version    REG_DWORD    0x1000000
    Language    REG_DWORD    0x409
    DisplayName    REG_SZ    MsiBuilderRemoveUI

msiexec /x:
WINDOW: Yes | No | Are you sure you want to uninstall this product?
PRESSED: Yes
DRIVER: done

log excerpt:
MSI (s) (34:AC) [22:57:31:850]: PROPERTY CHANGE: Adding UILevel property. Its value is '3'.
Property(S): UILevel = 3
```

The entry has no `NoModify`, and `ModifyPath` and `UninstallString` are the
same `MsiExec.exe /I{ProductCode}` command (the dump above; the test at
this commit only checked the `MsiExec.exe` prefix). The keep run of UI-09
ran `msiexec` with those arguments and reached the remove page. During
`/x` the driver, which polls every 500 ms, saw only the confirmation box,
the verbose log has no `Dialog created` line (the excerpt filter included
it), and the server ran at `UILevel` 3 (basic). The 74 `Note: 1: 2205`
lines (missing optional tables) are left out of the excerpt. Which command the
Change and Uninstall buttons of Add or Remove Programs run was not tested.
