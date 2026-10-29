# SEARCH-04 / ci
GitHub Actions `windows-latest` (Windows Server 2025 Datacenter 10.0.26100, image windows-2025-vs2026 20261021.278.1), elevated, 2026-10-29.
`TestMsiexecSearchesRegistry` (`pkg/builder/nativetest/search_windows_test.go`) at commit 527238c, run 40863315162.
Values written with `reg.exe add` under `HKLM\SOFTWARE\MsiBuilderTest\Search` (64-bit view unless noted):
`Str` "hello", `Dword` REG_DWORD 394802, `Expand` REG_EXPAND_SZ `%SystemRoot%\x`, `Multi` REG_MULTI_SZ `a\0b`,
the key's default "keydefault", `Brackets` "a[ProductName]b{c}", `Str32` "from32" (32-bit view only),
`Dir` = `%SystemRoot%`, `DirFile` = a file's path, `DirMissing` = `C:\MsiBuilderNoSuchFolder`.
Install: `msiexec /i search.msi P_CMD=cmd P_CMDDEFAULT=cmd /qn /l*v`; values read from the `Property(S):` lines.
`C_*` are type-51 actions before `CostInitialize` with the conditions `P_DWORD >= 394802` (C_NUMBER),
`P_DWORD >= "#394802"` (C_STRING) and `P_DWORD >= "#1000000"` (C_LONGER).
```
SEARCH-04 P_STR = "hello"
SEARCH-04 P_DWORD = "#394802"
SEARCH-04 P_EXPAND = "C:\\Windows\\x"
SEARCH-04 P_MULTI = "[~]a[~]b[~]"
SEARCH-04 P_KEYDEFAULT = "keydefault"
SEARCH-04 P_BRACKETS = "a[ProductName]b{c}"
SEARCH-04 P_32 = "from32"
SEARCH-04 P_64MISS = "none"
SEARCH-04 P_DEFAULT = "fallback [x] {y} {{z}}"
SEARCH-04 P_NODEFAULT = "<unset>"
SEARCH-04 P_CMD = "cmd"
SEARCH-04 P_CMDDEFAULT = "cmd"
SEARCH-04 P_DIR = "C:\\Windows\\"
SEARCH-04 P_DIRFILE = "none"
SEARCH-04 P_DIRMISSING = "none"
SEARCH-04 C_NUMBER = "<unset>"
SEARCH-04 C_STRING = "1"
SEARCH-04 C_LONGER = "1"
--- PASS: TestMsiexecSearchesRegistry (1.17s)
```
(The values are Go `%q` strings, so `\\` is one backslash.)
