# `go-msi verify`

Check the Authenticode signature of a package

Verify checks the content digest, the signature and the timestamp token
of a signed package and prints the signer. It does not check certificate
trust or revocation; Windows does that at install time. The exit code is 1
when the package is not signed or the signature does not match.

## Usage

```text
go-msi verify package.msi [flags]
```

## Examples

```text
  go-msi verify hello.msi
```

## Flags

```text
  -h, --help   help for verify
```

## See also

- [`go-msi`](go-msi.md): Build Windows Installer (MSI) packages from a YAML file
