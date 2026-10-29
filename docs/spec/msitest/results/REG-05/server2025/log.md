# REG-05 on SERVER2025

`docs/spec/msitest/reg05/run.ps1` from commit ca4a0c3, run elevated on
2026-10-29 as `JKROEPKE\Administrator` in the RDP session (session 2,
Explorer running). Windows Server 2025, NT 10.0.26100.0, msi.dll
5.0.26100.1742. Every install and uninstall exited 0.

## Report

```text
probe install : exit 0; HKCR\.reg05p exists=True
  log: MSI (s) (68:64) [18:34:09:996]: Doing action: BuilderAssocChanged
  log: BuilderRun: SHChangeNotify(SHCNE_ASSOCCHANGED) in session 2
probe uninstall : exit 0; HKCR\.reg05p exists=True
  log: MSI (s) (68:C4) [18:34:13:628]: Doing action: BuilderAssocChanged
  log: BuilderRun: SHChangeNotify(SHCNE_ASSOCCHANGED) in session 2
baseline install : exit 0; HKCR\.reg05b exists=True
baseline uninstall : exit 0; HKCR\.reg05b exists=False
native install : exit 0; HKCR\.reg05n exists=True
  log: MSI (s) (68:2C) [18:34:24:244]: Doing action: UnregisterExtensionInfo
  log: MSI (s) (68:2C) [18:34:24:244]: Doing action: UnregisterProgIdInfo
  log: MSI (s) (68:2C) [18:34:24:255]: Doing action: RegisterExtensionInfo
  log: MSI (s) (68:2C) [18:34:24:258]: Doing action: RegisterProgIdInfo
native uninstall : exit 0; HKCR\.reg05n exists=False
  log: MSI (s) (68:00) [18:34:27:751]: Doing action: UnregisterExtensionInfo
  log: MSI (s) (68:00) [18:34:27:755]: Doing action: UnregisterProgIdInfo
  log: MSI (s) (68:00) [18:34:27:765]: Doing action: RegisterExtensionInfo
  log: MSI (s) (68:00) [18:34:27:765]: Doing action: RegisterProgIdInfo
```

## Listener

```text
18:34:06.317 listener registered id=1 session=2 explorer=True
18:34:06.524 --- control: SHChangeNotify(SHCNE_ASSOCCHANGED) from run.ps1
18:34:06.568 EVENT 0x08000000
18:34:09.611 --- probe install start
18:34:10.070 EVENT 0x08000000
18:34:13.162 --- probe install end (exit 0)
18:34:13.413 --- probe uninstall start
18:34:13.681 EVENT 0x08000000
18:34:16.765 --- probe uninstall end (exit 0)
18:34:16.967 --- baseline install start
18:34:20.289 --- baseline install end (exit 0)
18:34:20.497 --- baseline uninstall start
18:34:23.763 --- baseline uninstall end (exit 0)
18:34:24.022 --- native install start
18:34:24.798 EVENT 0x08000000
18:34:27.381 --- native install end (exit 0)
18:34:27.592 --- native uninstall start
18:34:28.313 EVENT 0x08000000
18:34:30.883 --- native uninstall end (exit 0)
18:34:32.363 listener stopped
```

## Reading

- probe (msi-builder `fileTypes`): one `SHCNE_ASSOCCHANGED` on install and
  one on uninstall, from `BuilderAssocChanged`, which ran in session 2, the
  session of the `msiexec` client. `.reg05p` stays after uninstall, as
  intended: its default lives in a permanent component (attributes 276 =
  64-bit + permanent + registry key path), and the uninstall log says
  "Disallowing uninstallation of component … since another client exists"
  (the permanent client). The run cleaned the key and the component
  registration up by hand afterwards.
- baseline (the same kind of keys as plain `Registry` rows, no action): no
  notification. The engine sends none for `Registry` rows.
- native (`ProgId`, `Extension` and `Verb` rows, built with WiX 5 as a
  black box from `ProgId Advertise="yes"`, installed locally with `/i`, no
  custom action): one notification on install and one on uninstall, while
  the engine's Register/UnregisterExtensionInfo and
  Register/UnregisterProgIdInfo actions ran (which of them sends it was not
  isolated); `.reg05n` is removed on uninstall.
- Not checked: whether Explorer's view changed, and whether a session
  without the `msiexec` client receives the notification.

The GitHub `windows-latest` runner (CI run 40919274640,
`TestMsiexecFileTypes`) logged `BuilderRun: SHChangeNotify(SHCNE_ASSOCCHANGED)
in session 2` in both the install and the uninstall log.
