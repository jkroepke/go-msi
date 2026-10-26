# `go-msi`

Build Windows Installer (MSI) packages from a YAML file

go-msi builds Windows Installer (MSI) packages from a single YAML file,
signs them with Authenticode and inspects existing packages. It is pure Go
and builds the same packages on Linux, macOS and Windows.

## Usage

```text
go-msi [command]
```

## Flags

```text
  -h, --help      help for go-msi
      --version   print the version
```

## See also

- [`go-msi build`](go-msi_build.md): Build a package from a YAML file
- [`go-msi completion`](go-msi_completion.md): Print a shell completion script
- [`go-msi inspect`](go-msi_inspect.md): Print the tables, streams and summary of a package
- [`go-msi sign`](go-msi_sign.md): Add an Authenticode signature to a package
- [`go-msi validate`](go-msi_validate.md): Check a YAML file without building a package
- [`go-msi verify`](go-msi_verify.md): Check the Authenticode signature of a package
- [`go-msi version`](go-msi_version.md): Print the version
