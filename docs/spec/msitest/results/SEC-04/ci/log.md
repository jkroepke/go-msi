# SEC-04: children without a DACL in a granted folder (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), elevated, 2026-10-30
  and 2026-10-31
- Test: `TestMsiexecGrantNullDACLChild` in
  `internal/builder/nativetest/nulldacl_windows_test.go`
- Runs: 41144458040 (folder only, commit b32a5a5) and 41146150151 (folder,
  file and a folder written with `D:AINO_ACCESS_CONTROL`, commit 0dae6f1)

The package grants Guests (`S-1-5-32-546`) `readExecute`, inherited
(`OICI`), on `ProgramData\MsiBuilderSec03\existing`, which the test creates
first. In it the test writes a NULL DACL with `SetFileSecurityW` on the
folder `open` and the file `open.txt`. DACLs are read with
`GetFileSecurityW` and `ConvertSecurityDescriptorToStringSecurityDescriptorW`:
`Get-Acl` leaves a NULL DACL out of its `Sddl` (the first run, 41142573748,
read `O:LAG:…` without a `D:` part).

| Step | `existing` | `open`, `open.txt` |
| --- | --- | --- |
| before | `D:AI(A;OICIID;FA;;;SY)(A;OICIID;FA;;;BA)(A;ID;FA;;;LA)(A;OICIIOID;GA;;;CO)(A;OICIID;0x1200a9;;;BU)(A;CIID;DCLCRPCR;;;BU)` | `D:NO_ACCESS_CONTROL` |
| `FAILNOW=1` install (log: `granted`, then `revoked` on `existing`), 1603 | — | `D:NO_ACCESS_CONTROL`; writing a file into `open` succeeded |
| install | `D:AI(A;OICI;0x1200a9;;;BG)(A;OICIID;FA;;;SY)…` | `D:NO_ACCESS_CONTROL` |
| uninstall | — | `D:NO_ACCESS_CONTROL` |

- Microsoft's rules ("Automatic Propagation of Inheritable ACEs") predict
  `D:AI(A;OICIID;0x1200a9;;;BG)` on the children after the grant and an
  empty `D:AI` after the rollback. Neither appeared.
- `D:AINO_ACCESS_CONTROL` written with `SetFileSecurityW` read back as
  `D:NO_ACCESS_CONTROL`, so a NULL DACL marked auto-inherited was not
  produced; the variant was dropped from the test.
- Not run: an object deeper in the tree, a NULL DACL set with
  `SetNamedSecurityInfoW`, Windows Server 2016.
