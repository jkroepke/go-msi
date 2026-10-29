# UI-08: the wizard and a remembered install folder

- Runner: GitHub `windows-latest`, elevated, interactive session, 2026-10-29
- CI run 40966065131, commit 99dddc0 (directory `RegLocator` search, all
  three variants passed); first seen in CI run 40962873234, commit fd57ff1,
  with the raw search
- Test: `TestFullUIRememberInstallDir` in
  `internal/builder/nativetest/remember_windows_test.go`

Version 1 is installed silently with `INSTALLDIR=C:\MsiBuilderRemember\custom\`;
version 2 upgrades through the full UI, the driver pressing Next, Install
and Finish.

| Variant | Exit | Install folder page shows | v2 installs to |
| --- | --- | --- | --- |
| `ui: { enabled: true }` (welcome only) | 0 | (no page) | custom folder |
| `chooseDirectory: true`, perMachine | 0 | `C:\MsiBuilderRemember\custom\` | custom folder |
| `chooseDirectory: true`, perMachineOrUser (scope page, all users kept) | 0 | `C:\MsiBuilderRemember\custom\` | custom folder |

The PathEdit's text is among the window's names (in fd57ff1 the test
looked for a ValuePattern value instead and failed on its own check):

```text
WINDOW: C:\MsiBuilderRemember\custom\ | Change... | < Back | Next > | Cancel | BannerBmp | Click Next to install to the default folder or click Change to choose another. | Install MsiBuilderRemember to: | Destination Folder
```

Client log of the scope variant (run 40962873234): the UI sequence restores the folder,
`SetInstallScopeMachine_INSTALLDIR_Remembered` puts it into the machine
folder property the scope page applies, and the client passes it on:

```text
MSI (c) (0C:50) [22:10:48:209]: Doing action: RestoreInstallDirMachine
MSI (c) (0C:50) [22:10:48:209]: PROPERTY CHANGE: Adding INSTALLDIR property. Its value is 'C:\MsiBuilderRemember\custom\'.
MSI (c) (0C:50) [22:10:48:223]: PROPERTY CHANGE: Modifying InstallScopeMachine_INSTALLDIR property. Its current value is 'C:\Program Files\MsiBuilderRemember\'. Its new value: 'C:\MsiBuilderRemember\custom\'.
MSI (c) (0C:50) [22:10:53:840]: Switching to server: INSTALLDIR="C:\MsiBuilderRemember\custom\" INSTALLDIR_REMEMBERED_MACHINE="C:\MsiBuilderRemember\custom\" ...
MSI (s) (C0:44) [22:10:53:961]: Skipping action: RestoreInstallDirMachine (condition is false)
```

In the server, `RestoreInstallDir*` is skipped because `INSTALLDIR` already
came from the client.
