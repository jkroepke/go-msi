# `go-msi sign`

Add an Authenticode signature to a package

Sign adds an Authenticode signature to a package, replacing an existing
one. It writes in place unless --output is given; a failed signing keeps the
original file. The key comes from a PKCS #12 file (--pfx) or from a PEM
certificate chain and key (--cert and --key). See docs/signing.md.

## Usage

```text
go-msi sign [flags] package.msi
```

## Examples

```text
  go-msi sign --pfx codesign.pfx --password-env PFX_PASSWORD hello.msi
  go-msi sign --cert chain.pem --key key.pem --timestamp-url http://timestamp.digicert.com hello.msi
```

## Flags

```text
      --cert string            sign with this PEM certificate chain (signing certificate first)
      --description string     program name shown in the UAC prompt
      --hash string            signature digest: sha256, sha384, sha512 or sha1 (default "sha256")
  -h, --help                   help for sign
      --key string             PEM private key for --cert
  -o, --output string          output file (default: sign in place)
      --password-env string    environment variable holding the --pfx password
      --pfx string             sign with the key and certificates in this PKCS #12 file
  -q, --quiet                  print nothing on success
      --timestamp-url string   RFC 3161 time-stamping authority
      --url string             more-information link of the signature
```

## See also

- [`go-msi`](go-msi.md): Build Windows Installer (MSI) packages from a YAML file
