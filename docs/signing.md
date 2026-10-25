# Signing

go-msi signs packages with Authenticode without `signtool`, cgo or the Windows
SDK. The format is described in [MSISPEC §12](spec/MSISPEC.md#12-digital-signatures),
and the Windows results are SIG-01 to SIG-06 in
[MSISPEC-RESULTS](spec/MSISPEC-RESULTS.md).

## Command line

Sign while building, or sign an existing package:

```shell
go-msi build --pfx codesign.pfx --password-env PFX_PASSWORD -o hello.msi msi.yaml
go-msi sign --cert chain.pem --key key.pem --timestamp-url http://timestamp.digicert.com hello.msi
go-msi verify hello.msi
```

| Option | Meaning |
|---|---|
| `--pfx FILE` | PKCS #12 file with the key and the certificate chain |
| `--password-env NAME` | environment variable that holds the `--pfx` password; never pass it on the command line |
| `--cert FILE`, `--key FILE` | PEM certificate chain (signing certificate first) and unencrypted PEM key (PKCS #8, PKCS #1 or SEC 1) |
| `--hash` | `sha256` (default), `sha384`, `sha512` or the legacy `sha1` |
| `--timestamp-url URL` | RFC 3161 time-stamping authority; without one, the signature stops validating when the certificate expires |
| `--description TEXT` | program name in the UAC prompt; `build` uses the product name |
| `--url URL` | "more information" link |

`go-msi sign` replaces an existing signature and writes in place unless `-o`
is given. `go-msi verify` checks the content digest, the signature and the
timestamp token, and prints the signer. It does not check certificate trust
or revocation; Windows does that at install time.

## What go-msi writes

- A basic-mode signature: `\u0005DigitalSignature` only, never
  `\u0005MsiDigitalSignatureEx`. The signature covers the stream contents and
  the root CLSID, not the container layout, so re-laying out the file keeps it
  valid.
- RSA keys of at least 2048 bits with PKCS #1 v1.5, or ECDSA keys.
- `SpcSpOpusInfo` with the description and URL, and the individual
  code-signing `SpcStatementType`.
- RSA signatures without a timestamp are deterministic: the same package and
  key give the same bytes.

Packages with sub-storages (embedded transforms) cannot be signed, because
go-msi does not write sub-storages. `go-msi verify` reads them.

## Go API

The [`sign`](../pkg/sign) package takes any `crypto.Signer`, so keys in an
HSM or a cloud KMS work without exporting them:

```go
signed, err := sign.Sign(ctx, pkg, sign.Options{
	Signer:       kmsSigner,
	Certificates: chain,
	TimestampURL: "http://timestamp.digicert.com",
})
```

For signers that only return a finished PKCS #7, `sign.Digest` gives the
content digest to sign and `sign.Attach` stores and verifies the result.
`sign.Verify` and `sign.Unsign` check and remove a signature.
