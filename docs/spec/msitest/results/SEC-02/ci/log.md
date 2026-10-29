# SEC-02: SDDL without the P flag, and permission grants (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), elevated, 2026-10-29
- Tests: `TestMsiexecSDDLWithoutProtection` and `TestMsiexecGrants` in
  `internal/builder/nativetest/permissions_windows_test.go`

## MsiLockPermissionsEx SDDL without P

CI run 40959118227, commit 5d41654 (the probe, which only logged). The
package secures `app.exe` with `D:(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;NS)`, two
CreateFolder rows below `ProgramData\MsiBuilderSec02` with
`D:(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;NS)` (`existing` was
created by the test first, with an explicit `(OI)(CI)R` ACE for Guests),
and a Registry row with `D:(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;NS)`.

| Object | SDDL before | SDDL after install |
| --- | --- | --- |
| parent `MsiBuilderSec02` install folder | inherits from Program Files | `D:AI(A;ID;FA;;;S-1-5-80-…)…(A;ID;0x1200a9;;;BU)…` |
| `app.exe` | — | `O:SYG:SYD:P(A;;FA;;;SY)(A;;FA;;;NS)(A;;FA;;;BA)` |
| `new` folder | — | `O:SYG:SYD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;NS)(A;OICI;FA;;;BA)` |
| `existing` folder | `D:AI(A;OICI;FR;;;BG)(A;OICIID;FA;;;SY)(A;OICIID;FA;;;BA)(A;ID;FA;;;LA)…(A;OICIID;0x1200a9;;;BU)…` | `O:LA…D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;NS)(A;OICI;FA;;;BA)` |
| registry key | — | `O:SYG:SYD:(A;CIID;KA;;;SY)(A;CIID;KA;;;BA)(A;CIID;KR;;;NS)` |

Files and folders get a protected DACL with exactly the authored ACEs,
reordered, whatever the SDDL's flags: the inherited ACEs are gone and the
existing folder lost its Guests ACE. The registry key is not protected,
but holds only the authored ACEs, marked inherited (`ID`), and none of
`HKLM\SOFTWARE`'s. A `/fa` repair in that run left every DACL as after the
install. The test now asserts this.

## Grants

Pending: the CI run of the grant tests on the final head.
