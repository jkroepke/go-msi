# SEARCH-02: version search on an OS file (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, build 26100), elevated, 2026-10-29
- CI run 40940932552, commit a9b7cef
- Test: `TestMsiexecSearchesOSFileVersion` in
  `internal/builder/nativetest/audit_windows_test.go`

The test copies `C:\Windows\System32\ucrtbase.dll` (file version
10.0.26100.33648, read with PowerShell's `FileVersionRaw`) to
`C:\MsiBuilderSearch02` and installs a package whose file searches look for
it in `[System64Folder]` and in the copy's folder. msi-builder writes
`minVersion` lowered by one in its last field (MSISPEC section 8.7), so
`minVersion: 10.0.26100.33648` means "this version or later".

| Property | Search | Result |
| --- | --- | --- |
| SYS_PLAIN | `[System64Folder]`, no bounds | `C:\Windows\system32\ucrtbase.dll` |
| SYS_EQUAL | `[System64Folder]`, `minVersion` 10.0.26100.33648 | not found |
| SYS_ABOVE | `[System64Folder]`, `minVersion` 10.0.26100.33649 | not found |
| SYS_LOW | `[System64Folder]`, `minVersion` 6.0.0.0 | `C:\Windows\system32\ucrtbase.dll` |
| PRIV_EQUAL | private copy, `minVersion` 10.0.26100.33648 | `C:\MsiBuilderSearch02\ucrtbase.dll` |
| PRIV_ABOVE | private copy, `minVersion` 10.0.26100.33649 | not found |

The same bytes are found at their real version in a private folder and only
below it in the system folder, consistent with the reported compatibility
shim for the system's own files; the version the engine saw was not
measured. Confirmed.
