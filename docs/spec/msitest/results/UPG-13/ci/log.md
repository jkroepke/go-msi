# UPG-13: a minimum version on the main upgrade row (CI)

- Runner: GitHub `windows-latest` (x64), elevated, 2026-10-31
- CI run 41222624233, commit 71fb4ae
- Test: `TestMsiexecUpgradeFloor` in
  `internal/builder/nativetest/upgradefloor_windows_test.go`

v1 is the installed version, built with the standard upgrade policy. v2
1.2.0 has the same upgrade code and `app.exe` at the same path, with
`upgrade.minVersion: 1.0.0`. Both use generated component codes.

| v1 | v2 upgrade | v2 exit | v1 registered after v2 | `app.exe` after v2 is uninstalled |
| --- | --- | --- | --- | --- |
| 0.9.0 | `minVersion: 1.0.0` | 0 | yes | present |
| 1.0.0 | `minVersion: 1.0.0` | 0 | no | gone |
| 1.0.0 | `minVersion: 1.0.0, includeMin: false` | 0 | yes | present |
| 1.1.0 | `minVersion: 1.0.0` | 0 | no | gone |
| 0.9.0 | `minVersion: 1.0.0` + related row of the own upgrade code (`maxVersion: 1.0.0`, `onlyDetect: true`, `BELOWFLOORFOUND`) + launch condition `NOT BELOWFLOORFOUND` | 1603 | yes | present (v2 not installed) |

Where v1 was below the floor, FindRelatedProducts found nothing for the
removing row and RemoveExistingProducts removed nothing. In the blocked
case FindRelatedProducts set `BELOWFLOORFOUND` to v1's product code, the
launch condition showed its message and the install failed. Uninstalling
v1 afterwards removed the folder in every case. Confirmed.
