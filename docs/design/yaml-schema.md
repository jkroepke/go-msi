# go-msi YAML schema, version 1

This document defines the input format of go-msi: one YAML file that describes
an MSI package. The `config` package implements it (types, loading, defaults
and validation). The MSI writer builds on that package and never reads YAML
itself.

The design is driven by two sources:

- the requirements for a generic builder, with windows_exporter as the
  reference package ([`examples/windows_exporter/msi.yaml`](../../examples/windows_exporter/msi.yaml)
  is the first acceptance test), and
- the Windows Installer specification in [`docs/spec/MSISPEC.md`](../spec/MSISPEC.md)
  and its Windows test results in [`docs/spec/MSISPEC-RESULTS.md`](../spec/MSISPEC-RESULTS.md).

The Windows runs (57 cases on Windows Server 2025, `msi.dll` 5.0.26100.1742)
contradicted eight claims of the specification and found several ways to
build a package that installs with exit code 0 and is still broken. Section 2
lists every finding and the schema rule it produced. The rest of the document
is the schema itself.

## 1. Principles

1. **Say what, not how.** Authors describe directories, files, services,
   registry values and actions. Table rows, attribute bits, `#`/`[~]` value
   encodings, sequence numbers and custom action type numbers are the
   builder's job. Every typed field exists because a raw encoding was easy to
   get wrong.
2. **Make silent failures unrepresentable.** Where Windows accepts a package
   and then misbehaves (an executable without a version, a removal that
   `*` does not imply, a script action whose return code is ignored), the
   schema has no way to write it, or validation fails the build.
3. **Validate against the engine, not the folklore.** Limits come from the
   Windows runs where they exist (31 columns, Page Count 500 at most), and
   from the specification otherwise.
4. **Reproducible by default.** The same file, the same inputs and the same
   `SOURCE_DATE_EPOCH` give a byte-identical MSI. Nothing is random unless the
   author asks for it.
5. **Nothing package-specific is built-in.** windows_exporter's firewall rule,
   service recovery and event log source are generic helpers that any package
   can use.
6. **Strict input.** Unknown keys are errors, so a typo never silently drops a
   setting. Every error names the YAML path (`components[0].files[1].source`).
7. **Short by default.** A simple package is a product, a platform and its
   files. The install folder, the feature, component IDs and the upgrade
   policy have defaults (section 3.3), and the explicit forms stay for the
   packages that need them.

## 2. What the Windows runs changed

Each row is a result from `MSISPEC-RESULTS.md` and the rule it put into the
schema or the validator. "Writer" marks rules for the MSI writer that have no
schema surface; they are listed so that the writer's design starts from them.

### 2.1 Contradicted claims

| Finding | Result on Windows | Schema rule |
|---|---|---|
| 1. Column limit (DB-07) | The 32nd column is rejected (2280, 2216); the limit is 31 | `tables[].columns` allows at most 31 entries. |
| 2. Page Count (SUM-03) | x64 installs from 150 up; above 500 fails with 1613 on every platform; 0 and 1 fail | `package.installerVersion` maps to Page Count. It accepts 2.0 to 5.0 for x64, 5.0 only for Arm64 and 1.0 to 5.0 for x86, and rejects anything above 5.0. x64 keeps 2.0 as its floor because ICE80 was not run (section 4.2). |
| 3. String refcounts (DB-05) | `msi.dll` keeps refcounts in 15 bits and drops a string whose count is a multiple of 32768 | Writer: every refcount stays in 1 … 32767. A string referenced more often is a build error until the writer can split it; no realistic package gets there, so the schema has no knob. |
| 4. `*` and removal (RUN-08) | `=*NAME` (machine scope, no `-`) survives uninstall | `environment[].scope` and `environment[].removeOnUninstall` are separate fields. Removal defaults to `true` and the builder always writes `-` explicitly; scope never implies it. |
| 5. RemoveExistingProducts window (RUN-17) | 1399 (before InstallValidate) works like 1401; only 5802 gives 2613 | `upgrade.removeOld` names one of four windows. The validator checks the numbered result against the windows as the engine accepts them, not against a fixed number. |
| 6. Native CFB version (CFB-01) | `msi.dll` writes v4 even for small databases; v3 and v4 behave the same | Writer: emits v4 (4096-byte sectors) by default. No schema field. |
| 7. Per-user rollback (RUN-13) | A deferred action failing after `PublishProduct` rolled back completely | The "deferred action after RegisterProduct in a per-user package" check is a warning, not an error. It stays a warning because the reported leftover may need a non-administrator user (RUN-09 was not run). |
| 8. SystemIdentifier (DB-14) | The engine ignores it; the native writer stores `0x0002000A` | Writer: writes `0x0002000A` like the native writer. No schema field. |

### 2.2 Silent failures

These packages install with exit code 0 and are wrong afterwards. The schema
closes each one.

| Case | What happens | Schema rule |
|---|---|---|
| Executable with an empty `File.Version` (RUN-02) | On a major upgrade with early removal the new, unversioned file is disallowed against the old versioned one, then the old product is removed: the executable is **deleted** | `files[].version` defaults to `auto` (read from the PE version resource). A PE file without a version resource is a build error. `version: none` must be written explicitly and is only allowed for non-PE files, or with `allowUnversionedExecutable: true`, which also makes the validator warn about the upgrade. An empty string is never accepted. |
| Corrupted cabinet data (CAB-02, CAB-04) | Wrong DEFLATE history, or a bit flip without a checksum, installs a corrupted file | Writer: every cabinet data block gets a checksum, and the builder re-extracts every cabinet after writing it and compares each file's SHA-256 with the source. Neither can be turned off, so the schema has no field for them. |
| Script custom actions (RUN-18) | Inline JScript/VBScript actions (types 37/38) report success whatever they return; a Binary script returning 5 ends the install with 0 and leaves files unregistered | There are no script custom actions. Actions run an executable (`exec`), a DLL (`dll`), set a property or directory, or fail with an error. |
| EXE exit codes (RUN-18) | Any non-zero exit fails the install unless the action ignores it | `customActions[].return` is required for `exec` actions (`check` or `ignore`), so the author decides. |
| Same ProductCode, new package (RUN-11) | Installing a rebuilt package over the installed one fails with 1638 | `product.productCode: auto` derives the code from the upgrade code, version, platform and a digest of every input. Changed content gives a new ProductCode (a major upgrade); unchanged content gives the same package byte for byte. A pinned ProductCode makes the validator warn. |
| Versions that differ in the fourth field only (RUN-03) | Without an inclusive maximum both products stay installed side by side | `upgrade.allowSameVersion` defaults to `true` (maximum inclusive), so `0.32.0.5` replaces `0.32.0.4`. Four-field versions get a warning that Windows compares only three. |

### 2.3 Settled facts with a schema rule

| Fact | Source | Schema rule |
|---|---|---|
| Template platform tokens are exact and case-sensitive; `X64` and `arm64` give 1633 | SUM-02 | `package.platform` takes `x86`, `x64` or `arm64` in any case, plus the aliases `intel`, `amd64` and `aarch64`, and the builder writes the exact token. |
| Two languages in the Template give 1624 | SUM-02 | `product.language` is one integer LCID, not a list. |
| Summary code page 65001 or 1200 gives 1620; a 65001 database works | SUM-04, DB-08 | `package.codepage` sets the database code page (1252 by default, 65001 allowed). The summary code page is the database code page when that is an ANSI page, else 1252. |
| Stream names are limited to 62 characters | DB-11 | `icons` keys are at most 57 characters and `binaries` keys at most 55 (`Icon.` and `Binary.` count). The embedded cabinet name is at most 62 characters. |
| The minimum integer of a column type is stored as NULL | DB-02 | `tables[].rows` reject −32768 in `i2` columns and −2147483648 in `i4` columns. |
| `MsiGetFileHash` returns zeros for an empty file | RUN-12 | Writer: an empty unversioned file gets a hash row of four zeros. |
| A service is deleted at uninstall only with `ServiceControl` event 128 | RUN-05 | `services[].control.deleteOnUninstall` defaults to `true`. |
| `MsiServiceConfigFailureActions` with restart actions fails with 1939; `sc.exe failure` works | RUN-05 | `services[].recovery` is implemented with a deferred `sc.exe` action, never with that table. |
| A launch-condition row and a type-19 action both block a downgrade with 1603 | RUN-04 | `upgrade.downgrade: error` uses a launch condition, which needs no custom action. |
| The root and directory tree of the compound file must be ordered | CFB-04, CFB-06 | Writer: a correctly ordered red-black-compatible tree; colours are free. |

## 3. File layout

```yaml
schema: go-msi/v1        # required, selects this document
vars: {}                 # defaults for ${NAME} substitution
product: {}              # identity
package: {}              # platform, scope, compression, summary information
upgrade: {}              # major upgrade policy (on by default)
installDir: {}           # the product's folder (default: Program Files\<name>)
directories: []          # the install tree, for folders the shorthands do not cover
features: []             # the feature tree
components: []           # files, registry, services, folders, environment
properties: {}           # Property table, Secure and Hidden flags
launchConditions: []
customActions: []
icons: {}
binaries: {}
firewall: []             # helper: firewall rules
ui: {}                   # built-in wizard and added dialogs
tables: []               # escape hatch: custom tables
```

### 3.1 Variables

`${NAME}` anywhere in a string value is replaced before parsing the rest of
the file. Values come, in order of precedence, from the command line
(`--var NAME=value`), from the environment when the CLI is started with
`--env`, and from `vars`. An undefined variable is an error; `$${` writes a
literal `${`. Windows Installer's own `[Property]` syntax is left alone, so
`[INSTALLDIR]` and `${VERSION}` never collide. Inside a flow mapping
(`{ … }`) a value that starts with `${` must be quoted, as YAML reads `{` as
the start of a mapping.

```yaml
vars:
  ARCH: x64
product:
  version: ${VERSION}
package:
  platform: ${ARCH}
```

### 3.2 Editor support

[`pkg/config/schema.json`](../../pkg/config/schema.json) is a JSON Schema of this
format. Editors that use the YAML language server pick it up from a comment
in the first line:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jkroepke/go-msi/main/pkg/config/schema.json
schema: go-msi/v1
```

The JSON Schema checks keys, types, enums and syntax such as GUIDs, and
rejects the nil GUID as an upgrade code. Editors see the file before
variable substitution, so every typed value also accepts a string with a
`${NAME}` variable. References between sections, lengths that depend on
other values and the rules of section 2 are checked only by
`config.Validate`, which stays the authority. A test keeps the JSON Schema
in step with the Go types.

### 3.3 Defaults and shorthands

The smallest package needs no directories, features or upgrade section:

```yaml
schema: go-msi/v1
product:
  name: Hello
  manufacturer: Example
  version: 1.0.0
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
package:
  platform: x64
components:
  - files:
      - source: hello.exe
```

The loader expands the short forms into the explicit model before
validation, so the builder and every rule below see the same thing either
way:

| Short form | Expands to |
|---|---|
| no `installDir` | `INSTALLDIR`, configurable, named after the product, under `ProgramFiles64Folder` (`ProgramFilesFolder` on x86, `LocalAppDataFolder\Programs` for `perUser`). Not added when `directories` already defines that ID (section 4.4). |
| component without `directory` | the install dir |
| `directory: bin` or `directory: CommonAppDataFolder/Hello/logs` | directory nodes for the path, with generated IDs such as `INSTALLDIR_bin` (section 4.4) |
| component without `id` | an ID from the key file's name (`hello.exe`), else the first service name, else the directory ID |
| no `features` | one feature `MainFeature`, titled with the product name, holding every component, with the install dir as its configurable directory |
| a component written inside `features[].components` | the same component in `components`, referenced by ID |
| `features[].components` entry that is only an ID | a reference to that component; in Go, `config.ComponentRef(id)` |
| no `upgrade` | the standard policy of section 4.3, as `upgrade: {}`; `upgrade: false` writes no Upgrade rows |
| `features[].default: on`, `off` or `disabled` | `level: 1`, `2` or `0` (section 4.5) |
| `features[].firewall` | the rules in `firewall`, following an empty component `<feature>_Firewall` that the feature holds (section 4.11) |
| a service argument with a `condition` | a private property set by an immediate `setProperty` action before InstallServices under that condition (section 4.6) |

Generated IDs are stable: the same file gives the same IDs. Component GUIDs
do not depend on component IDs (section 4.6), so renaming or dropping an ID
never changes a GUID.

## 4. Sections

### 4.1 `product`

| Key | Type | Default | Notes |
|---|---|---|---|
| `name` | string | required | `ProductName`, at most 63 characters |
| `manufacturer` | string | required | `Manufacturer` and summary Author |
| `version` | string | required | `major.minor.build[.revision]`, at most 255.255.65535; a fourth field warns (section 2.2) |
| `language` | int | `1033` | One LCID; 0 is neutral |
| `upgradeCode` | GUID | required | Never changes for a product; the nil GUID is rejected |
| `productCode` | GUID or `auto` | `auto` | Section 2.2 |
| `packageCode` | GUID or `auto` | `auto` | Digest of the finished database |
| `description` | string | `<name> <version> installer` | Summary Subject |
| `comments` | string | `This installer database contains the logic and data required to install <name>.` | Summary Comments |
| `arp` | map | | `ARP*` properties: `comments`, `contact`, `helpLink`, `aboutLink`, `updateLink`, `icon` (an `icons` key), `noRepair`, `noModify`, `noRemove`, `size` (KB), `installLocation` (a directory ID; schedules `ARPINSTALLLOCATION` after CostFinalize) |

GUIDs are accepted with or without braces and in any case. The builder writes
them braced and upper case.

### 4.2 `package`

| Key | Type | Default | Notes |
|---|---|---|---|
| `platform` | enum | required | `x86`, `x64`, `arm64` (aliases `intel`, `amd64`, `aarch64`), any case |
| `scope` | enum | `perMachine` | `perMachine` (`ALLUSERS=1`) or `perUser` (no `ALLUSERS`) |
| `installerVersion` | string | `"5.0"` | Page Count = major × 100 + minor. x86: 1.0 to 5.0, x64: 2.0 to 5.0, arm64: 5.0 |
| `codepage` | int | `1252` | Database code page, section 2.3 |
| `compression` | enum | `mszip` | `mszip` or `none` (stored blocks) |
| `cabinet` | string | `<name>.cab` | Embedded cabinet stream name, at most 62 characters |
| `longFileNames` | bool | `true` | `false` sets Word Count bit 1 and requires 8.3 names |
| `summary` | map | | `title` (`Installation Database`), `keywords` (`Installer`), `author` (manufacturer) |

The x64 floor stays at 2.0 although the engine accepted 1.5 (SUM-03): ICE80
was not part of the run, and the requirements ask for ICE-clean output.
Above 5.0 every platform fails with 1613 on Windows Installer 5.0, the
current engine, so it is rejected.

`perUser` packages are checked for ICE105: no services, no HKLM registry
values, no no-impersonate actions and no per-machine directories.

### 4.3 `upgrade`

```yaml
upgrade:
  allowSameVersion: true          # maximum inclusive (default true)
  downgrade: error                # error | allow | ignore
  downgradeMessage: A later version of [ProductName] is already installed. Setup will now exit.
  removeOld: afterInstallInitialize
  migrateFeatures: true
  property: OLDERVERSIONBEINGUPGRADED
  newerProperty: NEWERVERSIONDETECTED
  related:                        # extra Upgrade rows, e.g. a renamed predecessor
    - upgradeCode: 11111111-2222-3333-4444-555555555555
      minVersion: 0.0.0
      maxVersion: 1.0.0
      includeMax: false
      onlyDetect: false
      property: OLDPRODUCTFOUND
```

`upgrade` absent, `upgrade: true` and `upgrade: {}` all mean the standard
policy, and `upgrade: false` means no Upgrade rows at all. The builder
writes the two standard rows: one removes versions up to this one
(inclusive when `allowSameVersion`), and one detects newer versions.

`downgrade: error` adds a launch condition on `newerProperty`; `ignore`
detects newer versions but installs side by side; `allow` drops the
detection row.

`removeOld` windows (section 9.2 of the specification, widened by RUN-17):

| Value | Placed | Behaviour |
|---|---|---|
| `afterInstallValidate` | right after InstallValidate | old product removed first, outside the transaction |
| `afterInstallInitialize` (default) | right after InstallInitialize | removed first, inside the transaction, so it rolls back |
| `afterInstallExecute` | right after InstallExecute | new first, then old, one transaction; strict component rules |
| `afterInstallFinalize` | after InstallFinalize | separate transaction; a failed removal leaves both |

Every property named here must be public, is added to
`SecureCustomProperties` and must not appear in `properties`.

### 4.4 `installDir` and `directories`

Most packages only need `installDir`, the product's folder:

```yaml
installDir: windows_exporter          # the folder name alone
installDir:
  id: APPLICATIONFOLDER               # Directory key and property, default INSTALLDIR
  name: Example/windows_exporter      # "/" nests folders, default product.name
  parent: ProgramFiles64Folder        # a standard folder, default by platform and scope
```

The generated install dir is configurable, so `msiexec /i … INSTALLDIR=D:\App`
and the wizard's Browse button can move it, and its ID must be upper case.
When `directories` defines the install dir's ID, that node is used as
written, including its `configurable` setting. A package with a directory
tree and no `installDir` key gets the install dir only when a component
lands in it, so an explicit tree never gains an unused folder.

A per-user install dir lies in the user profile. Windows Installer wants an
HKCU registry value as the key path of such components (ICE38) and
RemoveFolder rows for their folders (ICE64); go-msi does not write either
yet, and the validator warns about such components.

A component's `directory` is a directory ID, a standard folder, or a folder
path. A path starts at its first segment when that is a directory ID or a
standard folder, and in the install dir otherwise; `./` forces the install
dir. A single upper-case word is always an ID, and so is a word that
matches a known ID or standard folder in another case; the same holds for
the first segment of a path, so typos such as
`INSTALDIR` or `ProgramFiles64folder` fail validation instead of creating a
folder. Folders a path
names are added below their parent with IDs from the parent ID and the name
(`INSTALLDIR_textfile_inputs`); a folder that already exists is reused.

`directories` is the explicit tree, for directory IDs that something else
refers to or short names that must be pinned. Every node has an `id` (the
Directory key and the property that holds its path) and a `name`.

```yaml
directories:
  - id: ProgramFiles64Folder
    children:
      - id: APPLICATIONFOLDER
        name: windows_exporter
        configurable: true       # public + Secure, can be set on the command line and in the UI
        children:
          - id: TEXTFILEDIR
            name: textfile_inputs
```

- Top-level nodes must be standard folder properties (`ProgramFilesFolder`,
  `ProgramFiles64Folder`, `CommonAppDataFolder`, `System64Folder`, …) or
  `TARGETDIR`. The builder adds `TARGETDIR` with `SourceDir` itself.
- `name` may be `short|long`. Without a short part the builder generates one
  from a hash of the parent key and the long name (specification section 11.3).
- A 64-bit package that installs a component under `ProgramFilesFolder`,
  `CommonFilesFolder` or `SystemFolder` gets a warning (ICE80): those are the
  32-bit folders on 64-bit Windows.
- `configurable: true` requires an upper-case ID.

### 4.5 `features`

`features` is optional; without it every component goes into one feature
(section 3.3). A feature lists components by ID, or defines them in place,
which is the shortest form for a component that only one feature uses:

```yaml
features:
  - id: DefaultFeature
    title: windows_exporter ${VERSION}
    description: The binary and configuration files for windows_exporter
    default: on                # on | off | disabled
    display: expanded          # expanded | collapsed | hidden
    directory: APPLICATIONFOLDER
    allowAbsent: false
    allowAdvertise: false
    components:
      - Executable                 # defined in components
      - files:                     # defined here
          - source: README.md
    features:
      - id: FirewallException
        title: Firewall Exception
        default: off
        firewall:                  # section 4.11
          - name: windows_exporter
            program: '[#windows_exporter.exe]'
            protocol: tcp
            port: '[LISTEN_PORT]'
```

- Feature IDs are at most 38 characters; depth is at most 16.
- `default` says whether the feature is installed: `on` (the default) is
  `level: 1`, `off` is `level: 2`, which a user turns on in the feature tree
  or with `ADDLOCAL`, and `disabled` is `level: 0`. A higher `INSTALLLEVEL`
  also installs `off` features. Set `default` or `level`, not both; `level`
  is for packages that use more than these three levels.
- `level: 0` disables a feature; `ADDLOCAL=ALL` cannot turn it on.
- Every component is in at least one feature (ICE21). A component used by
  two features is defined once in `components` and named by ID in both.
- `allowAdvertise` defaults to `false`; advertised installs are not in schema v1 (section 6).

### 4.6 `components`

A component is one unit that Windows Installer installs, repairs and removes
together. Rules from section 8.4 of the specification apply: one versioned
key file, resources never move between components, and GUIDs are stable.

```yaml
components:
  - id: Executable                 # default: the key file's name
    directory: APPLICATIONFOLDER   # default: the install dir
    guid: 5546455E-B5EA-5AF8-A21A-AA526F04D083   # or auto (default) or none
    transitive: true
    files:
      - source: dist/windows_exporter.exe
        checksum: true
    registry: []
    services: []
    eventLog: []
    environment: []
    createFolder: false
    condition: ""
    permanent: false
    neverOverwrite: false
    sharedDllRefCount: false
    keyPath: windows_exporter.exe   # file name, registry id or "directory"
    bitness: auto                    # auto (from platform) | 32 | 64
```

**GUIDs.** `auto` derives a UUIDv5 from the upgrade code, the bitness and
the key path's canonical location (directory path and filename, or
registry root, key and value name). A changed key path therefore gets a new
GUID, as the component rules require. A pinned GUID is kept as it is (the
windows_exporter executable component keeps the GUID of its released
packages). `none` writes a NULL ComponentId and is only accepted with
`permanent: true`, because such a component is never removed (specification
section 8.4).

**Key path.** Default: the first file, else the first registry value, else
the directory, which gets a `CreateFolder` row (ICE18). A component with a
service must have the service executable as key file.

**Files.**

| Key | Default | Notes |
|---|---|---|
| `source` | required | Path relative to the YAML file |
| `name` | base name of `source` | `short|long` allowed |
| `id` | the name, if it is a legal identifier and unique, else generated | File key |
| `version` | `auto` | `auto`, a four-field version, the ID of another file (companion), or `none`; never empty (section 2.2) |
| `language` | `auto` | From the PE resource; `none` for fonts |
| `allowUnversionedExecutable` | `false` | Required to build a PE file with `version: none` or without a version resource |
| `vital` | `true` | |
| `checksum` | `false` | Verifies the PE checksum at install |
| `readOnly`, `hidden`, `system` | `false` | |

Unversioned files get an `MsiFileHash` row automatically.

**Permissions.** A file, registry value, service or the component's folder
can carry a `permissions` block (`folderPermissions` on the component, which
then needs a `CreateFolder` row: `createFolder: true` or a directory key
path). It becomes one `MsiLockPermissionsEx` row; the authored DACL replaces
the inherited one (specification section 6.2, SEC-01):

```yaml
files:
  - source: windows_exporter.exe
    permissions:
      sddl: 'D:PAI(A;;FA;;;BA)(A;;FRFX;;;<BUILTIN\Users>)'
      condition: LOCKDOWN=1      # optional; a false condition keeps the inherited ACL
```

The SDDL is checked at build time after each `<DOMAIN\account>` form, which
the engine resolves (including `[%ENV]` inside it), is replaced by a
placeholder SID. `[Property]` references are not expanded by the engine and
are rejected. A deny ACE gets a warning, because SEC-01 saw a deny for
`BUILTIN\Users` lock the installer out of its own file on uninstall. A
`LockPermissions` custom table cannot coexist with it (ICE104, error 1941).

**Registry values.**

```yaml
registry:
  - id: EventMessageFile          # optional; generated from the content otherwise
    root: HKLM                    # HKLM | HKCU | HKCR | HKU | HKMU (per-user or per-machine)
    key: SYSTEM\CurrentControlSet\Services\EventLog\Application\windows_exporter
    name: EventMessageFile        # empty: the default value
    type: expandString            # string | expandString | multiString | dword | binary | key
    value: '%SystemRoot%\System32\EventCreate.exe'
```

The builder encodes the value: `dword` writes `#n`, `binary` writes `#x…`,
`expandString` writes `#%…`, `multiString` takes a list and joins it with
`[~]`, and a `string` that starts with `#` is escaped as `##`. `type: key`
creates the key (`+`) or, with `removeOnUninstall: true`, creates and removes
it (`*`). `append: true` and `prepend: true` on a `multiString` merge with an
existing value.

**Services** (only in a component whose key file is the service executable):

```yaml
services:
  - name: windows_exporter
    displayName: windows_exporter
    description: Exports Prometheus metrics about the system
    type: ownProcess             # ownProcess | shareProcess
    start: auto                  # auto | demand | disabled
    delayedAutoStart: true       # MsiServiceConfig ConfigType 3
    errorControl: normal         # ignore | normal | critical
    vital: true                  # ErrorControl | 0x8000
    account: '[SERVICE_ACCOUNT]'
    password: '[SERVICE_PASSWORD]'
    arguments:                   # one string, or pieces
      - --log.format=json
      - value: --collectors.enabled [ENABLED_COLLECTORS]
        condition: ENABLED_COLLECTORS
    dependencies: [wmiApSrv]
    failureActionsOnNonCrashFailures: true   # MsiServiceConfig ConfigType 4
    control:
      startOnInstall: true
      stopOnInstall: true
      stopOnUninstall: true
      deleteOnUninstall: true
      wait: true
    recovery:
      resetAfterDays: 0
      actions: [restart, restart, restart]
      restartDelay: 60s
```

`arguments` is the service command line, as one formatted string or as a
list of pieces joined with spaces. A piece with a `condition` is only part of
the command line when the condition is true: it becomes a private property
`Arg_<service>_<n>`, set by an immediate `setProperty` action
`SetArg_<service>_<n>` before InstallServices, and the command line names
the property. A property that is not set formats to an empty string, so the
command line has no placeholder left over. Conditions are checked like
those of section 4.9.

`recovery` becomes a deferred, no-impersonate `sc.exe failure` action after
InstallServices, with its exit code ignored (RUN-05). A `password` is added
to `MsiHiddenProperties` when it names a property.

**Event log sources** become Registry rows of the component:

```yaml
eventLog:
  - source: windows_exporter
    log: Application
    messageFile: '%SystemRoot%\System32\EventCreate.exe'
    typesSupported: 7
```

**Environment variables** (section 2.1, finding 4):

```yaml
environment:
  - name: PATH
    value: '[APPLICATIONFOLDER]'
    action: append               # set | setIfAbsent | append | prepend | remove
    scope: machine               # machine | user
    removeOnUninstall: true      # default true (false for remove); written as an explicit "-"
```

`append` and `prepend` build the `[~];` form, and a `PATH`-like value without
them gets a warning, since `set` replaces the whole variable.

**Top-level `files` shorthand.** Not in version 1. Every file belongs to an
explicit component, so that its GUID and key path are visible in review.

### 4.7 `properties`

```yaml
properties:
  LISTEN_PORT: "9182"                  # short form: value only
  ENABLED_COLLECTORS: {}               # declared, no default value
  SERVICE_PASSWORD: { hidden: true }
  MSIRESTARTMANAGERCONTROL: Disable
  myPrivate: { value: x }
```

| Key | Default | Notes |
|---|---|---|
| `value` | none | Written to the Property table; an empty value writes no row |
| `secure` | `true` for public names | Listed in `SecureCustomProperties` |
| `hidden` | `false` | Listed in `MsiHiddenProperties` |

Public names have no lower-case letter. `secure: true` on a private name is
an error. The request properties (`ADDLOCAL`, `REMOVE`, `REINSTALL`, …) and
the properties the builder owns (`ProductCode`, `ProductVersion`, `ALLUSERS`,
`SecureCustomProperties`, …) cannot be set here (ICE87).

### 4.8 `launchConditions`

```yaml
launchConditions:
  - condition: Installed OR VersionNT64
    message: This package requires 64-bit Windows.
```

Conditions are parsed with the grammar of specification section 8.8 at build
time, because a launch condition that does not parse is ignored by the
engine.

### 4.9 `customActions`

```yaml
customActions:
  - id: CheckExtraFlags
    error: The parameter '--config.file' must not be included in EXTRA_FLAGS.
    schedule:
      - sequence: execute
        before: InstallInitialize
        condition: EXTRA_FLAGS AND (EXTRA_FLAGS >< "--config.file")

  - id: SetCollectorsFlag
    setProperty: { name: CollectorsFlag, value: '--collectors.enabled [ENABLED_COLLECTORS]' }
    schedule:
      - { sequence: execute, after: InstallFiles, condition: ENABLED_COLLECTORS }

  - id: CreateConfigFile
    exec:
      command: '"[%ComSpec]" /c TYPE NUL >>"[ConfigFile_Default]"'
      directory: APPLICATIONFOLDER   # working directory, default TARGETDIR
    execute: deferred                # immediate | deferred | rollback | commit
    impersonate: false
    return: check                    # check | ignore | asyncWait | asyncNoWait
    schedule:
      - { sequence: execute, before: InstallServices, condition: ConfigFile_Default }
```

Kinds, exactly one per action:

| Key | Type | Notes |
|---|---|---|
| `setProperty` | 51 | Never deferred |
| `setDirectory` | 35 | After CostFinalize |
| `error` | 19 | Text or an `Error` table number |
| `exec` | 34 | `command` is a formatted command line; deferred commands are formatted when the script is written, so no CustomActionData is needed |
| `execProperty` | 50 | `property` holds the executable, `arguments` the rest |
| `dll` | 1 | `binary` (a `binaries` key) and `entry` |

Options: `execute` (default `immediate`), `impersonate` (default `true`;
`false` needs a deferred action), `return` (required for `exec` and
`execProperty`, default `check` otherwise), `hideTarget`, `once`
(`firstSequence`, `oncePerProcess`, `clientRepeat`).

**Schedule.** A list of placements, one per sequence table: `execute`, `ui`,
`adminExecute`, `adminUI`. Each has exactly one of `before`, `after` or
`at` (an absolute number) and an optional `condition`. Actions that share an
anchor keep their order in the file: the builder expands every chain
next to its anchor (specification section 11.7) and fails, rather than
shifts, when a chain runs into another fixed number. After numbering it
checks deferred actions against InstallInitialize … InstallFinalize (ICE77)
and RemoveExistingProducts against the windows of section 4.3.

Standard actions are added automatically for the tables a package uses. An
action used only as an anchor is added at its default number.

### 4.10 `icons` and `binaries`

```yaml
icons:
  icon.ico: installer/icon.ico
binaries:
  helper.dll: build/helper.dll
```

Keys are stream names, limited by the 62-character stream limit (section
2.3). Binaries are for `dll` custom actions and for the UI's bitmaps.

### 4.11 `firewall` helper

```yaml
firewall:
  - name: windows_exporter
    description: windows_exporter HTTP endpoint
    component: FirewallException      # the rule follows this component's state
    program: '[#windows_exporter.exe]'
    protocol: tcp
    port: '[LISTEN_PORT]'
    remoteAddresses: '[REMOTE_ADDR]'  # empty means any
    profiles: all                     # all | domain | private | public, or a list
```

The builder writes deferred, no-impersonate `netsh advfirewall` actions: on
install and reinstall it deletes by name and program, then adds, with a
rollback that deletes; on removal it deletes, with a rollback that adds. The
`component` is usually an empty component with its own GUID, in the feature
that switches the rule.

A rule that a feature switches can be written under that feature instead,
without `component`:

```yaml
features:
  - id: FirewallException
    title: Firewall Exception
    default: off
    firewall:
      - name: windows_exporter
        program: '[#windows_exporter.exe]'
        protocol: tcp
        port: '[LISTEN_PORT]'
```

The loader adds an empty component `FirewallException_Firewall` to the
feature and points the rules at it. The component goes in the feature's
`directory`, else in the install dir, else in the first component's
directory when an explicit tree has no install dir, so it adds no folder. Its GUID comes from the
upgrade code, the bitness and the feature ID, so it does not collide with
other components keyed on the install dir. Errors name the rule's place
under the feature, such as `features[1].firewall[0].program`.

### 4.12 `ui`

```yaml
ui:
  wizard: featureTree               # none | minimal | featureTree
  dialogs:
    - id: ConfigDlg
      title: windows_exporter configuration
      description: Configuration of the windows_exporter service
      after: CustomizeDlg            # inserted into the wizard's Next/Back flow
      fields:
        - property: ENABLED_COLLECTORS
          label: Comma-separated list of collectors to use. Use '[\[]defaults[\]]' for the defaults.
        - property: LISTEN_PORT
          label: Port to listen on
```

The wizards are written from scratch for go-msi, including the bitmaps,
which the `artwork` package draws from a colour palette at build time.
`minimal` has welcome, ready, progress and finish; `featureTree` adds the
feature tree (`CustomizeDlg`) with Browse and disk usage. Both include
maintenance (Repair and Remove; Change with the feature tree), cancel, user
exit, fatal error, files in use, Restart Manager and the error dialog. Added
dialogs are laid out automatically: one label and one edit field per
property (at most four per dialog), Back, Next and Cancel. `after` names the
page the dialog follows: `WelcomeDlg`, `CustomizeDlg` or an added dialog.
`none` writes no UI tables; silent installs never depend on the UI in any
mode. Without feature selection (`none`, `minimal`) `ARPNOMODIFY` is set.

### 4.13 `tables`

An escape hatch for tables the schema does not model.

```yaml
tables:
  - name: MyTable
    columns:
      - { name: Key, type: s72, key: true }
      - { name: Value, type: I4 }
    rows:
      - [first, 1]
```

At most 31 columns (DB-07), a name of at most 31 characters, integers inside
the column type's range without its minimum (DB-02). Standard tables that the
builder writes cannot be redefined here.

## 5. Validation

`config.Validate` returns every problem at once, each with its YAML path and
a severity. Errors fail the build; warnings are printed. The checks that need
only the configuration live in the `config` package:

- required fields, enums, GUID and version syntax, identifier syntax and
  lengths;
- references: feature components, component directories, key paths, icon and
  binary keys, firewall components, dialog properties;
- the rules of section 2 that are visible in the configuration.

Checks that need the inputs or the numbered tables run in the builder: PE
versions, sequence numbering and windows, condition parsing, ICE rules over
the generated tables, and the cabinet round trip.

## 6. Not in version 1

Multiple languages in one package, advertised installs, script custom
actions, and a top-level file harvester. A future `schema: go-msi/v2` can add
them without changing the meaning of a v1 file.

Patches, transforms, merge modules and bundles are out of scope for good
(see [Out of scope](../../README.md#out-of-scope) and
[architecture.md](architecture.md)).
