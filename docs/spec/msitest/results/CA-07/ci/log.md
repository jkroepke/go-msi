# CA-07: firewall port lists and ranges (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), elevated, 2026-10-29
- CI run 40940932552, commit a9b7cef
- Test: `TestMsiexecFirewallPortLists` in
  `internal/builder/nativetest/audit_windows_test.go`

A rule named `MsiBuilderCA07`, protocol tcp, with `port` set to
`80,443,8000-8010` in the package, and to `[PORTS]` with
`PORTS=80,443,8000-8010` on the command line. After the install
`netsh advfirewall firewall show rule name=MsiBuilderCA07` reported:

| Variant | LocalPort | After uninstall |
| --- | --- | --- |
| in the package | `80,443,8000-8010` | no rule |
| from a property | `80,443,8000-8010` | no rule |

Confirmed.
