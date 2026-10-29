# FILE-08 / ci
GitHub Actions `windows-latest`, elevated, 2026-10-29.
`TestMsiexecUpgradesFolder` (`internal/builder/nativetest/folder_windows_test.go`) at commit e31aa93, run 40880910505, job 123473088266 ("Build & Test (windows-latest)", step "go test": pass).
An earlier form of the same test, with the folder listed under a top-level `harvest` key, passed at commit 067f3e9 (run 40876771056); the generated tables are the same.

Package: `components: [{ files: [{ source: dist/, exclude: '*.pdb' }] }]`, x64, install dir `Program Files\MsiBuilderFolder`.

| Path below dist | v1 (1.0.0) | v2 (2.0.0) |
| --- | --- | --- |
| `app.exe` | version 1.0.0.0 | version 2.0.0.0 |
| `app.pdb` | excluded | absent |
| `readme.txt` | "readme first" | "readme second" |
| `Notes.txt` / `NOTES.txt` | "notes first" as `Notes.txt` | "notes second" as `NOTES.txt` |
| `docs/old.txt` | present | dropped, with its folder |
| `keep/same.txt` | "same" | "same" |
| `lib/new.txt` | absent | new folder and file |

Run twice, with `upgrade.removeOld` `afterInstallInitialize` and `afterInstallExecute`:
`msiexec /i v1 /qn /l*v`, `msiexec /i v2 /qn /l*v` (major upgrade), `msiexec /x v2 /qn`.

Assertions, all passed for both values of `removeOld`:

- Before installing: `app.exe`, `readme.txt`, `notes.txt` and `same.txt` have the same `ComponentId` in v1 and v2.
- After v1: every v1 file holds its v1 content and `app.pdb` is absent.
- After the upgrade (exit code 0): `app.exe`, `readme.txt`, `notes.txt` (found case-insensitively), `lib/new.txt` and `keep/same.txt` hold their v2 content. `docs/old.txt` and the `docs` folder are gone.
- After the uninstall (exit code 0): the install dir is gone.

The step runs `go test` without `-v`, so the per-file `t.Logf` lines are not in the job log; a failing assertion would have failed the step.
