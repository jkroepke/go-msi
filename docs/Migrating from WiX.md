# Migrating from WiX

This page is for people who build their installer with the WiX toolset
today and want to build the same package with msi-builder. It shows how a
`.wxs` file turns into an `msi.yaml` file, element by element, and lists
what msi-builder does not do.

Most of a WiX file has no counterpart at all: msi-builder writes component
key paths, cabinets, short names, `ServiceControl` elements, Start menu cleanup
and the wizard's dialogs itself. A port is therefore usually much shorter
than the `.wxs` file it replaces. A test builds every YAML example on this
page with this version of msi-builder. The XML snippets use WiX v4 and later
syntax; v3 names are mentioned where they differ.

## How a port goes

1. Let msi-builder write a first `msi.yaml`:
   `msi-builder convert -o msi.yaml app.wxs`. It converts what it can and
   leaves a `# TODO` comment for everything else; see
   [convert](cli/msi-builder_convert.md). It reads one `.wxs` file, the one
   with `Package` (v3 `Product`); port fragments from other files by hand.
2. Work through these comments with the table and the sections below,
   and check the result with `msi-builder validate`. WiX variables become
   `${NAME}` references, which the comment at the top of the file lists;
   pass each one, such as `msi-builder validate --var Version=1.4.0 msi.yaml`,
   or give it a default under `vars:`.
3. Build it, and compare the tables of both packages with
   `msi-builder inspect`, which reads any `.msi`, also one built by WiX:
   `msi-builder inspect --table Registry old.msi`.
4. Install your last WiX release, upgrade it with the new package, and
   uninstall it, as described in [Checking the port](#checking-the-port).

## Keep these values

- **`UpgradeCode`** goes into `product.upgradeCode` unchanged. With another
  code, Windows sees a new product and installs it next to the old one.
- **Names that users type**, such as the install folder property
  (`INSTALLFOLDER`) and the public properties of your command line
  (`PORT=8080`). Keep the install folder's ID with `installDir.id`.
- **Component GUIDs you wrote yourself** go into `guid`. Components with
  `Guid="*"` or no `Guid` get new GUIDs from msi-builder, which is fine with
  the default upgrade, because it removes the old version before it installs
  the new one. It matters only with a late removal; see
  [Checking the port](#checking-the-port).

## A whole package

A small WiX package with a service and a `PATH` entry:

```xml
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="$(var.Version)"
           UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade DowngradeErrorMessage="A newer version of [ProductName] is already installed." />
    <MediaTemplate EmbedCab="yes" />

    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>

    <Feature Id="Main">
      <ComponentGroupRef Id="HelloComponents" />
    </Feature>

    <ComponentGroup Id="HelloComponents" Directory="INSTALLFOLDER">
      <Component>
        <File Source="hello.exe" KeyPath="yes" />
        <ServiceInstall Name="Hello" DisplayName="Hello Service" Type="ownProcess"
                        Start="auto" ErrorControl="normal" Account="LocalSystem" />
        <ServiceControl Id="HelloControl" Name="Hello" Start="install" Stop="both"
                        Remove="uninstall" Wait="yes" />
      </Component>
      <Component>
        <File Source="hello.yaml" />
      </Component>
      <Component Id="PathEnv" Guid="0b7e6b4e-6c1a-4c55-9e43-3b8f3c0d6a11" KeyPath="yes">
        <Environment Id="PATH" Name="PATH" Value="[INSTALLFOLDER]" Action="set"
                     Part="last" System="yes" Permanent="no" />
      </Component>
    </ComponentGroup>
  </Package>
</Wix>
```

The same package in msi-builder:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jkroepke/msi-builder/main/pkg/config/schema.json
schema: msi-builder/v1

product:
  name: Hello
  manufacturer: Example
  version: ${VERSION}
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301

package:
  platform: x64            # wix build -arch x64

upgrade:
  downgradeMessage: A newer version of [ProductName] is already installed.

installDir:
  id: INSTALLFOLDER
  name: Hello

components:
  - files:
      - source: hello.exe
    services:
      - name: Hello
        displayName: Hello Service
        start: auto
  - files:
      - source: hello.yaml
  - id: PathEnv
    guid: 0b7e6b4e-6c1a-4c55-9e43-3b8f3c0d6a11
    environment:
      - name: PATH
        value: '[INSTALLFOLDER]'
        action: append       # Action="set" Part="last"
        scope: machine       # System="yes"
```

Build it with `msi-builder build --var VERSION=1.4.0 -o hello.msi msi.yaml`.
The rest of this page shows the parts of a file one at a time. Each example
replaces the keys of the same name in this file.

## What you can leave out

These WiX elements and attributes need nothing in `msi.yaml`, because
msi-builder does the same by default:

| WiX | msi-builder |
|---|---|
| `MediaTemplate`, `Media`, `EmbedCab`, `Compressed="yes"` | Embedded LZX cabinets, split automatically for large payloads; `package.compression`, `package.cabinet` and `package.embedCabinets` change it |
| `InstallerVersion` | 5.0; `package.installerVersion` sets another |
| `KeyPath="yes"` on the first file, or on the component | The first file of a component, else its first registry value, else its folder; `keyPath` names another one |
| `Guid="*"` | Component GUIDs are generated from the upgrade code and stable names |
| `ProductCode="*"` (v3 `Product Id="*"`) | Generated from the upgrade code, the version, the platform and the inputs: an unchanged rebuild keeps it, any change gives a new one |
| `ComponentGroup`, `ComponentGroupRef`, `ComponentRef`, `FeatureRef` | Features list their components; without `features` every component is in one feature |
| `ServiceControl` for the package's own service | Started on install, stopped on install and uninstall, deleted on uninstall, with `Wait="yes"` |
| `RemoveFolder` and an HKCU `RegistryValue` key path next to a Start menu shortcut | Written for every shortcut and profile folder |
| Short names (`ShortName`) | Generated |
| `<Property Id="ALLUSERS">`, `Scope="perMachine"` | `package.scope`, `perMachine` by default |
| `Secure="yes"` on a public property | Public properties are secure |
| `WixUIExtension`, `WixUI` references | The `ui` keys |

## Element by element

| WiX | `msi.yaml` |
|---|---|
| `Package` (v3 `Product`) `Name`, `Manufacturer`, `Version`, `UpgradeCode`, `Language` | `product.name`, `manufacturer`, `version`, `upgradeCode`, `language` |
| `Package` `Codepage`, `Scope`; `-arch` | `package.codepage`, `package.scope`, `package.platform` |
| `SummaryInformation` | `package.summary`, `product.description`, `product.comments` |
| `ARPPRODUCTICON`, `ARPHELPLINK`, `ARPURLINFOABOUT`, `ARPURLUPDATEINFO`, `ARPCONTACT`, `ARPCOMMENTS`, `ARPNOREPAIR`, `ARPNOMODIFY`, `ARPNOREMOVE`, `ARPINSTALLLOCATION` | `product.arp` keys `icon`, `helpLink`, `aboutLink`, `updateLink`, `contact`, `comments`, `noRepair`, `noModify`, `noRemove`, `installLocation` |
| `Icon` | `icons` |
| `Binary` | `binaries` |
| `MajorUpgrade`, `Upgrade`, `UpgradeVersion` | `upgrade` |
| `StandardDirectory`, `Directory`, `DirectoryRef` | `installDir`, `components[].directory`, `directories` |
| `SetDirectory` | `customActions[].setDirectory` |
| `Feature` | `features` |
| `Component` | `components` |
| `File` | `components[].files` |
| Heat harvesting, `Files` (v5) | A folder `source` ending in `/` |
| `CreateFolder` | `components[].createFolder` |
| `RemoveFile`, `RemoveFolder` | `components[].removeFiles` |
| `util:RemoveFolderEx` | `components[].purge` |
| `RegistryKey`, `RegistryValue` | `components[].registry` |
| `RemoveRegistryKey`, `RemoveRegistryValue` | `components[].removeRegistry` |
| `Environment` | `components[].environment` |
| `Shortcut`, `util:InternetShortcut` | `files[].shortcuts`, `components[].shortcuts` |
| `ProgId`, `Extension`, `Verb` | `files[].fileTypes` |
| `ServiceInstall`, `ServiceConfig`, `util:ServiceConfig` | `components[].services` |
| `ServiceControl` for another service | `components[].serviceControl` |
| `util:EventSource` | `components[].eventLog` |
| `fw:FirewallException` | `firewall` |
| `PermissionEx` (SDDL), `Permission` | `permissions.sddl`, `folderPermissions.sddl` |
| `util:PermissionEx` | `permissions.grant`, `folderPermissions.grant`; on a registry key, `permissions.sddl` |
| `Property` | `properties` |
| `RegistrySearch`, `DirectorySearch`, `FileSearch`, `ComponentSearch` | `properties.<NAME>.search` |
| `Launch` (v3 `Condition` under `Product`) | `launchConditions` |
| `CustomAction`, `SetProperty`, `Custom` | `customActions` |
| `InstallExecuteSequence`, `InstallUISequence` | `customActions[].schedule` |
| `ScheduleReboot` | `scheduleReboot` |
| `WixUI`, `WixVariable` `WixUILicenseRtf`, `WixUIBannerBmp`, `WixUIDialogBmp` | `ui` |
| `.wxl` localization files | `ui.strings`, `ui.languages` |
| Custom tables (`CustomTable`) | `tables` |

## Variables and versions

WiX preprocessor variables become `${NAME}` variables:

| WiX | msi-builder |
|---|---|
| `$(var.Version)` with `wix build -d Version=1.4.0` | `${VERSION}` with `msi-builder build --var VERSION=1.4.0` |
| `$(env.VERSION)` | `${VERSION}` with `--env` |
| `<?define Port = 8080 ?>` | `vars:` at the top of the file |
| `!(bind.FileVersion.hello.exe)` | Pass the version as a variable |
| `<?if?>`, `<?include?>` | Not available; use component `condition`s, or generate the YAML file in your build |

`[INSTALLFOLDER]`, `[#hello.exe]` and other names in brackets work as they
do in WiX: Windows Installer fills them in at install time.

## Package and upgrades

```xml
<Icon Id="HelloIcon" SourceFile="hello.ico" />
<Property Id="ARPPRODUCTICON" Value="HelloIcon" />
<Property Id="ARPHELPLINK" Value="https://example.com/help" />
<Property Id="ARPNOREPAIR" Value="1" />

<MajorUpgrade Schedule="afterInstallValidate" AllowSameVersionUpgrades="yes"
              DowngradeErrorMessage="A newer version of [ProductName] is already installed." />
<Upgrade Id="513044d7-fae5-4715-984a-14f911d06fcc">
  <UpgradeVersion Maximum="$(var.Version)" Property="OLDX86FOUND" />
</Upgrade>
```

```yaml
product:
  name: Hello
  manufacturer: Example
  version: ${VERSION}
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
  arp:
    icon: hello.ico
    helpLink: https://example.com/help
    noRepair: true

icons:
  hello.ico: hello.ico

upgrade:
  removeOld: afterInstallValidate   # MajorUpgrade Schedule
  allowSameVersion: true            # the default
  downgradeMessage: A newer version of [ProductName] is already installed.
  related:                          # Upgrade / UpgradeVersion
    - upgradeCode: 513044d7-fae5-4715-984a-14f911d06fcc
      maxVersion: ${VERSION}
      includeMax: false
      property: OLDX86FOUND
```

- `Schedule` takes the same names in `removeOld`: `afterInstallValidate`,
  `afterInstallInitialize`, `afterInstallExecute` and
  `afterInstallFinalize`; `afterInstallExecuteAgain` has no counterpart, use
  `afterInstallExecute`. WiX's default is
  `afterInstallValidate`; msi-builder's is `afterInstallInitialize`, which
  also removes the old version first, but inside the installation, so a
  failed upgrade puts the old version back.
- `AllowDowngrades="yes"` has no direct counterpart: msi-builder's upgrade
  never removes a newer version. `downgrade: allow` and `downgrade: ignore`
  install the older version next to the newer one.
- `IgnoreRemoveFailure="yes"` is `ignoreRemoveFailure: true`.
- WiX sets `WIX_UPGRADE_DETECTED` during an upgrade, msi-builder sets
  `OLDERVERSIONBEINGUPGRADED`. If your conditions use the WiX name, keep it
  with `upgrade.property: WIX_UPGRADE_DETECTED`, and
  `upgrade.newerProperty: WIX_DOWNGRADE_DETECTED` for the other one.

## Folders

```xml
<StandardDirectory Id="ProgramFiles64Folder">
  <Directory Id="INSTALLFOLDER" Name="Hello">
    <Directory Id="CONFIGDIR" Name="config" />
  </Directory>
</StandardDirectory>
<StandardDirectory Id="CommonAppDataFolder">
  <Directory Id="DATADIR" Name="Hello" />
</StandardDirectory>
```

A component names its folder with `directory`: a folder below the install
folder, a standard folder with a path, or a directory ID:

```yaml
installDir:
  id: INSTALLFOLDER
  name: Hello

components:
  - files:
      - source: hello.exe
  - directory: config                  # CONFIGDIR: INSTALLFOLDER\config
    files:
      - source: config/hello.yaml
  - directory: CommonAppDataFolder/Hello   # DATADIR
    createFolder: true
```

Only when something else refers to a folder by its ID, such as a
`[DATADIR]` in a registry value, write the tree with `directories`:

```yaml
directories:
  - id: CommonAppDataFolder
    children:
      - id: DATADIR
        name: Hello

components:
  - files:
      - source: hello.exe
    registry:
      - root: HKLM
        key: SOFTWARE\Example\Hello
        name: DataDir
        value: '[DATADIR]'
  - directory: DATADIR
    createFolder: true
```

- The install folder is configurable by default, so
  `msiexec /i hello.msi INSTALLFOLDER=D:\Hello` works without
  `ConfigurableDirectory`.
- `ProgramFiles6432Folder` is not needed: `installDir` picks the Program
  Files folder of `package.platform`.
- A `SetDirectory` that points a folder somewhere else at install time
  becomes a `setDirectory` custom action. Often a standard folder does the
  same: `[%SYSTEMROOT]\Temp\Hello` is `WindowsFolder/Temp/Hello`.
- A WiX `RegistrySearch` that restores the install folder from the last
  install is `installDir.remember: true`. It writes its own value,
  `InstallDir` under `HKLM\Software\<manufacturer>\<product name>`, so the
  first upgrade from the WiX package does not find the WiX value. To keep
  the WiX value instead, leave `remember` out, keep the WiX registry value
  in `registry`, and turn the search into a `search` of the install folder
  property, such as `properties.INSTALLFOLDER.search` (see
  [Properties and searches](#properties-and-searches)). Use one of the two,
  not both. With `remember`, drop an `InstallDir` value that your WiX
  package wrote under that same key from `registry`.

## Components, files and features

| WiX `Component` / `File` | msi-builder |
|---|---|
| `Component Id`, `Guid`, `Directory`, `Condition` (v3 child `Condition`) | `id`, `guid`, `directory`, `condition` |
| `Permanent`, `NeverOverwrite`, `SharedDllRefCount`, `Transitive` | `permanent`, `neverOverwrite`, `sharedDllRefCount`, `transitive` |
| `Bitness="always32"`, `"always64"` (v3 `Win64`) | `bitness: 32`, `64` |
| `KeyPath="yes"` on a later file or a registry value | `keyPath` |
| `File Source`, `Name`, `Id` | `source`, `name`, `id` |
| `Checksum`, `Vital`, `ReadOnly`, `Hidden`, `System` | `checksum`, `vital`, `readOnly`, `hidden`, `system` |
| `DefaultVersion`, `CompanionFile` | `version` |

msi-builder requires a version resource on programs (`.exe`, `.dll`),
because an upgrade can lose a program without one.
`allowUnversionedExecutable: true` on the file allows it, with a warning.

A feature lists its components, by ID or in place. `Level="1"` is the
default, `Level="2"` is `default: off`, `Level="0"` is
`default: disabled`, and any other level goes into `level`:

```xml
<Feature Id="Main" Title="Hello">
  <ComponentRef Id="HelloExe" />
</Feature>
<Feature Id="Docs" Title="Documentation" Level="2">
  <ComponentGroupRef Id="DocComponents" />
</Feature>
```

```yaml
components:
  - id: HelloExe
    files:
      - source: hello.exe
  - id: Manual
    directory: docs
    files:
      - source: docs/manual.pdf

features:
  - id: Main
    title: Hello
    components: [HelloExe]
  - id: Docs
    title: Documentation
    default: off
    components: [Manual]
```

`AllowAbsent` and `ConfigurableDirectory` are `allowAbsent` and
`directory` on the feature, and `Display` is `display` with the values
`expanded`, `collapsed` and `hidden`.

Files that Heat harvested, or a WiX v5 `Files` element, become a folder
source. Every file below the folder is installed in the same structure:

```yaml
components:
  - files:
      - source: dist/
        exclude: ['*.pdb']
```

## Registry

```xml
<RegistryKey Root="HKLM" Key="SOFTWARE\Example\Hello" ForceDeleteOnUninstall="yes">
  <RegistryValue Name="InstallDir" Type="string" Value="[INSTALLFOLDER]" />
  <RegistryValue Name="Port" Type="integer" Value="8080" />
  <RegistryKey Key="Plugins" ForceCreateOnInstall="yes" />
</RegistryKey>
<RemoveRegistryKey Root="HKLM" Key="SOFTWARE\Example\HelloOld" Action="removeOnInstall" />
```

Each value is one entry with its full key. A key without values is an entry
with `type: key`:

```yaml
components:
  - files:
      - source: hello.exe
    registry:
      - root: HKLM
        key: SOFTWARE\Example\Hello
        type: key
        removeOnUninstall: true       # ForceDeleteOnUninstall="yes"
      - root: HKLM
        key: SOFTWARE\Example\Hello
        name: InstallDir
        value: '[INSTALLFOLDER]'
      - root: HKLM
        key: SOFTWARE\Example\Hello
        name: Port
        type: dword                   # Type="integer"
        value: 8080
      - root: HKLM
        key: SOFTWARE\Example\Hello\Plugins
        type: key                     # ForceCreateOnInstall="yes"
    removeRegistry:
      - root: HKLM
        key: SOFTWARE\Example\HelloOld   # RemoveRegistryKey Action="removeOnInstall"
```

| WiX `Type` | `type` |
|---|---|
| `string` | `string` (the default) |
| `expandable` | `expandString` |
| `multiString` | `multiString`, with a list as `value` |
| `integer` | `dword` |
| `binary` | `binary` |

Values the package writes are removed on uninstall, as in WiX.
`removeRegistry` removes keys and values that exist before the install,
when the component is installed. `RemoveRegistryKey` with
`Action="removeOnUninstall"` is a `type: key` entry with
`removeOnUninstall: true`.

## Environment variables

| WiX `Environment` | `environment` entry |
|---|---|
| `Action="set"` `Part="all"` (or no `Part`) | `action: set` |
| `Action="set"` `Part="last"` | `action: append` |
| `Action="set"` `Part="first"` | `action: prepend` |
| `Action="create"` | `action: setIfAbsent` |
| `Action="remove"` | `action: remove` |
| `System="yes"`, `System="no"` | `scope: machine`, `scope: user` |
| `Permanent="yes"` | `removeOnUninstall: false` |

Drop the `BroadcastEnvironmentChange` custom action reference that WiX
packages often carry. Windows Installer itself tells running programs that
the environment changed, after the install, a repair and the uninstall, so
programs started from Explorer afterwards see the new value. A command
prompt that is already open keeps the old `PATH`; open a new one.

## Shortcuts

A shortcut goes on the file it starts. msi-builder adds the folder cleanup
and the HKCU key path that WiX needs a separate component for:

```xml
<Component Id="StartMenu" Directory="ProgramMenuDir" Guid="...">
  <Shortcut Name="Hello" Description="Say hello" Target="[INSTALLFOLDER]hello.exe"
            WorkingDirectory="INSTALLFOLDER" />
  <RemoveFolder Id="RemoveProgramMenuDir" On="uninstall" />
  <RegistryValue Root="HKCU" Key="Software\Example\Hello" Name="installed"
                 Type="integer" Value="1" KeyPath="yes" />
</Component>
<Component Id="Website" Directory="ProgramMenuDir" Guid="...">
  <util:InternetShortcut Id="HelloWebsite" Name="Hello website" Target="https://example.com" />
</Component>
```

```yaml
components:
  - files:
      - source: hello.exe
        shortcuts:
          - name: Hello
            directory: ProgramMenuFolder/Example
            description: Say hello
  - id: Website
    shortcuts:
      - name: Hello website
        directory: ProgramMenuFolder/Example
        url: https://example.com
```

A shortcut starts in its program's folder. Shortcuts to folders and to
programs outside the package are component shortcuts with `folder` or
`program`. Advertised shortcuts (`Advertise="yes"`) are not supported.

## Services

```xml
<ServiceInstall Name="Hello" DisplayName="Hello Service" Description="Answers hello requests."
                Type="ownProcess" Start="auto" ErrorControl="normal" Vital="yes"
                Account="NT AUTHORITY\NetworkService" Arguments="--config &quot;[INSTALLFOLDER]hello.yaml&quot;">
  <ServiceDependency Id="Tcpip" />
  <ServiceConfig DelayedAutoStart="yes" OnInstall="yes" OnReinstall="yes" />
  <util:ServiceConfig ResetPeriodInDays="1" RestartServiceDelayInSeconds="120"
                      FirstFailureActionType="restart" SecondFailureActionType="restart"
                      ThirdFailureActionType="restart" />
</ServiceInstall>
<ServiceControl Id="HelloControl" Name="Hello" Start="install" Stop="both" Remove="uninstall" Wait="no" />
<ServiceControl Id="OldControl" Name="HelloLegacy" Stop="install" Remove="install" />
```

```yaml
components:
  - files:
      - source: hello.exe
    services:
      - name: Hello
        displayName: Hello Service
        description: Answers hello requests.
        start: auto                   # auto, demand or disabled
        vital: true
        account: NT AUTHORITY\NetworkService
        arguments: --config "[INSTALLFOLDER]hello.yaml"
        dependencies: [Tcpip]
        delayedAutoStart: true        # ServiceConfig DelayedAutoStart
        control:
          wait: false                 # ServiceControl Wait="no"
        recovery:                     # util:ServiceConfig
          resetAfterDays: 1
          actions: [restart, restart, restart]
          restartDelay: 120s
    serviceControl:                   # services the package does not install
      - name: HelloLegacy
        stopOnInstall: true
        deleteOnInstall: true
```

The service must be in the component whose file is the service program.
`ServiceControl` for the package's own service is in `control`, and only
needs keys where it differs from the defaults (`startOnInstall`,
`stopOnInstall`, `stopOnUninstall` and `deleteOnUninstall` are on, `wait`
is `true`).

## Event sources, firewall rules and permissions

```xml
<util:EventSource Name="Hello" Log="Application"
                  EventMessageFile="%SystemRoot%\System32\EventCreate.exe" />
<fw:FirewallException Id="HelloRule" Name="Hello" Program="[#hello.exe]"
                      Protocol="tcp" Port="8080" Scope="any" />
<CreateFolder Directory="DATADIR">
  <util:PermissionEx User="NT AUTHORITY\NetworkService" GenericAll="yes" />
</CreateFolder>
```

```yaml
components:
  - id: Hello
    files:
      - source: hello.exe
    eventLog:
      - source: Hello
        log: Application
        messageFile: '%SystemRoot%\System32\EventCreate.exe'
  - directory: CommonAppDataFolder/Example/Hello
    createFolder: true
    folderPermissions:
      grant:
        - account: NT AUTHORITY\NetworkService
          rights: full                # GenericAll="yes"

firewall:
  - name: Hello
    component: Hello                  # the component the rule is installed with
    program: '[#hello.exe]'
    protocol: tcp
    port: 8080
```

- `util:PermissionEx` adds rights and keeps the permissions the object
  already has; so does `grant`. The rights are `full`, `modify`,
  `readExecute`, `read` and `write`, and more in
  [YAML Schema Design](design/yaml-schema.md).
- The core `PermissionEx` with an `Sddl` attribute, and `Permission`,
  replace the permissions; write an SDDL string in `sddl` for those.
  Registry values take only `sddl`, so a `util:PermissionEx` on a registry
  key becomes an SDDL string too.
- `Scope="any"` is the default; `remoteAddresses` limits the rule and
  `profiles` the networks.

## Properties and searches

```xml
<Property Id="PORT" Value="8080" />
<Property Id="API_TOKEN" Hidden="yes" />
<Property Id="OLD_INSTALLDIR">
  <RegistrySearch Id="OldDir" Root="HKLM" Key="SOFTWARE\Example\Hello"
                  Name="InstallDir" Type="directory" />
</Property>
<Launch Condition="Installed OR VersionNT64" Message="Hello needs 64-bit Windows." />
```

```yaml
properties:
  PORT: "8080"
  API_TOKEN: { hidden: true }
  OLD_INSTALLDIR:
    search:
      registry:
        root: HKLM
        key: SOFTWARE\Example\Hello
        name: InstallDir
        type: directory

launchConditions:
  - condition: Installed OR VersionNT64
    message: Hello needs 64-bit Windows.
```

`DirectorySearch`, `FileSearch` and `ComponentSearch` are `directory`,
`file` and `component` searches; a list of searches takes the first one
that finds something. One thing works differently: in WiX, a search
overwrites a value given on the command line. In msi-builder the command
line wins, and the search result is used only when the property is still
empty.

## Custom actions

Each `CustomAction` becomes an entry of `customActions`, and its `Custom`
element in a sequence becomes a `schedule` placement of that entry:

| WiX `CustomAction` | msi-builder |
|---|---|
| `Property` + `Value`, or `SetProperty` | `setProperty: {name, value}` |
| `Directory` + `Value`, or `SetDirectory` | `setDirectory` |
| `FileRef` (v3 `FileKey`) + `ExeCommand`; `Directory` + `ExeCommand` | `run` (hidden window, output in the log) or `exec` |
| `Property` + `ExeCommand` | `execProperty` |
| `WixQuietExec` / `CAQuietExec` with the command in a property | `run` |
| `BinaryRef` (v3 `BinaryKey`) + `DllEntry` | `dll` with a `binaries` entry |
| `BinaryRef` + `ExeCommand` | `exe` with a `binaries` entry |
| `Error` | `error` |
| `Execute`, `Impersonate`, `Return`, `HideTarget` | `execute`, `impersonate`, `return`, `hideTarget` |

| WiX `Custom` | `schedule` entry |
|---|---|
| In `InstallExecuteSequence`, `InstallUISequence` | `sequence: execute`, `sequence: ui` |
| `Before`, `After`, `Sequence` | `before`, `after`, `at` |
| `Condition` (v3: the element's text) | `condition` |

```xml
<Property Id="HELLO_ARGS" Secure="yes" />
<CustomAction Id="SetHelloArgs" Property="HELLO_ARGS" Value="--config &quot;[INSTALLFOLDER]hello.yaml&quot;" />
<CustomAction Id="StopClients" FileRef="hello.exe" ExeCommand="detach --all"
              Execute="deferred" Impersonate="no" Return="ignore" />
<InstallExecuteSequence>
  <Custom Action="SetHelloArgs" Before="InstallFiles" Condition="NOT HELLO_ARGS" />
  <Custom Action="StopClients" Before="StopServices" Condition="REMOVE~=&quot;ALL&quot; AND NOT UPGRADINGPRODUCTCODE" />
</InstallExecuteSequence>
```

```yaml
properties:
  HELLO_ARGS: {}

customActions:
  - id: SetHelloArgs
    setProperty: { name: HELLO_ARGS, value: '--config "[INSTALLFOLDER]hello.yaml"' }
    schedule:
      - { sequence: execute, before: InstallFiles, condition: NOT HELLO_ARGS }
  - id: StopClients
    run:
      command: '"[#hello.exe]" detach --all'
    execute: deferred
    impersonate: false
    return: ignore
    schedule:
      - { sequence: execute, before: StopServices, when: uninstall }
```

`when` replaces the usual conditions, and is combined with `condition`
when both are given:

| WiX condition | `when` |
|---|---|
| `NOT Installed AND NOT WIX_UPGRADE_DETECTED` | `install` |
| `WIX_UPGRADE_DETECTED` | `upgrade` |
| `REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE` | `uninstall` |
| `Installed AND NOT REMOVE~="ALL"` | `maintenance` (repair and change) |

Actions with the same `before` or `after` run in the order of the file, as
in WiX. Prefer the keys of the sections above over a custom action where
one fits: services, registry values, environment variables and permissions
undo themselves on uninstall and roll back on a failed install.

## The setup wizard

| WiX | msi-builder |
|---|---|
| `WixUI_Minimal` | `ui.enabled: true` with `ui.license` |
| `WixUI_InstallDir` | `ui.enabled: true` with `chooseDirectory: true` |
| `WixUI_FeatureTree`, `WixUI_Mondo` | `ui.enabled: true` with `chooseFeatures: true` |
| `WixUI_Advanced` | `package.scope: perUserOrMachine` or `perMachineOrUser`, which adds a scope page |
| `WixUILicenseRtf` | `ui.license` (`.rtf` or text) |
| `WixUIBannerBmp`, `WixUIDialogBmp` | `ui.theme.banner`, `ui.theme.dialog` (BMP, PNG or JPEG, same sizes) |
| `WIXUI_EXITDIALOGOPTIONALCHECKBOX` with `WixShellExec` | `ui.launch.file` |
| `WIXUI_EXITDIALOGOPTIONALTEXT` | `ui.finishText` |
| A `Dialog` that asks for properties | `ui.dialogs` with `fields` |
| `.wxl` strings | `ui.strings`, and `ui.languages` for more languages in one package |

```yaml
ui:
  enabled: true
  license: license.rtf
  chooseDirectory: true
  launch:
    file: hello.exe
```

The wizard pages are msi-builder's own, with its own bitmaps. It has no
setup type page (Typical, Custom, Complete) like `WixUI_Mondo`, and dialogs
with a free layout of controls cannot be ported; `ui.dialogs` lays out
text, password, checkbox and choice fields for you. See
[YAML Schema](YAML%20Schema.md#show-a-setup-wizard) for the wizard recipes.

## Removing files

```xml
<Component Id="Logs" Directory="LOGSDIR" Guid="..." KeyPath="yes">
  <CreateFolder />
  <RemoveFile Id="RemoveLogs" Name="*.log" On="uninstall" />
</Component>
<Component Id="Data" Directory="DATADIR" Guid="...">
  <util:RemoveFolderEx On="uninstall" Property="DATADIR" />
</Component>
```

```yaml
components:
  - files:
      - source: hello.exe
  - directory: logs
    createFolder: true
    removeFiles:
      - name: '*.log'                 # one folder, no subfolders
        on: uninstall                 # install, uninstall (the default) or both
  - directory: CommonAppDataFolder/Example/Hello
    createFolder: true
    purge: true                       # util:RemoveFolderEx: the folder and everything in it
```

`purge` deletes the component's folder on uninstall, but not when an
upgrade replaces the product. The folder must be outside the install
folder, written as a path below a standard folder like
`CommonAppDataFolder/Example/Hello`; a public directory ID such as
`DATADIR`, which the command line could point anywhere, is rejected. It
takes a `condition`, such as `DELETE_DATA="1"`.

## Signing

`signtool sign` after `wix build` becomes a flag of the build, or the
`sign` command:

```shell
msi-builder build --pfx codesign.pfx --password-env PFX_PASSWORD \
  --timestamp-url http://timestamp.digicert.com -o hello.msi msi.yaml
```

msi-builder signs on Linux and macOS too; see [Signing](signing.md).

## Not supported

| WiX | What to do instead |
|---|---|
| Drivers (`difx:Driver`, DIFxApp, driver install custom actions) | Not supported |
| `util:XmlFile`, `util:XmlConfig` | Ship the complete file, or edit it with a `run` action (no rollback) |
| `util:EventManifest` | Install the manifest file and register it with `wevtutil`, see below |
| `CopyFile`, `DuplicateFile` of a package file | Install the file twice, in two components |
| `MoveFile`, `CopyFile` of a file already on the machine | A `run` action |
| `IniFile` | Ship the complete file; `.url` shortcuts are supported |
| `Class`, `TypeLib`, `AppId` (COM) | Write the registry values with `registry` |
| `util:User`, `util:Group`, `util:FileShare`, `util:PerformanceCategory`, IIS, SQL and HTTP extensions | A `run` action, if you need it |
| `util:CloseApplication` | The built-in files-in-use dialog and Restart Manager |
| `util:BroadcastEnvironmentChange` | Drop it: Windows Installer broadcasts environment changes itself; see [Environment variables](#environment-variables) |
| `util:BroadcastSettingChange` | None; if your `registry` values are a setting that Windows reads only on `WM_SETTINGCHANGE`, send it from a `run` action |
| `DiskPrompt`, `Media DiskPrompt` and `VolumeLabel` | Leave them out: they only matter for packages on several disks |
| Several `Media` disks, spanned cabinets | Large payloads are split into several cabinets automatically; above 2 GB, `package.embedCabinets: false` puts them beside the `.msi` |
| XML fragments, `<?include?>`, raw table XML | No XML input; `tables` adds tables the schema does not cover, but cannot change the standard ones |
| Hand-built dialogs | `ui.dialogs` fields |
| Merge modules (`Merge`, `MergeRef`), patches, transforms (`.mst`), Burn bundles | Never supported; ship a new package instead of a patch |
| Advertised shortcuts and installs (`Advertise="yes"`) | Never supported |
| VBScript and JScript custom actions | Never supported; use a `run` action |
| `WixExitEarlyWithSuccess` | A launch condition, which stops the install as a failure |

An event manifest needs its file installed and two `run` actions, one that
imports it on install and one that removes it on uninstall. msi-builder has
not tested this on Windows:

```yaml
components:
  - files:
      - source: hello.exe
      - source: hello.man

customActions:
  - id: ImportManifest
    run:
      command: '"[System64Folder]wevtutil.exe" im "[#hello.man]"'
    execute: deferred
    impersonate: false
    return: check
    schedule:
      - { sequence: execute, after: InstallFiles, condition: NOT REMOVE~="ALL" }
  - id: RemoveManifest
    run:
      command: '"[System64Folder]wevtutil.exe" um "[#hello.man]"'
    execute: deferred
    impersonate: false
    return: ignore
    schedule:
      - { sequence: execute, before: RemoveFiles, when: uninstall }
```

## Checking the port

1. `msi-builder validate msi.yaml` checks the file without building it.
   Every error names the key, such as `components[1].files[0].source`.
2. Compare the tables with `msi-builder inspect --table <name>` on both
   packages, for example `Registry`, `ServiceInstall` and `Environment`.
   Generated IDs and GUIDs differ; values and paths should not.
3. On a test machine, install your last WiX release, then install the new
   package over it as an upgrade, with a log:
   `msiexec /i hello.msi /qn /l*v upgrade.log`. Check that the files,
   services and settings are still there, then uninstall and check that
   nothing is left behind.

The upgrade from WiX is where a changed component GUID shows. With the
default `upgrade.removeOld` (`afterInstallInitialize`), or
`afterInstallValidate`, the old version is removed before the new one is
installed, and new GUIDs do no harm. With `afterInstallExecute`, which
`preserve: true` also selects, the new version is installed first: it
skips files that are already there, and removing the old version then
deletes the files of every component whose GUID changed. Keep the WiX
GUIDs with `guid` on those components, or keep the early removal.
`msi-builder inspect --table Component old.msi` shows the GUIDs of the WiX
package, also the ones WiX generated. A folder source cannot take a `guid`,
so its files are safe only with the early removal, which a package with
`preserve: true` cannot use.
