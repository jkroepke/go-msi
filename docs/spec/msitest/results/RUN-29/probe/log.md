# RUN-29 / probe

Package built by go-msi from this `msi.yaml` (`package.longFileNames: false`):

```yaml
schema: go-msi/v1

product:
  name: Short Name Probe
  manufacturer: Example
  version: 1.0.0
  upgradeCode: 6b1d0c5e-2f7a-4c3e-9d41-8a5f3e2b7c10

package:
  platform: x64
  longFileNames: false

components:
  - files:
      - source: HELLO.EXE
```

Summary Word Count 3 (short names, compressed). Directory row
`INSTALLDIR ProgramFiles64Folder owyyokav|Short Name Probe`; `HELLO.EXE` in
the embedded cabinet.

```text
msiexec /i probe.msi /qn /l*v install.log -> exit 0
  PROPERTY CHANGE: Adding INSTALLDIR property. Its value is 'C:\Program Files\Short Name Probe\'.
  Dir (source): Key: INSTALLDIR, Object: ...\.sfn-probe\, LongSubPath: PFiles64\Short Name Probe\, ShortSubPath: PFiles64\owyyokav\
  SetTargetFolder(Folder=C:\Program Files\Short Name Probe\)
  SetSourceFolder(Folder=1\PFiles64\owyyokav\|PFiles64\Short Name Probe\)
  created C:\Program Files\Short Name Probe\HELLO.EXE (2048 bytes)
msiexec /x probe.msi /qn /l*v uninstall.log -> exit 0
  neither C:\Program Files\Short Name Probe nor C:\Program Files\owyyokav left
msiexec /a probe.msi /qn TARGETDIR=C:\sfn-admin /l*v admin.log -> exit 0
  created C:\sfn-admin\probe.msi (32768 bytes, Word Count 4)
  created C:\sfn-admin\PFiles64\Short Name Probe\HELLO.EXE
msiexec /i probe.msi /qn SHORTFILENAMES=1 /l*v install-sfn.log -> exit 0
  INSTALLDIR = C:\Program Files\owyyokav\
  SetTargetFolder(Folder=C:\Program Files\owyyokav\)
  created C:\Program Files\owyyokav\HELLO.EXE
msiexec /x probe.msi /qn /l*v uninstall-sfn.log -> exit 0
  both folders gone; Win32_Product query for "Short Name Probe" empty
RESULT probe: target and admin image use the long name, SHORTFILENAMES=1 installs to the short name; no long-name package run for comparison
```
