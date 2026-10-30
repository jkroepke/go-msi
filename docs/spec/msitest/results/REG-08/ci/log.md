# REG-08 / ci
GitHub Actions `windows-latest` (Windows Server 2025, image windows-2025-vs2026 20261021.278.1), elevated, 2026-10-30.
The block below is condensed from the test's one-line-per-key `t.Logf` output (`present=true|false`); "with Tree\Sub" and the ICE result come from the passing assertions.
`TestMsiexecRemovesKeysOnUninstall` (`internal/builder/nativetest/registry_uninstall_windows_test.go`) at commit 8d71e07, run 41064566042 (PR #157).
The package has `Registry` rows named `-` with a Null value in a 64-bit component (`Tree`, `Existing`, HKCR `Directory\shell\MsiBuilderTestVerb`, and `Both`, which also has a `RemoveRegistry` row) and in a 32-bit component (`Only32`). `Existing` (with a subkey) and `Both` exist before the install; before each later step the test writes `Tree\Sub` (64-bit view), `Tree` (32-bit view), the verb's `command` subkey, `Only32` in both views and `Both`, as the program would at run time.
```
REG-08 install:       Tree(64) false, Tree(32) false, Existing true, Both false, Verb false, Only32(32) false, Only32(64) false
REG-08 repair /fomus: Tree(64) true,  Tree(32) true,  Existing true, Both false, Verb true,  Only32(32) true,  Only32(64) true
REG-08 early upgrade (1.0.1, removeOld afterInstallInitialize):
                      Tree(64) false, Tree(32) true,  Existing false, Both false, Verb false, Only32(32) false, Only32(64) true
REG-08 late upgrade (1.0.2, removeOld afterInstallExecute):
                      Tree(64) true,  Tree(32) true,  Existing false, Both false, Verb true,  Only32(32) true,  Only32(64) true
REG-08 uninstall /x:  Tree(64) false (with Tree\Sub), Tree(32) true, Existing false, Both false, Verb false, Only32(32) false, Only32(64) true
--- PASS: TestMsiexecRemovesKeysOnUninstall (4.66s)
```
`TestICERemoveOnUninstall` passed in the same run with two warnings (ICE61 is the usual warning for `Upgrade` rows that include the package's own version):
```
registry_uninstall_windows_test.go:186: ICE33 warning: Reg key regCF5621F4E47C4D5869C6723AA10CC9A1 is used in an unsupported way. Shell extension verbs info should be registered via the Verb table. This entry may overwrite a value created through that table. [Registry.Registry regCF5621F4E47C4D5869C6723AA10CC9A1]
registry_uninstall_windows_test.go:186: ICE61 warning: This product should remove only older versions of itself. The Maximum version is not less than the current product. (1.0.0 1.0.0) [Upgrade.Attributes {0B7E3A95-6C2D-4F18-A4E9-71D5C3B80F26}//1.0.0//513]
--- PASS: TestICERemoveOnUninstall (1.42s)
```
