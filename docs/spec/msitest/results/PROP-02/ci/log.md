# PROP-02: operating-system properties (CI)

- Runner: GitHub `windows-latest`, elevated, 2026-10-29
- CI run 40944028040, commit 269ca2d, step "go test (spec audit results)"
  (first run without the registry search: CI run 40940932552, same values)
- Test: `TestMsiexecOSProperties` in
  `internal/builder/nativetest/audit_windows_test.go`

`RtlGetVersion`, which no compatibility shim changes: 10.0.26100, product
type 3 (server), service pack 0.0. The server properties of the install log:

| Property | Value |
| --- | --- |
| VersionNT | 603 |
| VersionNT64 | 603 |
| WindowsBuild | 9600 |
| ServicePackLevel | 0 |
| MsiNTProductType | 3 |
| Msix64 | 25 |
| Intel | 25 |
| VersionMsi | 5.00 |
| VersionDatabase | 500 |
| OS_BUILD (registry search of `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion` `CurrentBuildNumber`) | 26100 |
| BUILD_GE_22000 (type 51 under `OS_BUILD >= 22000`) | 1 |
| BUILD_GE_99999 (type 51 under `OS_BUILD >= 99999`) | not set |

`VersionNT` 603 on current Windows is confirmed. `WindowsBuild` is 9600, the
Windows 8.1 value, on build 26100, so it does not tell releases apart:
contradicted. A registry search of `CurrentBuildNumber` returns the real
build, and a condition compares it with an integer as a number. Windows 11
was not run.
