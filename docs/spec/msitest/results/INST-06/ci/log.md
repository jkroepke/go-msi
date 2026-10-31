# INST-06: TestMsiexecFeatureConditions (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated, 2026-10-31
- CI run 41230502028 (commit e9d728c), job `Build & Test (windows-latest, native)`, PR #177: test passed; run 41230441589 (commit b565ec0) passed with the same steps
- Extra is `default: disabled` (level 0) with the row `EXTRA = 1` (level 1), written as `(Installed AND (REMOVE OR REINSTALL)) OR (EXTRA = 1)`; the other features keep `NOT Installed AND (…)` (INST-05)
- Before `repair /fa without EXTRA`, the test deleted extra.txt; it was back afterwards
- `v1/v2 registered` is `MsiQueryProductState` = `INSTALLSTATE_DEFAULT` for each product code after the step
- An upgrade step logs the new product's feature states first, then the old product's removal

```text
INST-05/INST-06 install EXTRA=1: exit 0, files [app.exe extra.txt], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Local;   Action: Local
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 repair /fa without EXTRA: exit 0, files [app.exe extra.txt], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Reinstall;   Action: Reinstall
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Reinstall;   Action: Reinstall
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 uninstall without EXTRA: exit 0, files [], v1/v2 registered [false false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 install EXTRA=1: exit 0, files [app.exe extra.txt], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Local;   Action: Local
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 remove with REMOVE=Extra: exit 0, files [app.exe], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Null;   Action: Null
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 uninstall: exit 0, files [], v1/v2 registered [false false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 install: exit 0, files [app.exe], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 repair /fa: exit 0, files [app.exe], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Reinstall;   Action: Reinstall
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 add with ADDLOCAL=Extra: exit 0, files [app.exe], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Null;   Action: Null
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 add with EXTRA=1 ADDLOCAL=Extra: exit 0, files [app.exe extra.txt], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Null;   Action: Null
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Local;   Action: Local
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 uninstall: exit 0, files [], v1/v2 registered [false false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 install EXTRA=1: exit 0, files [app.exe extra.txt], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Local;   Action: Local
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 add with ASSOC=1 ADDLOCAL=Assoc: exit 0, files [app.exe assoc.txt extra.txt], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Local;   Action: Local
        Base; Installed: Local;   Request: Null;   Action: Null
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 uninstall: exit 0, files [], v1/v2 registered [false false]
        Assoc; Installed: Local;   Request: Absent;   Action: Absent
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 install v1 EXTRA=1: exit 0, files [app.exe extra.txt], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Local;   Action: Local
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 upgrade to v2 without EXTRA: exit 0, files [app.exe], v1/v2 registered [false true]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Absent;   Action: Absent
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Absent;   Action: Absent
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 uninstall v2: exit 0, files [], v1/v2 registered [false false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 install v1: exit 0, files [app.exe], v1/v2 registered [true false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 upgrade to v2 EXTRA=1: exit 0, files [app.exe extra.txt], v1/v2 registered [false true]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Absent;   Request: Local;   Action: Local
        Both; Installed: Absent;   Request: Absent;   Action: Absent
        Extra; Installed: Absent;   Request: Local;   Action: Local
        Soft; Installed: Absent;   Request: Absent;   Action: Absent
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Absent;   Request: Null;   Action: Null
        Soft; Installed: Absent;   Request: Null;   Action: Null
INST-05/INST-06 uninstall v2: exit 0, files [], v1/v2 registered [false false]
        Assoc; Installed: Absent;   Request: Null;   Action: Null
        Base; Installed: Local;   Request: Absent;   Action: Absent
        Both; Installed: Absent;   Request: Null;   Action: Null
        Extra; Installed: Local;   Request: Absent;   Action: Absent
        Soft; Installed: Absent;   Request: Null;   Action: Null
--- PASS: TestMsiexecFeatureConditions (17.74s)
```
