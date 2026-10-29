# SEARCH-05: searches through msi-builder's YAML (CI)

- Runner: GitHub `windows-latest`, elevated, 2026-10-29
- CI run 40912856615, commit a0f7b4b
- Test: `TestMsiexecSearchesFiles` and `TestICELocators` in
  `internal/builder/nativetest/locator_windows_test.go`

`TestMsiexecSearchesFiles` installs a base package whose component the
component searches look for, then the search package with `L_CMD=cmd` on the
command line, and compares each property in the verbose log. All asserted
values matched; `TestICELocators` reported no ICE error.

| Property | Search | Result |
| --- | --- | --- |
| D_ABS | folder, full path | `C:\MsiBuilderLocator\` |
| D_PROP | `[WindowsFolder]System32` | `C:\Windows\System32\` |
| D_PF64 | `[ProgramFiles64Folder]MsiBuilderLocatorBase` | the base product's folder |
| D_UNDER_REG | `sub` under a registry folder | `C:\MsiBuilderLocator\sub\` |
| D_DEPTH | `…\b` with depth 3 | `C:\MsiBuilderLocator\deep\a\b\` |
| D_DEPTH_MISS | `…\b` with depth 1 | not found (`none`) |
| D_MISSING | missing folder | not found |
| F_ABS | `app.exe` in a full path | found |
| F_DEPTH / F_DEPTH_MISS | `app.exe` one level down, depth 1 / 0 | found / not found |
| F_LONG | `long file name.txt`, written as a generated `short\|long` pair | `C:\MsiBuilderLocator\long file name.txt` |
| F_LONG_EXT | `settings.json` (four-letter extension), `short\|long` pair | `C:\MsiBuilderLocator\settings.json` |
| F_LONG_MISS | `long file other.txt`, missing, `short\|long` pair | not found |
| F_MIN_EQUAL | `minVersion` 1.2.3.4 = file version, no languages (written 1.2.3.3) | found |
| F_MIN_EQUAL_1033 / F_MIN_EQUAL_1031 | same with languages 1033 / 1031 (written as given) | found / not found |
| F_MIN_ABOVE | `minVersion` 1.2.3.5 | not found |
| F_MAX_EQUAL / F_MAX_BELOW | `maxVersion` 1.2.3.4 / 1.2.3.3 | found / not found |
| F_SIZE / F_SIZE_BELOW | exact size / one byte less as maximum | found / not found |
| F_DATE | 2000-01-01 to 2043-12-31 | found |
| F_DATE_TODAY | `maxDate` today (file written minutes before) | found |
| F_DATE_FUTURE | `minDate` 2043-01-01 | not found |
| F_UNVERSIONED | `plain.txt`, `minVersion` 0.0.0.1 (written 0.0.0.0) | not found |
| F_UNDER_REG | `app.exe` in `sub` under a registry folder | found |
| F_UNDER_COMPONENT | `base.exe` under the installed component | found |
| C_FOLDER / C_MISSING | installed / unknown component | key file's folder / not found |
| L_FIRST_HIT | missing, `sub`, root | `C:\MsiBuilderLocator\sub\` (first hit wins) |
| L_NONE | two misses | the default `none` |
| L_CMD | two hits, `L_CMD=cmd` on the command line | `cmd` |

`OS_PLAIN`, `OS_WINDOWS` and `OS_MIN` (kernel32.dll searches) are only logged
with `go test -v`; this run did not record them.
