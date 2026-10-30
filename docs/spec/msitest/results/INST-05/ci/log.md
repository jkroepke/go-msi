# INST-05: TestMsiexecFeatureConditions (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated, 2026-10-30
- CI run 41083409657 (commit 3353df3), job `Build & Test (windows-latest, native)`, PR #162
- Rows written as `NOT Installed AND (<condition>)`; features Assoc (level 3; `ASSOC = 1` on, `ASSOC <> 1` disabled), Both (level 1; `1` disabled, `1` level 2) and Soft (level 3; `SOFT = 1` on)

In this run `repair /fa` returned 1622 because its log name held a slash (a test bug, fixed after the run), and the upgrade of a v1 without Soft to v2 with `SOFT=1` did not install Soft, against the test's expectation; the test now expects that.

```text
INST-05 install: exit 0, files [app.exe]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 uninstall: exit 0, files []
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 install ASSOC=1: exit 0, files [app.exe assoc.txt]
        Assoc; Installed: Absent;   Request: Local;   Action: Local
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 repair /fa: exit 1622, files [app.exe assoc.txt]
repair /fa: msiexec returned 1622
INST-05 uninstall without ASSOC: exit 0, files []
        Assoc; Installed: Local;   Request: Absent;   Action: Absent
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 install: exit 0, files [app.exe]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 add with ADDLOCAL=Assoc: exit 0, files [app.exe assoc.txt]
        Assoc; Installed: Absent;   Request: Local;   Action: Local
        Base; Installed: Local;   Request: Null;   Action: Null
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 uninstall: exit 0, files []
        Assoc; Installed: Local;   Request: Absent;   Action: Absent
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 install ASSOC=0 ADDLOCAL=Assoc: exit 0, files []
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Null;   Action: Null
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 install ASSOC=1 ADDLOCAL=ALL: exit 0, files [app.exe assoc.txt both.txt soft.txt]
        Assoc; Installed: Absent;   Request: Local;   Action: Local
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Local;   Action: Local
        Soft; Installed: Absent;   Request: Local;   Action: Local
INST-05 uninstall: exit 0, files []
        Assoc; Installed: Local;   Request: Absent;   Action: Absent
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Local;   Request: Absent;   Action: Absent
        Soft; Installed: Local;   Request: Absent;   Action: Absent
INST-05 install v1 ASSOC=1 SOFT=1: exit 0, files [app.exe assoc.txt soft.txt]
        Assoc; Installed: Absent;   Request: Local;   Action: Local
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Local;   Action: Local
INST-05 upgrade to v2: exit 0, files [app.exe soft.txt]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Local;   Action: Local
        Assoc; Installed: Local;   Request: Absent;   Action: Absent
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Local;   Request: Absent;   Action: Absent
INST-05 uninstall v2: exit 0, files []
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Local;   Request: Absent;   Action: Absent
INST-05 install v1: exit 0, files [app.exe]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05 upgrade to v2 ASSOC=1 SOFT=1: exit 0, files [app.exe assoc.txt]
        Assoc; Installed: Absent;   Request: Local;   Action: Local
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Absent;   Action: Absent
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
upgrade to v2 ASSOC=1 SOFT=1: files [app.exe assoc.txt], want [app.exe assoc.txt soft.txt]
INST-05 uninstall v2: exit 0, files []
        Assoc; Installed: Local;   Request: Absent;   Action: Absent
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
```
