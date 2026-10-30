# ENVVAR-02: WM_SETTINGCHANGE for Environment rows (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated,
  interactive session with Explorer, 2026-10-30
- CI runs 41039238183 (commit 8233970, with a parser bug) and 41049121338
  (commit 0c5a51f, broadcast and Explorer), PR #142, with the probe
  version of the test that logged instead of asserting. The asserting
  test passed in CI run 41051717271 (commit 9db6b3a), and its final
  version (shell found with `GetShellWindow`, a fresh unannounced value
  before each step) in CI run 41052974347 (commit 9647272)
- Test: `TestMsiexecEnvironmentBroadcast` in
  `internal/builder/nativetest/broadcast_windows_test.go`

A hidden top-level window on a thread of the test process logs every
`WM_SETTINGCHANGE`. Explorer's environment block is read through its PEB
(`ProcessParameters` at PEB+0x20, `Environment` at +0x80), which is what a
program started from the Start menu or Run inherits (inferred). Once
before the first package and before each step of the three variable
packages, `reg add HKCU\Environment` writes a fresh value to a variable
named `MSIBUILDER_UNANNOUNCED_<time>` without a broadcast (the probe runs
and the first asserting run wrote one value once). Each package installs `app.txt` and sets its
variable to `[INSTALLDIR]`; every step runs `msiexec /qn` and waits 5 s.

| Package | Step | Messages | Registry | Explorer sees the variable |
| --- | --- | --- | --- | --- |
| perMachine, `scope: machine` | install | one, `wParam=0`, `lParam="Environment"` | set | yes |
| | repair (`/fvamus`) | the same | set | yes |
| | uninstall | the same | removed | no |
| perMachine, `scope: user` | install, repair, uninstall | the same each time | set, set, removed | yes, yes, no |
| perUser, `scope: user` | install, repair, uninstall | the same each time | set, set, removed | yes, yes, no |
| perMachine, no `Environment` rows | install, repair, uninstall | none | — | — |

- In the probe runs, after the plain `reg add` and 5 s, the window had received nothing and
  Explorer did not have `MSIBUILDER_UNANNOUNCED`. After the first install
  it had it, so Explorer reloaded its whole environment because of the
  engine's broadcast.
- In the final run, the fresh value written before each install, repair
  and uninstall was in Explorer's block after that step, for all three
  variants.
- The first run read only the first entry of Explorer's block (a parser
  bug, fixed in the second run); its broadcast results were the same.
- A `SendMessageTimeoutW(HWND_BROADCAST, WM_SETTINGCHANGE, 0,
  "Environment", SMTO_ABORTIFHUNG, 5000)` from the test process (the
  window's own process), which is what a broadcast custom action sends,
  reached the window with the same parameters (run 41049121338). That
  probe step is not part of the final test.
- Not covered: programs other than Explorer that were already running
  (an open command prompt), a pending restart (Microsoft's page says no
  message is sent then), the per-user install by a standard user, Windows
  Server 2016.

Confirmed: the engine broadcasts by itself, so msi-builder needs no
broadcast action.
