# FILE-04: the engine consumes MsiFileHash (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025), elevated, 2026-10-29
- CI run 40940932552, commit a9b7cef
- Test: `TestMsiexecUsesMsiFileHash` in
  `internal/builder/nativetest/audit_windows_test.go`

Before each install the test writes `plain.txt` with the package's bytes to
the install folder and sets its creation, access and modification times to
2020-01-01, so that the engine sees an unmodified unversioned file. The
package is msi-builder's, edited with `msidb` for the two variants. The
file-copy decision from the verbose log:

| Variant | Log line | Copied |
| --- | --- | --- |
| builder's `MsiFileHash` row | `Won't Overwrite; Won't patch; Existing file is unversioned and unmodified - hash matches source file` | no |
| `HashPart1` changed | `Overwrite; Won't patch; Existing file is unversioned and unmodified - hash doesn't match source file` | yes |
| same rows in a table named `FileHash` | `Overwrite; Won't patch; Existing file is unversioned and unmodified - no source file hash provided to compare` | yes |

The engine reads `MsiFileHash` during file costing and ignores a table
named `FileHash`. The earlier repair runs (`msifilehash/log.md`) could not
tell the variants apart because a repair of a corrupted file copies it in
any case. Confirmed.
