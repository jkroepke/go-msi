# INST-02 non-admin

SERVER2025 (Windows Server 2025, 10.0.26100, msi.dll 5.0.26100.1742),
2026-10-29, JKROEPKE\msitest-user (Domain Users only, medium integrity),
started with `Start-Process -Credential -LoadUserProfile`;
`HKLM\SOFTWARE\Policies\Microsoft\Windows\Installer\DisableMsi=0` was set for
the run and deleted afterwards. Same packages (Word Count 2, bit 3 clear) and
script (`-NonAdmin`) as the admin run; condensed as there.

| Variant | Command line | Exit | Server | Result |
|---|---|---|---|---|
| user-default | perUserOrMachine `/qn` | 0 | `ALLUSERS` 2 deleted, `MSIINSTALLPERUSER=1`, `Privileged` unset | files, HKMU, HKCR, shortcut and Products key all per user; uninstall 0, clean |
| user-to-machine | perUserOrMachine `/qn MSIINSTALLPERUSER=""` | 1603 | `ALLUSERS` 2 → 1 | Error 1925 ("You do not have sufficient privileges to complete this installation for all users of the machine"); nothing installed |
| machine-default | perMachineOrUser `/qn` | 1603 | `ALLUSERS` 2 → 1 | Error 1925; nothing installed |
| machine-to-user | perMachineOrUser `/qn MSIINSTALLPERUSER=1` | 0 | `MSIINSTALLPERUSER=1`, `ALLUSERS` deleted | everything per user; uninstall 0, clean |

A standard user installs a dual package per user without elevation and
without Word Count bit 3. A silent per-machine install fails with 1925
instead of asking for elevation; whether the full UI asks for it on
Install was not tested (no one to answer the prompt).
