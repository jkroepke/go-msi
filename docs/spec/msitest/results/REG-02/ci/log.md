# REG-02 / ci
GitHub Actions `windows-latest` (Windows Server 2025 Datacenter 10.0.26100, image windows-2025-vs2026 20261021.278.1), elevated, 2026-10-29.
`TestMsiexecRemovesRegistry` (`pkg/builder/nativetest/registry_windows_test.go`) at commit 527238c, run 40863315162.
The test seeds the keys, installs, seeds them again, repairs with `/fomus`, seeds again and uninstalls.
```
install: every asserted removal passed (Old in both views, Only64 / Only32 in their own view only,
         Keep\Legacy with Keep\Other kept, HKMU Mu, HKCU User, Reset\Stale; Reset\Value = "new")
REG-02 repair /fomus: exit 0, Old(64) present=false, Old(32) present=false, Keep\Legacy present=false, User present=false, Reset\Stale present=false, Reset\Value="new"
REG-02 uninstall: exit 0, Old(64) present=true, Keep\Legacy present=true, Keep\Other="kept", Reset\Stale present=true, Reset\Value="<absent>"
--- PASS: TestMsiexecRemovesRegistry (2.81s)
```
