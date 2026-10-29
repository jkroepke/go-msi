# SEARCH-06: installDir.remember across scopes, platforms and registry views (CI)

- Runners: GitHub `windows-latest` (x64) and `windows-11-arm` (arm64),
  elevated, 2026-10-29
- CI run 40962873234, commit fd57ff1 (raw `RegLocator` search, before the
  switch to a directory search)
- Test: `TestMsiexecRememberInstallDir` in
  `internal/builder/nativetest/remember_windows_test.go`

Each variant installs version 1 silently with
`INSTALLDIR=C:\MsiBuilderRemember\custom\`, then upgrades silently to
version 2 without `INSTALLDIR`. The dual packages pass the same
`MSIINSTALLPERUSER` to both installs.

| Variant | Value written to | Also visible in the other view | v2 `INSTALLDIR` | v2 installs to |
| --- | --- | --- | --- | --- |
| perMachine x64 | HKLM, 64-bit view | no | `C:\MsiBuilderRemember\custom\` | custom folder |
| perUser x64 | HKCU | yes (HKCU\Software is shared) | `C:\MsiBuilderRemember\custom\` | custom folder |
| perMachine x86 | HKLM, 32-bit view | no | `C:\MsiBuilderRemember\custom\` | custom folder |
| perMachine x64, `parent: ProgramFilesFolder` | HKLM, 32-bit view | no | `C:\MsiBuilderRemember\custom\` | custom folder |
| perMachine arm64 (arm64 runner) | HKLM, 64-bit view | no | `C:\MsiBuilderRemember\custom\` | custom folder |
| perMachineOrUser, per machine | HKLM, 64-bit view | no | `C:\MsiBuilderRemember\custom\` | custom folder |
| perMachineOrUser, `MSIINSTALLPERUSER=1` | HKCU | yes (shared) | `C:\MsiBuilderRemember\custom\` | custom folder |
| perUserOrMachine, per user | HKCU | yes (shared) | `C:\MsiBuilderRemember\custom\` | custom folder |
| perUserOrMachine, `MSIINSTALLPERUSER=""` | HKLM, 64-bit view | no | `C:\MsiBuilderRemember\custom\` | custom folder |

No variant installed into its default folder. The three HKCU variants failed
only the test's own check that the value is absent from the other view:
HKCU\Software is not redirected, so `reg query /reg:32` reads the same value.
The test now checks the other view only for HKLM.

Raw lines (arm64):

```text
SEARCH-06 perMachine arm64: v1 stored "C:\\MsiBuilderRemember\\custom\\" in HKLM view 64 ("<absent>" in view 32); v2 INSTALLDIR="C:\\MsiBuilderRemember\\custom\\", ALLUSERS="1", in custom=true, in default=false
```

Raw lines (x64, HKCU variants):

```text
SEARCH-06 perUser x64: v1 stored "C:\\MsiBuilderRemember\\custom\\" in HKCU view 64 ("C:\\MsiBuilderRemember\\custom\\" in view 32); v2 INSTALLDIR="C:\\MsiBuilderRemember\\custom\\", ALLUSERS="", in custom=true, in default=false
SEARCH-06 perMachineOrUser per user: v1 stored "C:\\MsiBuilderRemember\\custom\\" in HKCU view 64 ("C:\\MsiBuilderRemember\\custom\\" in view 32); v2 INSTALLDIR="C:\\MsiBuilderRemember\\custom\\", ALLUSERS="", in custom=true, in default=false
SEARCH-06 perUserOrMachine per user: v1 stored "C:\\MsiBuilderRemember\\custom\\" in HKCU view 64 ("C:\\MsiBuilderRemember\\custom\\" in view 32); v2 INSTALLDIR="C:\\MsiBuilderRemember\\custom\\", ALLUSERS="", in custom=true, in default=false
```
