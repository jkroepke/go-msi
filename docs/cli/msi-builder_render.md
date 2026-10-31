# `msi-builder render`

Print a template file as build and validate --template read it

Render runs msi.yaml through Go's text/template, as build and validate do
with --template, and prints the YAML that comes out. ${NAME} references are
left as they are. The line numbers of a YAML error with --template are lines
of this output.

## Usage

```text
msi-builder render [flags] msi.yaml
```

## Examples

```text
  msi-builder render --var ARCH=arm64 msi.yaml
```

## Flags

```text
      --env              let {{ var "NAME" }} read environment variables
  -h, --help             help for render
      --var NAME=VALUE   set a variable read as {{ .NAME }} or {{ var "NAME" }}; repeatable
```

## See also

- [`msi-builder`](msi-builder.md): Build Windows Installer (MSI) packages from a YAML file
