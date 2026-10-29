# PROP-01: secure public properties in an elevated install (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, build 26100), 2026-10-29
- CI run 40945525570, commit c09f4da, step "go test (non-admin)"
- Test: `TestNonAdminSecureProperties` in
  `internal/builder/nativetest/nonadmin_windows_test.go`

The package has two public properties with the default value `default`:
`PUB_SECURE` (secure, the msi-builder default, so it is listed in
`SecureCustomProperties`) and `PUB_PLAIN` (`secure: false`). A deferred
no-impersonate `run` action echoes both and `Privileged`. Both installs pass
`PUB_SECURE=cmd PUB_PLAIN=cmd` on the command line, silently.

| Install | Exit | Echoed by the deferred action |
| --- | --- | --- |
| Administrator (elevated runner account) | 0 | `SECURE=cmd PLAIN=cmd Privileged=1` |
| Standard user `msitest`, elevated by `AlwaysInstallElevated` (HKLM and the user's policy key), with `DisableMsi` 0 set by the CI step | 0 | `SECURE=cmd PLAIN=default Privileged=1` |

An administrator passes every public property to the server. In a standard
user's elevated install, the server keeps only the properties listed in
`SecureCustomProperties`; the unlisted one falls back to the package value.
Confirmed. Managed (advertised or group-policy assigned) installs were not run;
advertised installs are a non-goal.

The `AlwaysInstallElevated` values were removed again after the run, and the
package uninstalled.
