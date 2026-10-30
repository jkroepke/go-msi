# ENVVAR-02: WM_SETTINGCHANGE for Environment rows (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated,
  interactive session with Explorer, 2026-10-30
- CI runs 41039238183 (broadcast only) and 41049121338 (broadcast and
  Explorer), PR #142
- Test: `TestMsiexecEnvironmentBroadcast` in
  `internal/builder/nativetest/broadcast_windows_test.go`

A hidden top-level window on a thread of the test process logs every
`WM_SETTINGCHANGE`. Explorer's environment block is read through its PEB
(`ProcessParameters` at PEB+0x20, `Environment` at +0x80), which is what a
program started from the Start menu or Run inherits. Before the packages
run, `reg add HKCU\Environment /v MSIBUILDER_UNANNOUNCED /d 1` writes a
variable without a broadcast. Each package installs `app.txt` and sets its
variable to `[INSTALLDIR]`; every step runs `msiexec /qn` and waits 5 s.

| Package | Step | Messages | Registry | Explorer sees the variable |
| --- | --- | --- | --- | --- |
| perMachine, `scope: machine` | install | one, `wParam=0`, `lParam="Environment"` | set | yes |
| | repair (`/fvamus`) | the same | set | yes |
| | uninstall | the same | removed | no |
| perMachine, `scope: user` | install, repair, uninstall | the same each time | set, set, removed | yes, yes, no |
| perUser, `scope: user` | install, repair, uninstall | the same each time | set, set, removed | yes, yes, no |
| perMachine, no `Environment` rows | install, repair, uninstall | none | — | — |

- After the plain `reg add` and 5 s, the window had received nothing and
  Explorer did not have `MSIBUILDER_UNANNOUNCED`. After the first install
  it had it, so Explorer reloaded its whole environment because of the
  engine's broadcast.
- The first run read only the first entry of Explorer's block (a parser
  bug, fixed in the second run); its broadcast results were the same.
- A `SendMessageTimeoutW(HWND_BROADCAST, WM_SETTINGCHANGE, 0,
  "Environment", SMTO_ABORTIFHUNG, 5000)` from the test process, which is
  what a broadcast custom action sends, reached the window with the same
  parameters (run 41049121338).
- Not covered: a pending restart (Microsoft's page says no message is sent
  then), the per-user install by a standard user, Windows Server 2016.

Confirmed: the engine broadcasts by itself, so msi-builder needs no
broadcast action.
