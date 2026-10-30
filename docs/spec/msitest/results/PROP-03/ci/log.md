# PROP-03: refusing Arm64 from an x64 package (CI)

- Runners: GitHub `windows-latest` (native suite) and `windows-11-arm`,
  elevated, 2026-10-30
- CI run 41142970055, PR #168 (head b84b0da), steps "go test (native)" and
  "go test (Windows on Arm)"
- Test: `TestMsiexecNativeArch` in
  `internal/builder/nativetest/arch_windows_test.go`

The x64 package searches `PROCESSOR_ARCHITECTURE` under
`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment` into
`NATIVE_ARCH`, sets `ENV_ARCH` to `[%PROCESSOR_ARCHITECTURE]` after
AppSearch in the execute sequence, and has the launch condition
`Installed OR ALLOW_ARM64 = "1" OR NATIVE_ARCH <> "ARM64"`. Server
properties of the install with `ALLOW_ARM64=1`:

| Property | x64 runner | Arm64 runner |
| --- | --- | --- |
| NATIVE_ARCH (registry search) | AMD64 | ARM64 |
| ENV_ARCH (`[%PROCESSOR_ARCHITECTURE]`) | AMD64 | ARM64 |
| Msix64 | 6 | 3401 |
| VersionNT64 | 603 | 603 |
| Intel | 6 | 15 |

Without `ALLOW_ARM64` the plain install exited 0 on x64 and 1603 on Arm64,
and the Arm64 verbose `/qn` log contained the launch condition's message
"This x64 package does not install on Arm64 Windows.".

`Msix64` and `Intel` hold processor values (the x64 runner of PROP-02 had 25),
so they are no architecture signal. The registry search is.
