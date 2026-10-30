# UPG-12: WiX Files glob upgraded by the converted package (CI)

- Runner: GitHub `windows-latest` (Windows Server 2025, x64), elevated, 2026-10-30
- CI run 41085630668, commit d3d9866
- WiX 5.0.2 (`dotnet tool install --global wix --version 5.0.2`), run as a
  black box
- Test: `TestMsiexecHarvestedComponentCodes` in
  `internal/builder/nativetest/harvest_windows_test.go`

v1 1.0.0 is built by `wix build -arch x64` from a source whose only payload
is `<Files Include="dist\**" Directory="INSTALLFOLDER" />`, with
`MajorUpgrade Schedule="afterInstallExecute"`. `dist` holds `app.exe`,
`readme.txt`, `docs\guide.md` and `docs\api\index.md`. v2 1.1.0 is the same
source with the new version, converted by `msi-builder convert` and built by
`msi-builder build`.

`convert --msi v1.msi` found all four files in the WiX package, one
component each, and wrote their codes into the folder source:

```yaml
components:
  - files:
      - source: dist/
        guids:
          app.exe: '{F60D2DDC-2553-5D4D-BA94-987744B5976C}'
          docs/api/index.md: '{8F112EF4-46C0-555A-9DEE-F60E89CD4724}'
          docs/guide.md: '{F824AE54-E7B7-5FE0-8879-9D17126A71D7}'
          readme.txt: '{CAF3105C-A653-52AA-9B6D-53925BC1345D}'
```

The paths below `INSTALLFOLDER` in the WiX package are the paths below
`dist`, so WiX installs the part of a `Files` path after the folder before
the first wildcard, subfolders included.

| v2 converted with | `app.exe` | `readme.txt` | `docs\guide.md` | `docs\api\index.md` |
| --- | --- | --- | --- | --- |
| `--msi` (WiX codes in `guids`) | kept | kept | kept | kept |
| no `--msi` (generated codes, `# TODO` on the glob) | gone | gone | gone | gone |

Every install ended with exit 0. With the WiX codes, the late removal of
the WiX package kept every file and the uninstall of v2 removed the folder;
with generated codes, the removal deleted every file v2 had just installed
at the same path, as in UPG-11. Confirmed.
