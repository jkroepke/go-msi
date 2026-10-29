# INST-02 admin

SERVER2025 (Windows Server 2025, 10.0.26100, msi.dll 5.0.26100.1742),
2026-10-29, elevated JKROEPKE\Administrator, branch commit 2c926f2,
`docs/spec/msitest/inst02/run.ps1`. Condensed from the script's report: the
`Error 4: SELECT ...` lookup lines (the probe has no Error table) are left
out. `files`, `HKMU`, `HKCR`, `shortcut` and `registration` say where each
resource was found: in the user's profile and HKCU (user), in Program Files,
HKLM and the all-users Start menu (machine). Every uninstall returned 0 and
left nothing, except where noted.

| Variant | Package and command line | Exit | Server context (log) | Files | HKMU | HKCR | Shortcut | Products key |
|---|---|---|---|---|---|---|---|---|
| user-default | perUserOrMachine `/qn` | 0 | `ALLUSERS` 2 deleted, `MSIINSTALLPERUSER=1`, `ProgramFiles64Folder=%LOCALAPPDATA%\Programs\`, `Privileged=1` | user | user | user | user | user |
| user-to-machine | perUserOrMachine `/qn MSIINSTALLPERUSER=""` | 0 | `MSIINSTALLPERUSER` deleted, `ALLUSERS` 2 → 1 | machine | machine (+ HKCU, see below) | machine | machine | machine |
| user-allusers1 | perUserOrMachine `/qn ALLUSERS=1` | 0 | `ALLUSERS` 2 → 1, then `MSIINSTALLPERUSER` deleted | machine | machine (+ HKCU) | machine | machine | machine |
| machine-default | perMachineOrUser `/qn` | 0 | `ALLUSERS` 2 → 1 | machine | machine (+ HKCU) | machine | machine | machine |
| machine-to-user | perMachineOrUser `/qn MSIINSTALLPERUSER=1` | 0 | `MSIINSTALLPERUSER=1` added, `ALLUSERS` 2 deleted | user | user | user | user | user |

"+ HKCU" is the HKCU value the builder gives the shortcut's component as its
key path (ICE38), which a per-machine install writes for the installing
user too. In every per-user and per-machine install the
`HKLM\...\Uninstall\{ProductCode}` key existed as well.

## Changing the context in the client after CostFinalize (`/qr`)

Type-51 actions in the UI sequence after CostFinalize set `ALLUSERS` and
`MSIINSTALLPERUSER` the way a scope page would. `/qr` runs the UI sequence
(the client logged `Doing action: ProbeBefore`).

| Variant | Client after the switch | Server | Files | HKMU | HKCR | Shortcut | Products key |
|---|---|---|---|---|---|---|---|
| qr-user (no switch) | context per user | per user | user | user | user | user | user |
| qr-user-switch-pu: `ALLUSERS=2`, `MSIINSTALLPERUSER` cleared | `ProgramFiles64Folder` and `INSTALLDIR` stay `%LOCALAPPDATA%\Programs\...` | `ALLUSERS` 2 → 1, `ProgramFiles64Folder=C:\Program Files\`, but `INSTALLDIR=%LOCALAPPDATA%\Programs\ScopeProbe\` from the client | **user** | machine | machine | machine | machine |
| qr-user-switch-au1: `ALLUSERS=1` | as above | as above | **user** | machine | machine | machine | machine |
| qr-machine-switch: perMachineOrUser, `ALLUSERS=2`, `MSIINSTALLPERUSER=1` | `ProgramFiles64Folder` and `INSTALLDIR` stay `C:\Program Files\...` | `ALLUSERS` deleted, `ProgramFiles64Folder=%LOCALAPPDATA%\Programs\`, `INSTALLDIR=C:\Program Files\ScopeProbe\` from the client | **machine** | user | user | user | user |

The client fixes its context and folder properties when the package opens;
setting `ALLUSERS`/`MSIINSTALLPERUSER` later changes neither. The server
picks the context from the values it receives and resolves the system
folders and private directories for it, but a public directory property
the client resolved (`INSTALLDIR`) arrives set and is taken as it is, so the
files land in the other context's folder.

## Major upgrade

- upgrade-same: perUserOrMachine 1.0.0 per user, then 2.0.0 per user: 0;
  1.0.0 was removed by the upgrade (its uninstall afterwards: 1605).
- upgrade-cross: 1.0.0 per user, then 2.0.0 `MSIINSTALLPERUSER=""`: 0;
  1.0.0 stayed registered per user next to 2.0.0 per machine, files in both
  folders. Both uninstalls returned 0, but the user's HKCU
  `Software\MsiSpecProbe\ScopeProbe` value and the user Start menu shortcut
  stayed (the two products share component GUIDs across contexts); they
  were removed by hand.
