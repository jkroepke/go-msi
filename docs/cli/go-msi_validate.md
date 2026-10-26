# `go-msi validate`

Check a YAML file without building a package

Validate loads msi.yaml, checks it against the schema and runs the
configuration checks of a build. It reads no input files: checks that need
them, such as PE versions, run only in build. Diagnostics go to stderr; the
exit code is 1 when there is an error.

## Usage

```text
go-msi validate [flags] msi.yaml
```

## Examples

```text
  go-msi validate --var VERSION=1.2.3 msi.yaml
```

## Flags

```text
      --env              let environment variables fill ${NAME} references
  -h, --help             help for validate
      --var NAME=VALUE   set a variable referenced as ${NAME}; repeatable
```

## See also

- [`go-msi`](go-msi.md): Build Windows Installer (MSI) packages from a YAML file
