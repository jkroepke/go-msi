# SEC-04: children without a DACL in a granted folder (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), elevated, 2026-10-30
  and 2026-10-31
- Test: `TestMsiexecGrantNullDACLChild` in
  `internal/builder/nativetest/nulldacl_windows_test.go`
- Runs: 41149480758 (PR #173, commit 865d419, the test in this PR); earlier
  probes 41144458040 (folder only, commit b32a5a5) and 41146150151 (folder,
  file and a folder written with `D:AINO_ACCESS_CONTROL`, commit 0dae6f1)

The package grants Guests (`S-1-5-32-546`) `readExecute`, inherited
(`OICI`), on `ProgramData\MsiBuilderSec04\existing`, which the test creates
first (the probes used `MsiBuilderSec03`). In it the test writes a NULL DACL
with `SetFileSecurityW` on the folder `open` and the file `open.txt`, and
leaves `keep.txt` with the DACL it inherits. DACLs are read with
`GetFileSecurityW` and `ConvertSecurityDescriptorToStringSecurityDescriptorW`:
`Get-Acl` leaves a NULL DACL out of its `Sddl` (run 41142573748 read
`O:LAG:…` without a `D:` part).

Run 41149480758:

| Step | `existing` | `keep.txt` | `open`, `open.txt` |
| --- | --- | --- | --- |
| before | `D:AI(A;OICIID;FA;;;SY)(A;OICIID;FA;;;BA)(A;ID;FA;;;LA)(A;OICIIOID;GA;;;CO)(A;OICIID;0x1200a9;;;BU)(A;CIID;DCLCRPCR;;;BU)` | `D:AI(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;FA;;;LA)(A;ID;0x1200a9;;;BU)` | `D:NO_ACCESS_CONTROL` |
| `FAILNOW=1` install (log: `granted`, then `revoked` on `existing`), 1603 | no `(A;OICI;0x1200a9;;;BG)` (asserted) | no `(A;ID;0x1200a9;;;BG)` (asserted) | `D:NO_ACCESS_CONTROL`; writing a file into `open` succeeded (asserted) |
| install | has `(A;OICI;0x1200a9;;;BG)` (asserted) | `D:AI(A;ID;0x1200a9;;;BG)(A;ID;FA;;;SY)(A;ID;FA;;;BA)(A;ID;FA;;;LA)(A;ID;0x1200a9;;;BU)` | `D:NO_ACCESS_CONTROL` |
| uninstall | — | — | `D:NO_ACCESS_CONTROL` |

- Microsoft's rules ("Automatic Propagation of Inheritable ACEs") predict
  an inherited-only DACL on the NULL-DACL children after the grant
  (`D:AI(A;OICIID;0x1200a9;;;BG)` on `open`, `D:AI(A;ID;0x1200a9;;;BG)` on
  `open.txt`) and an empty `D:AI` after the rollback. Neither appeared. The
  inheriting sibling got the ACE on the install, so the grant propagates,
  and did not have it after the rollback; the run does not read it between
  the grant and the rollback of the failed install.
- Run 41146150151: `D:AINO_ACCESS_CONTROL` written with `SetFileSecurityW`
  read back as `D:NO_ACCESS_CONTROL`, so a NULL DACL marked auto-inherited
  was not produced; the variant was dropped from the test.
- Not run: a NULL DACL marked auto-inherited, a NULL DACL set with
  `SetNamedSecurityInfoW`, an object deeper in the tree, Windows Server 2016.
