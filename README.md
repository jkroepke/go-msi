[![CI](https://github.com/jkroepke/go-msi/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/jkroepke/go-msi/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/jkroepke/go-msi/graph/badge.svg)](https://codecov.io/gh/jkroepke/go-msi)
[![GitHub license](https://img.shields.io/github/license/jkroepke/go-msi)](https://github.com/jkroepke/go-msi/blob/main/LICENSE)
[![Current Release](https://img.shields.io/github/release/jkroepke/go-msi.svg?logo=github)](https://github.com/jkroepke/go-msi/releases/latest)
[![GitHub Repo stars](https://img.shields.io/github/stars/jkroepke/go-msi?style=flat&logo=github)](https://github.com/jkroepke/go-msi/stargazers)

# go-msi

⭐ Don't forget to star this repository! ⭐

## About

go-msi builds Windows Installer (MSI) packages from a single YAML file. It is
written in pure Go without cgo, so the same binary builds packages on Linux,
macOS and Windows, for example in a CI job next to the rest of your release.

- **Declarative.** Describe directories, files, shortcuts, services, registry
  values and actions. go-msi writes the tables, attribute bits and sequence numbers.
- **Safe by default.** Configurations that Windows would accept and then
  install in a broken state are rejected at build time.
- **Reproducible.** The same configuration, inputs and `SOURCE_DATE_EPOCH`
  produce a byte-identical MSI.
- **Signed.** go-msi adds an Authenticode signature itself, with an RFC 3161
  timestamp, so no Windows SDK or `signtool` is needed.
- **Focused.** go-msi only builds and signs MSI packages. For MSIX packages, use
  [nFPM](https://github.com/goreleaser/nfpm).

> [!NOTE]
> go-msi is under active development. The YAML schema is defined, the MSI
> writer and the command line tool are in progress.

## Example

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/jkroepke/go-msi/main/pkg/config/schema.json
# The smallest useful package: one executable, silent install only.
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

The file goes to `C:\Program Files\Hello`, which `INSTALLDIR=…` can change,
and a newer version replaces an installed one. Every default can be spelled
out; the [YAML schema](docs/design/yaml-schema.md) lists them.

```shell
go-msi validate msi.yaml
go-msi build -o hello.msi msi.yaml
```

To sign the package, pass a key and its certificate chain:

```shell
go-msi build --pfx codesign.pfx --password-env PFX_PASSWORD \
  --timestamp-url http://timestamp.digicert.com -o hello.msi msi.yaml
go-msi verify hello.msi
```

See [Signing](docs/signing.md) for PEM keys, `go-msi sign` and the Go API.

Packages can also be built from Go without a YAML file, including a whole
folder tree; see the [Go API](docs/go-api.md).

A complete package with a Windows service, a firewall rule, an event log source
and custom actions is in
[`examples/windows_exporter`](examples/windows_exporter/msi.yaml).

## Out of scope

go-msi aims at simple, single-product installers such as an exporter or an
agent with a service. These are not supported:

- **MSIX.** Use [nFPM](https://github.com/goreleaser/nfpm).
- **Running packages.** go-msi never installs or executes an MSI. Only its
  tests do, on Windows without `-short` (elevated).
- **Transforms (`.mst`), patches (`.msp`), merge modules (`.msm`) and
  bundles or bootstrappers.** Ship a new major-upgrade MSI instead of a patch.
- **Multi-language packages.** A package has one language; there are no
  embedded language transforms. Not in schema v1.
- **Spanned cabinets.** Every cabinet holds one folder; cabinets are LZX
  (the default), MSZIP or stored.
- **Script custom actions** (VBScript, JScript). Use an executable or DLL
  action instead. Not in schema v1.
- **Editing existing packages and WiX XML input.** `go-msi inspect` only
  reads packages to help debugging.
- **Advertised installs and advertised shortcuts.** Not in schema v1;
  shortcuts are ordinary `.lnk` files.
- **File harvesting in YAML.** List each file of a component; a YAML file
  cannot add a whole directory. Not in schema v1. Go callers can use
  `Config.AddTree` ([Go API](docs/go-api.md)).
- **Hand-built dialogs.** The UI is a built-in wizard plus optional form
  pages that ask for properties; there is no free control layout.

## Installation

Download a binary for Linux, macOS or Windows from the
[releases page](https://github.com/jkroepke/go-msi/releases/latest), or install
it with Go:

```shell
go install github.com/jkroepke/go-msi/cmd/go-msi@latest
```

or run the container image for Linux on amd64 and arm64:

```shell
docker run --rm -v "$PWD:/work" ghcr.io/jkroepke/go-msi build -o hello.msi msi.yaml
```

`go-msi completion <shell>` prints a completion script for Bash, Zsh, fish or
PowerShell; `go-msi completion <shell> --help` shows how to load it.

## Documentation

- [Command-line reference](docs/cli/go-msi.md)
- [YAML schema](docs/design/yaml-schema.md)
- [Signing](docs/signing.md)
- [Go API](docs/go-api.md)
- [MSI specification](docs/spec/MSISPEC.md) and its
  [Windows test results](docs/spec/MSISPEC-RESULTS.md)

The documentation is also published in the
[wiki](https://github.com/jkroepke/go-msi/wiki).

## Contributing

See the [contributing guide](CONTRIBUTING.md) to propose and submit changes, and
the [code of conduct](CODE_OF_CONDUCT.md) for how we work together. Report
security issues as described in the [security policy](SECURITY.md).

## License

This project is licensed under the [Apache License 2.0](LICENSE).
