# Getting Started

This guide builds a first package: one executable that installs to
`C:\Program Files\Hello`. You need msi-builder on any operating system, see
[Installation](Installation.md), and a Windows program to package, here
`hello.exe`.

## Write the configuration

Save this file as `msi.yaml` next to `hello.exe`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jkroepke/msi-builder/main/pkg/config/schema.json
schema: msi-builder/v1

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

Generate your own `upgradeCode` once and keep it for every later version: it
ties the versions of a product together, so a newer version replaces an
installed one. The first line lets editors with the YAML language server
complete and check the file.

Every default can be spelled out. Section 3.3 of the
[YAML Schema](design/yaml-schema.md#33-defaults-and-shorthands) lists them.

## Validate and build

```shell
msi-builder validate msi.yaml
msi-builder build -o hello.msi msi.yaml
```

`validate` checks the file against the schema and runs the configuration
checks of a build, without reading the input files. It rejects configurations
that Windows would accept and then install in a broken state. `build` also
checks the input files, such as the version of `hello.exe`, and writes
`hello.msi`. With the same configuration, inputs and
`SOURCE_DATE_EPOCH`, the package is byte-identical on every build.

## Sign and verify

Windows shows an unknown publisher for an unsigned package. Pass a key and its
certificate chain to sign while building:

```shell
msi-builder build --pfx codesign.pfx --password-env PFX_PASSWORD \
  --timestamp-url http://timestamp.digicert.com -o hello.msi msi.yaml
msi-builder verify hello.msi
```

[Signing](signing.md) covers PEM keys, signing an existing package with
`msi-builder sign`, and the hash algorithms.

## Install

On Windows, install the package silently and remove it again:

```powershell
msiexec /i hello.msi /qn INSTALLDIR="D:\Apps\Hello"
msiexec /x hello.msi /qn
```

`INSTALLDIR` is optional and changes the target directory.

## Next steps

- Add shortcuts, services, registry values, a firewall rule or the install
  wizard: see the sections of the [YAML Schema](design/yaml-schema.md). The
  [windows_exporter example](../examples/windows_exporter/msi.yaml) uses most
  of them.
- Inspect a package: [`msi-builder inspect`](cli/msi-builder_inspect.md) prints its
  summary, streams and tables.
- Build packages from Go code: [Go API](go-api.md).
