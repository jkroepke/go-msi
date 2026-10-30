# UPG-11: major upgrade with changed component codes (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated, 2026-10-30
- CI run 41058220285, commit ffe1750
- Test: `TestMsiexecChangedComponentCodes` in
  `internal/builder/nativetest/componentcodes_windows_test.go`

v1 1.0.0 pins three component codes: `App` (`app.exe`, `config.yaml` and
the machine variable `MSIBUILDER_COMPONENT_CODES`), `Settings` (the value
`Level` under `HKLM\SOFTWARE\MsiBuilderComponentCodes`) and `Menu`, a
component of one Start menu shortcut to `app.exe` with msi-builder's HKCU
key path. v2 1.1.0 has the same upgrade code and the same resources at the
same paths.

| v2 | `removeOld` | Files | Registry value | Variable | Shortcut |
| --- | --- | --- | --- | --- | --- |
| generated codes | afterInstallExecute | gone | gone | gone | gone |
| v1 codes pinned | afterInstallExecute | kept | kept | kept | kept |
| generated codes | afterInstallInitialize | kept | kept | kept | kept |
| v1 codes pinned, shortcut on the file (generated carrier) | afterInstallExecute | kept | kept | kept | gone |

Every v2 install ended with exit 0 and v1 was no longer registered. With
the late removal, v1's removal deleted every resource whose code v2 did not
share, although v2 had just installed it at the same path; the shortcut in
a carrier component with a new code was deleted too. Uninstalling v2 left
neither the folder, the shortcut nor the registry key behind. Confirmed.
