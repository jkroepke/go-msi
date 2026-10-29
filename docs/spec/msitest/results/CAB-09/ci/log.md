# CAB-09: a cabinet with 65535 entries (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), 2026-10-29
- CI run 40940932552, commit a9b7cef, step "go test (LZX at full size)"
- Test: `TestNativeFDIMaxFiles` in `internal/cab/native_windows_test.go`

`cab.Write` wrote a stored cabinet of 65535 one-byte files (`cab.MaxFiles`,
the count at which the builder starts a new cabinet). cabinet.dll's
`FDIIsCabinet` accepted it and `FDICopy` extracted all 65535 entries with
their names and bytes. `cab.Write` refuses 65536 files, so no cabinet with
an overflowing `cFiles` is written. FCI and smart-cabbed duplicates were not
run. Partial: the 65535-entry half is confirmed.
