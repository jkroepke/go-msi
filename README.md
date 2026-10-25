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

- **Declarative.** Describe directories, files, services, registry values and
  actions. go-msi writes the tables, attribute bits and sequence numbers.
- **Safe by default.** Configurations that Windows would accept and then
  install in a broken state are rejected at build time.
- **Reproducible.** The same configuration, inputs and `SOURCE_DATE_EPOCH`
  produce a byte-identical MSI.
- **Focused.** go-msi only builds MSI packages. Sign them with a tool such as
  [relic](https://github.com/sassoftware/relic) or
  [osslsigncode](https://github.com/mtrojnar/osslsigncode). For MSIX packages,
  use [nFPM](https://github.com/goreleaser/nfpm).

> [!NOTE]
> go-msi is under active development. The YAML schema is defined, the MSI
> writer and the command line tool are in progress.

## Example

```yaml
# The smallest useful package: one executable, silent install only.
schema: go-msi/v1

product:
  name: Hello
  manufacturer: Example
  version: 1.0.0
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301

package:
  platform: x64

upgrade: {}

directories:
  - id: ProgramFiles64Folder
    children:
      - id: INSTALLDIR
        name: Hello
        configurable: true

features:
  - id: Main
    title: Hello
    components: [Hello]

components:
  - id: Hello
    directory: INSTALLDIR
    files:
      - source: hello.exe
```

```shell
go-msi validate msi.yaml
go-msi build -o hello.msi msi.yaml
```

A complete package with a Windows service, a firewall rule, an event log source
and custom actions is in
[`examples/windows_exporter`](examples/windows_exporter/msi.yaml).

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

## Documentation

- [YAML schema](docs/design/yaml-schema.md)
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
