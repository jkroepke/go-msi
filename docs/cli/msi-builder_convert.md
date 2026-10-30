# `msi-builder convert`

Convert a WiX source file to a YAML file

Convert reads a WiX source file (.wxs, WiX v3 or later) and writes the
msi.yaml that describes the same package, as far as msi-builder can express
it. It is a starting point for a port, not a byte-for-byte translation:
everything it cannot convert, such as preprocessor conditions, includes,
bind variables, hand-built dialogs and extensions without a counterpart,
becomes a "# TODO" comment in the YAML file and a line on stderr.

The YAML file goes to stdout, or to the file --output names. Review it, then
check it with msi-builder validate.

## Usage

```text
msi-builder convert [flags] app.wxs
```

## Examples

```text
  msi-builder convert app.wxs
  msi-builder convert -o msi.yaml app.wxs
```

## Flags

```text
      --force           overwrite an existing output file
  -h, --help            help for convert
  -o, --output string   write the YAML file here instead of stdout
```

## See also

- [`msi-builder`](msi-builder.md): Build Windows Installer (MSI) packages from a YAML file
