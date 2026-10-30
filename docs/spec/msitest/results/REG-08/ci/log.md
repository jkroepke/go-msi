# REG-08 / ci
GitHub Actions `windows-latest` (Windows Server 2025, image windows-2025-vs2026 20261021.278.1), elevated, 2026-10-30.
`TestMsiexecRemovesKeysOnUninstall` (`internal/builder/nativetest/registry_uninstall_windows_test.go`) at commit 8d71e07, run 41064566042 (PR #157).
The package has `Registry` rows named `-` with a Null value in a 64-bit component (`Tree`, `Existing`, HKCR `Directory\shell\MsiBuilderTestVerb`, and `Both`, which also has a `RemoveRegistry` row) and in a 32-bit component (`Only32`). `Existing` (with a subkey) and `Both` exist before the install; before each later step the test writes `Tree\Sub` (64-bit view), `Tree` (32-bit view), the verb's `command` subkey, `Only32` in both views and `Both`, as the program would at run time.
```
REG-08 install:       Tree(64) false, Tree(32) false, Existing true, Both false, Verb false, Only32(32) false, Only32(64) false
REG-08 repair /fomus: Tree(64) true,  Tree(32) true,  Existing true, Both false, Verb true,  Only32(32) true,  Only32(64) true
REG-08 early upgrade (1.0.1, removeOld afterInstallInitialize):
                      Tree(64) false, Tree(32) true,  Existing false, Both false, Verb false, Only32(32) false, Only32(64) true
REG-08 late upgrade (1.0.2, removeOld afterInstallExecute):
                      Tree(64) true,  Tree(32) true,  Existing false, Both false, Verb true,  Only32(32) true,  Only32(64) true
REG-08 uninstall /x:  Tree(64) false (with Tree\Sub), Tree(32) true, Both false, Verb false, Only32(32) false, Only32(64) true
--- PASS: TestMsiexecRemovesKeysOnUninstall (4.66s)
```
`TestICERemoveOnUninstall` passed in the same run; ICE33 warns that the HKCR `shell` row should use the `Verb` table, and ICE61 about `Upgrade` rows that include the package's own version (the usual warning of every package).
