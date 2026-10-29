# Home

**YAML in, MSI out.** msi-builder turns one short YAML file into a Windows
Installer (MSI) package: write down what to install, run one command, ship the
`.msi`. There is no XML to write and no Windows machine needed to build on.

msi-builder is a single, self-contained binary, so the same command builds the
package on Linux, macOS and Windows, for example in a CI job next to the rest
of your release.

> [!NOTE]
> msi-builder is under active development and has not reached 1.0 yet, so the
> YAML schema can still change between releases.

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
