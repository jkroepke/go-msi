# INST-02 non-admin on the CI runner

- Runner: GitHub `windows-latest` (Windows Server 2025), 2026-10-29
- CI run 40942811121, commit 574a68f, step "go test (non-admin)"
- Test: `TestNonAdminInstallScopes` in
  `internal/builder/nativetest/nonadmin_windows_test.go`

The step creates a local standard user `msitest` with a random password and
sets `HKLM\SOFTWARE\Policies\Microsoft\Windows\Installer\DisableMsi=0`
(Windows Server refuses non-admin installs by default, 1625). The test
starts `msiexec /qn` as that user with `CreateProcessWithLogonW`
(`LOGON_WITH_PROFILE`).

| Scope | Exit | Server properties | Result |
| --- | --- | --- | --- |
| perUser | 0 | `INSTALLDIR` `C:\Users\msitest\AppData\Local\Programs\MsiBuilderNonAdmin\`, `ALLUSERS` and `Privileged` unset | installed in the profile, uninstall 0, removed |
| perUserOrMachine | 0 | same folder, `MSIINSTALLPERUSER=1`, `ALLUSERS` unset | installed in the profile, uninstall 0, removed |
| perMachineOrUser | 1603 | `ALLUSERS=1`, `INSTALLDIR` under `C:\Program Files` | error 1925 in the log |
| perMachine | 1603 | `ALLUSERS=1` | error 1925 in the log |

The same as the SERVER2025 run ([nonadmin](../nonadmin/log.md)), now repeated
on every CI run. Confirmed.
