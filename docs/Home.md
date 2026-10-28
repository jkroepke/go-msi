# Home

msi-builder builds Windows Installer (MSI) packages from a single YAML file. It is
written in pure Go without cgo, so the same binary builds packages on Linux,
macOS and Windows, for example in a CI job next to the rest of your release.

> [!NOTE]
> msi-builder is under active development. The YAML schema is defined, the MSI
> writer and the command line tool are in progress.

## Where to start

- [Getting Started](Getting%20Started.md) builds, signs and verifies a first
  package.
- [Installation](Installation.md) lists the release binaries, `go install` and
  the container image.

## Reference

- [YAML Schema](design/yaml-schema.md) describes every section, default and
  validation rule of the configuration file.
- [Command Line](cli/msi-builder.md) lists every command and flag.
- [Signing](signing.md) covers Authenticode signatures with PFX or PEM keys.
- [Go API](go-api.md) builds packages from Go code without a YAML file.

## Background

- [MSI Specification](spec/MSISPEC.md) describes the format msi-builder writes.
- [Windows Test Results](spec/MSISPEC-RESULTS.md) records what Windows
  accepted and rejected in the validation runs.
- [Architecture](design/architecture.md) explains how the writer is built.
