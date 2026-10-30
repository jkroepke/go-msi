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

### Registry key below an existing key

The key was `HKLM\SOFTWARE\MsiBuilderTest\Sec02`, and other tests create
and delete `HKLM\SOFTWARE\MsiBuilderTest`. With `-shuffle=on` the parent
sometimes existed before the install, and the key then read back as
`O:SYG:SYD:P(A;CI;KA;;;SY)(A;CI;KR;;;NS)(A;CI;KA;;;BA)` (main, CI run
41018777226, 2026-10-30): a protected DACL with exactly the authored ACEs,
as for files. The `ID` result above is what a key gets when the install
also creates its parent.

Since PR #141 the test uses its own root `HKLM\SOFTWARE\MsiBuilderSec02`,
creates `Existing` before the install, and writes `New\Key` and
`Existing\Key`. CI run 41036088523 (2026-10-30) passed both assertions:
`Existing\Key` has `D:P` with the three authored ACEs, `New\Key` the
three authored ACEs marked `ID` and no others.

## Grants

CI run 40974613442, commit 39126ee (`TestMsiexecGrants`, which asserts
what follows). The package grants `readExecute` on `app.exe` and `modify`
on the `data` folder and on a `locked` folder (which also has
`sddl: D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)`) to `[SVC_ACCOUNT]`, `full` to
`NT AUTHORITY\LocalService` and `readExecute` to `S-1-5-32-546` (Guests,
which already has `(A;OICI;FR;;;BG)`) on the `existing` folder, and `start`
and `stop` on the service to `BU`. `SVC_ACCOUNT=BUILTIN\Guests`.

| Object | SDDL after install |
| --- | --- |
| `existing` folder, before | `D:AI(A;OICI;FR;;;BG)(A;OICIID;FA;;;SY)(A;OICIID;FA;;;BA)(A;ID;FA;;;LA)(A;OICIIOID;GA;;;CO)(A;OICIID;0x1200a9;;;BU)(A;CIID;DCLCRPCR;;;BU)` |
| `existing` folder | `D:AI(A;OICI;FA;;;LS)(A;OICI;0x1200a9;;;BG)(A;OICIID;FA;;;SY)…` (all inherited ACEs kept; the Guests ACE merged from FR to 0x1200a9) |
| `existing\keep.txt` (made by the test before) | `D:AI(A;ID;0x1200a9;;;BG)(A;ID;FA;;;LS)(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;FA;;;LA)(A;ID;0x1200a9;;;BU)` |
| `app.exe` | `D:AI(A;;0x1200a9;;;BG)(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;0x1200a9;;;BU)(A;ID;0x1200a9;;;AC)(A;ID;0x1200a9;;;S-1-15-2-2)` |
| `data` folder | `D:AI(A;OICI;0x1301bf;;;BG)(A;OICIID;FA;;;SY)(A;OICIID;FA;;;BA)…` |
| `locked` folder | `D:PAI(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;0x1301bf;;;BG)` |
| service | `D:…(A;;CCLCSWLOCRRC;;;SU)(A;;RPWP;;;BU)S:…` |

- An install with `FAILNOW=1` (a deferred action that fails after the
  grants) returned 1603, logged `BuilderGrant: granted` and `revoked`, and
  left the `existing` folder with its ACEs from before; no rollback record
  and no remembered account were left.
- An install without `SVC_ACCOUNT` returned 1603 with `no account to grant
  rights to`.
- After the install, `reg query HKLM\SOFTWARE\msi-builder\GrantAccounts /s`
  listed `BUILTIN\Guests`.
- `msiexec /fa` without the property logged `granting to the remembered
  account` and `already granted` for the `data` folder, and every file and
  folder DACL had the ACEs of the install afterwards. `app.exe` was
  overwritten (`REINSTALLMODE specifies all files to be overwritten`) and
  then logged `BuilderGrant: C:\Program Files\MsiBuilderGrant\app.exe:
  already granted, left as it is`: the replaced file kept its explicit ACE
  (job 123799612563).
- In run 40968697181 (commit 8db502a), `msiexec /fa <package>
  SVC_ACCOUNT=BUILTIN\Guests FAILNOW=1` set neither property: the log's
  property dump had no `FAILNOW`, `FailNow` was skipped (condition false)
  and the grants with `[SVC_ACCOUNT]` were skipped as empty. The failing
  repair therefore uses `/i <package> REINSTALL=ALL REINSTALLMODE=amus`,
  which returned 1603 and left every DACL (the file, all three folders, the
  file in the existing folder) and the service with the ACEs of the install.
- The DACL's AI flag can drop while the ACEs stay: the rollback of the
  failed install turned the `existing` folder's `D:AI(` into `D:(` (run
  40964477950), a `/fa` repair did the same on `app.exe` (run
  40966318115), and the failed `REINSTALL=ALL` repair turned the `locked`
  folder's `D:PAI(` into `D:P(` (run 40971961598). The test compares the
  ACEs without that flag since.
- Uninstalling left the `existing` folder with its granted ACEs and removed
  the remembered account.
