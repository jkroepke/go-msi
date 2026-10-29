# UPG-08: leading zeros in ProductVersion (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), elevated, 2026-10-29
- CI run 40940932552, commit a9b7cef
- Test: `TestMsiexecUpgradeVersions` in
  `internal/builder/nativetest/audit_windows_test.go`

msi-builder keeps a version as written, so `1.010.0` stays `1.010.0` in
`ProductVersion` (`Property(S): ProductVersion = 1.010.0`).

| v1 | v2 | v2 exit | Result |
| --- | --- | --- | --- |
| 1.9.0 | 1.010.0 | 0 | v1 removed, v2 installed: 010 compares as 10 |
| 1.010.0 | 1.9.0 | 1603 | refused by the downgrade launch condition, v1 stays |

The engine compares the fields as numbers. Confirmed.
