# UI-09 / windows-latest

GitHub Actions `windows-latest` runner (interactive desktop, elevated), CI
run 40967817046, 2026-10-29, `TestFullUIRemoveDialog` at commit 3a4cb01
(step "go test (remove page results)").

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
same `MsiExec.exe /I{ProductCode}` command; UI-08 ran that command and
reached the remove page. `/x` showed only the confirmation box and ran the
server at `UILevel` 3 (basic), with no wizard page. Which command the
Change and Uninstall buttons of Add or Remove Programs run was not tested.
