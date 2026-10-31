# SEC-05: remembered grant accounts of per-user installs (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), 2026-10-30
- Tests: `TestMsiexecGrantsPerUser` and `TestNonAdminGrantsPerUser` in
  `internal/builder/nativetest/grants_peruser_windows_test.go`, and
  `repairWithPlantedAccount`, called by `TestMsiexecGrants`
  (`permissions_windows_test.go`)
- Runs: CI run 41143226769 (first design, `|[UserSID]` in the data) and CI
  run 41145746768 (commit eeb29fe, the DLL reads `UserSID` itself and
  follows the registered context); every test passed in both

## Per-user package as the administrator

The elevated runner account installs a `scope: perUser` package with
`SVC_ACCOUNT=BUILTIN\Guests` and a `grant` on
`[LocalAppDataFolder]MsiBuilderGrantUser`. The folder gets
`(A;OICI;0x1301bf;;;BG)` next to its inherited ACEs:

```text
O:LAG:S-1-5-21-…-513D:AI(A;OICI;0x1301bf;;;BG)(A;OICIID;FA;;;SY)(A;OICIID;FA;;;BA)(A;OICIID;FA;;;LA)
```

`HKCU\Software\msi-builder\GrantAccounts\<ProductCode>` holds
`[SVC_ACCOUNT] = BUILTIN\Guests`, and
`HKLM\SOFTWARE\msi-builder\GrantAccounts\<ProductCode>` does not exist. A
`/fa` repair logs `granting to the remembered account`, and after the
removal the HKCU key is gone.

## Per-user package as a standard user

The same package installed by the runner's throwaway standard user
(`msitest`, CreateProcessWithLogonW with its profile): the install log has
the `BuilderGrantUser` setter and `BuilderGrant: granted: C:\Users\msitest\AppData\Local\MsiBuilderGrantUser`
and `to: BUILTIN\Guests`, no `cannot remember`; `reg query
HKCU\Software\msi-builder\GrantAccounts\<ProductCode> /v [SVC_ACCOUNT]` run
as that user exits 0 after the install, a `/fa` repair grants to the
remembered account, and the query exits 1 after the removal. Before this
change the commit action, which runs as that user, could not write HKLM.

## Per-machine repair with ALLUSERS=""

SEC-02's per-machine package, installed with `SVC_ACCOUNT=BUILTIN\Guests`,
then `BUILTIN\Users` planted under `[SVC_ACCOUNT]` in the administrator's
`HKCU\Software\msi-builder\GrantAccounts\<ProductCode>` and a repair
`msiexec /i … REINSTALL=ALL REINSTALLMODE=amus ALLUSERS="" /qn` without an
account. It returned 0, granted to the account remembered in HKLM, and the
data folder got no ACE for Users. The engine itself reset the property:

```text
Command Line: REINSTALL=ALL REINSTALLMODE=amus ALLUSERS= …
PROPERTY CHANGE: Deleting ALLUSERS property. Its current value is '1'.
PROPERTY CHANGE: Adding ALLUSERS property. Its value is '1'.
Skipping action: BuilderGrantUser (condition is false)
```

So in a maintenance install of a per-machine product the per-user marker
never reached the DLL here, and the DLL's own check of the registered
context (`MsiGetProductInfoExW`) is defence in depth that this run did not
need.

CI run 41148675668 (commit 3c89b07, native job 124379949900) ran the final
DLL, whose commit action logs the context it found. The commit of the
standard user's first install (head line `i`) already finds the product
registered, and in the per-machine `ALLUSERS=""` repair (head line `m`) it
finds the machine registration:

```text
BuilderGrant: the product is registered for the user
Property(S): BuilderGrantCommit = c:#i{04529B4C-5A16-54A8-917D-6501647BA7F7}|
...
BuilderGrant: the product is registered per machine
Property(S): ALLUSERS = 1
Property(S): BuilderGrantCommit = c:######m{7C09CC9A-6052-58BE-90EE-B7534D6CB666}
```
