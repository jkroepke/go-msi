# UPG-09: major upgrade across platforms (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated, 2026-10-29
- CI run 40940932552, commit a9b7cef
- Test: `TestMsiexecUpgradeVersions` in
  `internal/builder/nativetest/audit_windows_test.go`

Same upgrade code, v1 1.0.0 and v2 1.1.0, default `upgrade` policy.

| v1 | v2 | v2 exit | Result |
| --- | --- | --- | --- |
| x64 | x86 | 0 | `Program Files\MsiBuilderVersions` removed, `Program Files (x86)\MsiBuilderVersions\app.exe` installed |
| x86 | x64 | 0 | `Program Files (x86)\MsiBuilderVersions` removed, `Program Files\MsiBuilderVersions\app.exe` installed |

The platform does not separate products of one upgrade code. Confirmed.
