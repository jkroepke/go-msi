# INST-05: Condition rows without NOT Installed (CI probe)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated, 2026-10-30
- CI run 41076138963 (commit fbeb95a), job `Build & Test (windows-latest, native)`, probe `TestMsiexecFeatureConditions`
- Rows as written: `ASSOC = 1` level 1 and `ASSOC <> 1` level 0 on `Assoc` (level 3); `1` level 0 and `1` level 2 on `Both` (level 1)

Each step: exit code, registration of v1/v2, payload files left in the install folder, then the server-side `Feature:` lines of the verbose log.

```text
INST-05 probe:
        install, no ASSOC: exit 0, v1 registered true, v2 registered false, files [app.exe]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        uninstall: exit 0, v1 registered false, v2 registered false, files []
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        install ASSOC=1: exit 0, v1 registered true, v2 registered false, files [app.exe assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Local;   Action: Local
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        repair /fa: exit 0, v1 registered true, v2 registered false, files [app.exe assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Reinstall;   Action: Reinstall
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        uninstall without ASSOC: exit 0, v1 registered false, v2 registered false, files [assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        install ASSOC=1 again: exit 0, v1 registered true, v2 registered false, files [app.exe assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Local;   Action: Local
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        uninstall ASSOC=0: exit 0, v1 registered false, v2 registered false, files [assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        install ASSOC=0 ADDLOCAL=ALL: exit 0, v1 registered true, v2 registered false, files [app.exe both.txt]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Local;   Action: Local
            Feature: [1],)
        uninstall: exit 0, v1 registered false, v2 registered false, files []
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Local;   Request: Absent;   Action: Absent
            Feature: [1],)
        install ASSOC=1 ADDLOCAL=Base: exit 0, v1 registered true, v2 registered false, files [app.exe]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        uninstall: exit 0, v1 registered false, v2 registered false, files []
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        install v1 ASSOC=1: exit 0, v1 registered true, v2 registered false, files [app.exe assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Local;   Action: Local
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        upgrade to v2 without ASSOC: exit 0, v1 registered false, v2 registered true, files [app.exe assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Absent;   Action: Absent
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
            Feature: [1],)
        uninstall v2: exit 0, v1 registered false, v2 registered false, files [assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        install v1 without ASSOC: exit 0, v1 registered true, v2 registered false, files [app.exe]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
        upgrade to v2 ASSOC=1: exit 0, v1 registered false, v2 registered true, files [app.exe assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Local;   Action: Local
            Feature: Base; Installed: Absent;   Request: Local;   Action: Local
            Feature: Both; Installed: Absent;   Request: Absent;   Action: Absent
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
            Feature: [1],)
        uninstall v2: exit 0, v1 registered false, v2 registered false, files [assoc.txt]
            Feature: Assoc; Installed: Absent;   Request: Null;   Action: Null
            Feature: Base; Installed: Local;   Request: Absent;   Action: Absent
            Feature: Both; Installed: Absent;   Request: Null;   Action: Null
            Feature: [1],)
```
