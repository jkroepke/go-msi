// Package sign adds, checks and removes the Authenticode signature of an
// .msi package without cgo or external tools.
//
// The signature is a PKCS #7 SignedData over a digest of the package's
// logical content, stored in the root stream \u0005DigitalSignature
// (docs/spec/MSISPEC.md §12). Sign writes basic-mode signatures only: it
// never writes \u0005MsiDigitalSignatureEx, so the signature does not depend
// on directory-entry times or the container layout. Verify accepts both
// modes.
//
// Verify checks the digests and the cryptographic signature. It does not
// build or trust a certificate chain; that is Windows' job.
package sign

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/jkroepke/msi-builder/internal/cfb"
)

// Options control Sign.
type Options struct {
	// Signer holds the private key: an *rsa.PrivateKey, an
	// *ecdsa.PrivateKey or any crypto.Signer backed by an HSM or cloud KMS.
	Signer crypto.Signer
	// Rand is the entropy source for the signature and the timestamp
	// nonce. Nil means crypto/rand.
	Rand io.Reader
	// HTTPClient sends the timestamp request. Nil means a client with a
	// one-minute timeout.
	HTTPClient *http.Client
	// Description is the program name shown in the UAC prompt
	// (SpcSpOpusInfo.programName). Optional.
	Description string
	// URL is the "more information" link (SpcSpOpusInfo.moreInfo).
	// Optional.
	URL string
	// TimestampURL is an RFC 3161 time-stamping authority. Empty means no
	// timestamp, and the signature stops validating when the certificate
	// expires.
	TimestampURL string
	// Certificates holds the signing certificate first, then any
	// intermediates. The root can be left out.
	Certificates []*x509.Certificate
	// Hash is SHA-256 when zero. SHA-1 is legacy; SHA-384 and SHA-512
	// need a recent Windows.
	Hash crypto.Hash
}

// Check reports whether the options can sign: a signer, its certificate,
// and a key the format supports. Sign and SignCabinet check the same, so a
// caller can find a bad key before it does the work that comes first.
func (o *Options) Check() error {
	if o.Signer == nil || len(o.Certificates) == 0 {
		return errors.New("sign: a signer and its certificate are required")
	}

	return checkKey(o.Signer, o.Certificates[0])
}

func (o *Options) hash() crypto.Hash {
	if o.Hash == 0 {
		return crypto.SHA256
	}

	return o.Hash
}

// readPackage parses an .msi and returns it with its root streams, minus
// both signature streams.
func readPackage(pkg []byte) (*cfb.File, []cfb.Stream, error) {
	file, err := cfb.Read(pkg)
	if err != nil {
		return nil, nil, fmt.Errorf("sign: %w", err)
	}

	for _, ent := range file.Entries[1:] {
		if ent.Type == typeStorage && ent.Parent >= 0 {
			return nil, nil, fmt.Errorf("sign: storage %q: packages with sub-storages are not supported", ent.Name)
		}
	}

	var streams []cfb.Stream

	for _, s := range file.Streams() {
		if s.Name != StreamSignature && s.Name != StreamSignatureEx {
			streams = append(streams, s)
		}
	}

	return file, streams, nil
}

// rewrite builds a container like file holding streams.
func rewrite(file *cfb.File, streams []cfb.Stream) ([]byte, error) {
	out, err := cfb.Write(streams, cfb.Options{Version: file.Version, CLSID: file.CLSID})
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	return out, nil
}

// Unsign returns pkg without its signature streams.
func Unsign(pkg []byte) ([]byte, error) {
	file, streams, err := readPackage(pkg)
	if err != nil {
		return nil, err
	}

	return rewrite(file, streams)
}

// Digest returns the content digest D of pkg as a basic-mode signature
// covers it, ignoring any existing signature. An external signer puts D
// into SpcIndirectDataContent; Attach then stores its PKCS #7.
func Digest(pkg []byte, algo crypto.Hash) ([]byte, error) {
	file, _, err := readPackage(pkg)
	if err != nil {
		return nil, err
	}

	return contentDigest(file, algo, true)
}

// Attach stores an externally made DER PKCS #7 signature in pkg, replacing
// any existing one, and verifies the result.
func Attach(pkg, signature []byte) ([]byte, *Signature, error) {
	file, streams, err := readPackage(pkg)
	if err != nil {
		return nil, nil, err
	}

	streams = append(streams, cfb.Stream{Name: StreamSignature, Data: signature})

	out, err := rewrite(file, streams)
	if err != nil {
		return nil, nil, err
	}

	sig, err := Verify(out)
	if err != nil {
		return nil, nil, err
	}

	return out, sig, nil
}

// Sign returns pkg with a fresh basic-mode signature, replacing any
// existing one.
func Sign(ctx context.Context, pkg []byte, opts Options) ([]byte, error) {
	if err := opts.Check(); err != nil {
		return nil, err
	}

	digest, err := Digest(pkg, opts.hash())
	if err != nil {
		return nil, err
	}

	content, err := indirectData(opts.hash(), digest)
	if err != nil {
		return nil, err
	}

	p7, err := buildSignature(ctx, content, &opts)
	if err != nil {
		return nil, err
	}

	out, _, err := Attach(pkg, p7)

	return out, err
}

// checkKey makes sure the key belongs to the certificate and is one the
// signature format supports.
func checkKey(signer crypto.Signer, cert *x509.Certificate) error {
	type equaler interface{ Equal(x crypto.PublicKey) bool }

	pub, ok := signer.Public().(equaler)
	if !ok || !pub.Equal(cert.PublicKey) {
		return errors.New("sign: the key does not match the signing certificate")
	}

	switch key := signer.Public().(type) {
	case *rsa.PublicKey:
		if key.N.BitLen() < 2048 {
			return fmt.Errorf("sign: RSA keys need at least 2048 bits, got %d", key.N.BitLen())
		}
	case *ecdsa.PublicKey:
	default:
		return fmt.Errorf("sign: unsupported key type %T", key)
	}

	return nil
}

// buildSignature returns the DER ContentInfo that signs content, a DER
// SpcIndirectDataContent.
func buildSignature(ctx context.Context, content []byte, opts *Options) ([]byte, error) {
	algo := opts.hash()

	alg, err := digestAlgorithm(algo)
	if err != nil {
		return nil, err
	}

	signedAttrs, toSign, err := signedAttributes(algo, content, opts)
	if err != nil {
		return nil, err
	}

	random := opts.Rand
	if random == nil {
		random = rand.Reader
	}

	sig, err := opts.Signer.Sign(random, toSign, algo)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	signer := opts.Certificates[0]
	parts := [][]byte{
		mustMarshal(1),
		sequence(signer.RawIssuer, mustMarshal(signer.SerialNumber)),
		mustMarshal(alg),
		signedAttrs,
		encryptionAlgorithm(opts.Signer.Public(), algo),
		mustMarshal(sig),
	}

	if opts.TimestampURL != "" {
		token, err := timestamp(ctx, opts, algo, sig)
		if err != nil {
			return nil, err
		}

		parts = append(parts, marshalSet(1, asn1.ClassContextSpecific, [][]byte{newAttribute(oidRFC3161Token, token)}))
	}

	certs := make([][]byte, 0, len(opts.Certificates))
	for _, c := range opts.Certificates {
		certs = append(certs, c.Raw)
	}

	sdata := sequence(
		mustMarshal(1),
		marshalSet(asn1.TagSet, asn1.ClassUniversal, [][]byte{mustMarshal(alg)}),
		sequence(mustMarshal(oidSpcIndirectData), tlv(asn1.ClassContextSpecific, 0, true, content)),
		marshalSet(0, asn1.ClassContextSpecific, certs),
		marshalSet(asn1.TagSet, asn1.ClassUniversal, [][]byte{sequence(parts...)}),
	)

	return sequence(mustMarshal(oidSignedData), tlv(asn1.ClassContextSpecific, 0, true, sdata)), nil
}

// signedAttributes returns the [0] IMPLICIT authenticated attributes for
// the DER SpcIndirectDataContent and the digest the signer signs.
func signedAttributes(algo crypto.Hash, content []byte, opts *Options) ([]byte, []byte, error) {
	body, err := contents(content)
	if err != nil {
		return nil, nil, err
	}

	msgDigest, err := hashOf(algo, body)
	if err != nil {
		return nil, nil, err
	}

	signedAttrs := marshalSet(0, asn1.ClassContextSpecific, [][]byte{
		newAttribute(oidContentType, mustMarshal(oidSpcIndirectData)),
		newAttribute(oidMessageDigest, mustMarshal(msgDigest)),
		newAttribute(oidSpcOpusInfo, opusInfo(opts.Description, opts.URL)),
		newAttribute(oidSpcStatement, sequence(mustMarshal(oidSpcIndividual))),
	})

	setBody, err := contents(signedAttrs)
	if err != nil {
		return nil, nil, err
	}

	toSign, err := hashOf(algo, retagSet(setBody))
	if err != nil {
		return nil, nil, err
	}

	return signedAttrs, toSign, nil
}

// encryptionAlgorithm is the SignerInfo's digestEncryptionAlgorithm.
func encryptionAlgorithm(pub crypto.PublicKey, algo crypto.Hash) []byte {
	if _, ok := pub.(*ecdsa.PublicKey); ok {
		return sequence(mustMarshal(ecdsaOID(algo)))
	}

	return sequence(mustMarshal(oidRSAEncryption), mustMarshal(asn1Null))
}
