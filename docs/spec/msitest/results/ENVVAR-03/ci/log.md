# ENVVAR-03: context-conditioned variables and HKLM values (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated,
  with a throwaway standard user for the non-admin step, 2026-10-30
- CI runs 41054581404 (commit cfdb8db: installs, wizard, standard user,
  ICE) and 41055965846 (commit 01d4cd1: the same plus the repair with
  transitive components), PR #149
- Tests in `internal/builder/nativetest`: `TestMsiexecInstallScopes`,
  `TestFullUIInstallScope`, `TestICEInstallScope`
  (`installscope_windows_test.go`) and `TestNonAdminInstallScopes`
  (`nonadmin_windows_test.go`)

The package is the INST-02 scope package (`perUserOrMachine` and
`perMachineOrUser`) with three additions: a variable
`MSIBUILDER_SCOPE_TEST` with `action: append` and scope `auto`, which the
builder writes as `=-*MSIBUILDER_SCOPE_TEST` in a component conditioned
`ALLUSERS=1` (key path HKMU) and as `=-MSIBUILDER_SCOPE_TEST` in one
conditioned `NOT ALLUSERS` (key path HKCU); a component `MachineOnly` with
the condition `ALLUSERS=1` and an HKLM value; and a component `UserOnly`
with `NOT ALLUSERS` and an HKCU value. In the second run every component
is transitive.

| Step | Context | Variable | HKLM value | HKCU value |
| --- | --- | --- | --- | --- |
| silent install, each scope's default and the other one through `MSIINSTALLPERUSER` | per machine | machine environment only | set | absent |
| | per user | `HKCU\Environment` only | absent | set |
| wizard, default and switched on the scope page (x64, and x86 to all users) | as chosen | as above | as above | as above |
| repair `REINSTALL=ALL REINSTALLMODE=amus`, transitive components | as installed | unchanged | unchanged | unchanged |
| uninstall | both | removed | removed | removed |

The server's `ALLUSERS` in the repair log was `1` per machine and empty per
user, so the transitive conditions kept the same components.

As a standard user, `perUserOrMachine` installed per user with exit 0
without writing the HKLM value or the machine variable; `perMachineOrUser`
failed with 1925 as in INST-02.

ICE suite (`TestICEInstallScope`, no errors): `perUserOrMachine` got one
new warning, `ICE105 warning: msidbRegistryRootLocalMachine or
msidbRegistryRootUsers cannot be defined as root for per user install of
application. [Registry.Root]`, for the HKLM row of the conditioned
component. `perMachineOrUser` got no ICE105 finding, and neither package
got one for the `*` Environment row. ICE105 therefore does not read
component conditions, warns rather than fails for HKLM rows, and checks
only a package that starts per user (inferred from the two packages).
