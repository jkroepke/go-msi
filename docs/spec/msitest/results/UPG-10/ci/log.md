# UPG-10: major upgrade when removing the old product fails (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated, 2026-10-29
- CI run 40965856107, commit 2e91637
- Test: `TestMsiexecIgnoreRemoveFailure` in
  `internal/builder/nativetest/upgrade_windows_test.go`

v1 1.0.0 has a type-19 action after InstallInitialize, conditioned
`REMOVE~="ALL" AND NOT KEEPGOING`, so every removal of it fails. v2 1.1.0
has the same upgrade code and no such action. Both packages carry the
same `removeOld` and `ignoreRemoveFailure`; IgnoreRemoveFailure is bit 4 of
the removing Upgrade row (517 = 1 + 4 + 512).

| `removeOld` | Bit 4 | REP return | v2 exit | v1 registered | v2 registered |
| --- | --- | --- | --- | --- | --- |
| afterInstallInitialize | no | 3 | 1603 | yes | no |
| afterInstallValidate | yes | 1 | 0 | yes | yes |
| afterInstallInitialize | yes | 1 | 0 | yes | yes |
| afterInstallExecute | yes | 1 | 0 | yes | yes |
| afterInstallFinalize | yes | 1 | 0 | yes | yes |

The nested removal logs the error action's message
(`Product: MsiBuilderRemoveFailure -- The old version refuses to be
removed.`). Without the bit `RemoveExistingProducts` ends with return value
3 and the install rolls back; with it the action ends with 1 and the install
completes (`Installation completed successfully.`). "Registered" is
`MsiQueryProductState` returning `INSTALLSTATE_DEFAULT`; `app.exe` was in the
install folder after every v2 install. Both products uninstalled cleanly
afterwards (v1 with `KEEPGOING=1`). Confirmed.
