package sign

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jkroepke/msi-builder/internal/cfb"
)

// ErrNotSigned is returned by Verify for a package without a signature.
var ErrNotSigned = errors.New("sign: package is not signed")

// Signature describes a verified package signature.
type Signature struct {
	// Timestamp is the RFC 3161 signing time; zero without a timestamp.
	Timestamp time.Time
	// Signer is the signing certificate.
	Signer *x509.Certificate
	// Certificates are all certificates the signature carries.
	Certificates []*x509.Certificate
	// Digest is the content digest D the signature covers.
	Digest []byte
	// Value is the signer's signature value, the SignerInfo encryptedDigest.
	// For a cabinet it is what MsiDigitalSignature.Hash holds (SIG-09).
	Value []byte
	// Hash is the digest algorithm.
	Hash crypto.Hash
	// Extended reports a MsiDigitalSignatureEx stream, whose pre-hash was
	// checked too.
	Extended bool
}

// parsedSignedData is a decoded PKCS #7 SignedData with one signer.
type parsedSignedData struct {
	contentType asn1.ObjectIdentifier
	// content is the eContent octets the messageDigest attribute covers:
	// the contents of the OCTET STRING, or of the SEQUENCE for
	// Authenticode's SpcIndirectDataContent.
	content []byte
	// contentDER is the whole eContent value.
	contentDER []byte
	certs      []*x509.Certificate
	signer     signerInfo
}

func parseSignedData(der []byte) (*parsedSignedData, error) {
	var cinfo contentInfo

	if err := unmarshalExact(der, &cinfo); err != nil {
		return nil, fmt.Errorf("malformed ContentInfo: %w", err)
	}

	if !cinfo.ContentType.Equal(oidSignedData) {
		return nil, fmt.Errorf("content type %v is not signedData", cinfo.ContentType)
	}

	outer, err := cinfo.inner()
	if err != nil {
		return nil, err
	}

	var sdata signedData

	if err := unmarshalExact(outer.FullBytes, &sdata); err != nil {
		return nil, fmt.Errorf("malformed SignedData: %w", err)
	}

	if len(sdata.SignerInfos) != 1 {
		return nil, fmt.Errorf("%d signers, want 1", len(sdata.SignerInfos))
	}

	econtent, err := sdata.ContentInfo.inner()
	if err != nil {
		return nil, err
	}

	out := &parsedSignedData{
		contentType: sdata.ContentInfo.ContentType,
		content:     econtent.Bytes,
		contentDER:  econtent.FullBytes,
		signer:      sdata.SignerInfos[0],
	}

	if len(sdata.Certificates.Bytes) > 0 {
		certs, err := x509.ParseCertificates(sdata.Certificates.Bytes)
		if err != nil {
			return nil, fmt.Errorf("certificates: %w", err)
		}

		out.certs = certs
	}

	return out, nil
}

// findSigner returns the certificate the SignerInfo names.
func (sdata *parsedSignedData) findSigner() (*x509.Certificate, error) {
	sid := sdata.signer.SID

	var match func(c *x509.Certificate) bool

	switch {
	case sid.Class == asn1.ClassUniversal && sid.Tag == asn1.TagSequence:
		var ias issuerAndSerial
		if _, err := asn1.Unmarshal(sid.FullBytes, &ias); err != nil {
			return nil, fmt.Errorf("issuerAndSerialNumber: %w", err)
		}

		match = func(c *x509.Certificate) bool {
			return bytes.Equal(c.RawIssuer, ias.Issuer.FullBytes) && c.SerialNumber.Cmp(ias.Serial) == 0
		}
	case sid.Class == asn1.ClassContextSpecific && sid.Tag == 0:
		match = func(c *x509.Certificate) bool {
			return len(c.SubjectKeyId) > 0 && bytes.Equal(c.SubjectKeyId, sid.Bytes)
		}
	default:
		return nil, errors.New("unknown signer identifier")
	}

	if i := slices.IndexFunc(sdata.certs, match); i >= 0 {
		return sdata.certs[i], nil
	}

	return nil, errors.New("the signing certificate is missing")
}

// verify checks the messageDigest attribute and the signature, and
// returns the signer's certificate and digest algorithm. Authenticode and
// RFC 3161 both require authenticated attributes.
func (sdata *parsedSignedData) verify() (*x509.Certificate, crypto.Hash, error) {
	algo, err := hashFromOID(sdata.signer.DigestAlgorithm.Algorithm)
	if err != nil {
		return nil, 0, err
	}

	cert, err := sdata.findSigner()
	if err != nil {
		return nil, 0, err
	}

	if err := sdata.checkAttributes(algo); err != nil {
		return nil, 0, err
	}

	sum, err := hashOf(algo, retagSet(sdata.signer.AuthenticatedAttributes.Bytes))
	if err != nil {
		return nil, 0, err
	}

	if err := checkSignature(cert.PublicKey, algo, sum, sdata.signer.EncryptedDigest); err != nil {
		return nil, 0, err
	}

	return cert, algo, nil
}

// checkAttributes checks the contentType and messageDigest attributes.
func (sdata *parsedSignedData) checkAttributes(algo crypto.Hash) error {
	if len(sdata.signer.AuthenticatedAttributes.FullBytes) == 0 {
		return errors.New("the signer has no authenticated attributes")
	}

	attrs, err := parseAttributes(sdata.signer.AuthenticatedAttributes.Bytes)
	if err != nil {
		return err
	}

	var contentType asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(attrs[oidContentType.String()], &contentType); err != nil || !contentType.Equal(sdata.contentType) {
		return errors.New("contentType attribute does not match the content")
	}

	var msgDigest []byte
	if _, err := asn1.Unmarshal(attrs[oidMessageDigest.String()], &msgDigest); err != nil {
		return errors.New("messageDigest attribute is missing")
	}

	want, err := hashOf(algo, sdata.content)
	if err != nil {
		return err
	}

	if !bytes.Equal(msgDigest, want) {
		return errors.New("messageDigest does not match the content")
	}

	return nil
}

// checkSignature verifies an RSA PKCS #1 v1.5 or ECDSA signature over the
// digest sum.
func checkSignature(key crypto.PublicKey, algo crypto.Hash, sum, sig []byte) error {
	switch pub := key.(type) {
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(pub, algo, sum, sig); err != nil {
			return fmt.Errorf("signature: %w", err)
		}
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(pub, sum, sig) {
			return errors.New("signature: ECDSA verification failed")
		}
	default:
		return fmt.Errorf("unsupported key type %T", pub)
	}

	return nil
}

// Verify checks the signature of pkg against its content: the content
// digest, the pre-hash when MsiDigitalSignatureEx is present, the message
// digest, the signature and the timestamp token. It returns ErrNotSigned
// for an unsigned package. Certificate trust is not checked.
func Verify(pkg []byte) (*Signature, error) {
	file, err := cfb.Read(pkg)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	der, ok := file.Stream(StreamSignature)
	if !ok {
		return nil, ErrNotSigned
	}

	sdata, err := parseSignedData(der)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	algo, signed, err := parseIndirectData(sdata)
	if err != nil {
		return nil, err
	}

	cert, signerHash, err := sdata.verify()
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	if signerHash != algo {
		return nil, fmt.Errorf("sign: signer uses %v but the content digest is %v", signerHash, algo)
	}

	sig := &Signature{Signer: cert, Certificates: sdata.certs, Hash: algo, Digest: signed, Value: sdata.signer.EncryptedDigest}

	if sig.Extended, err = checkContent(file, algo, signed); err != nil {
		return nil, err
	}

	if sig.Timestamp, err = checkUnsigned(sdata); err != nil {
		return nil, err
	}

	return sig, nil
}

// parseIndirectData returns the digest algorithm and the content digest D
// an Authenticode signature of an MSI carries.
func parseIndirectData(sdata *parsedSignedData) (crypto.Hash, []byte, error) {
	indirect, err := parseIndirect(sdata, oidSpcSipInfo)
	if err != nil {
		return 0, nil, err
	}

	var sip spcSipInfo

	if _, err := asn1.Unmarshal(indirect.Data.Value.FullBytes, &sip); err != nil || !bytes.Equal(sip.GUID, sipGUID) {
		return 0, nil, errors.New("sign: the signature is not for a Windows Installer package")
	}

	algo, err := hashFromOID(indirect.MessageDigest.Algorithm.Algorithm)
	if err != nil {
		return 0, nil, err
	}

	return algo, indirect.MessageDigest.Digest, nil
}

// parseIndirect decodes the SpcIndirectDataContent of sdata and checks that
// its data has one of the types kinds.
func parseIndirect(sdata *parsedSignedData, kinds ...asn1.ObjectIdentifier) (*spcIndirectDataContent, error) {
	if !sdata.contentType.Equal(oidSpcIndirectData) {
		return nil, fmt.Errorf("sign: signed content %v is not SpcIndirectDataContent", sdata.contentType)
	}

	var indirect spcIndirectDataContent

	if err := unmarshalExact(sdata.contentDER, &indirect); err != nil {
		return nil, fmt.Errorf("sign: malformed SpcIndirectDataContent: %w", err)
	}

	if !slices.ContainsFunc(kinds, indirect.Data.Type.Equal) {
		return nil, fmt.Errorf("sign: indirect data %v, want %v", indirect.Data.Type, kinds)
	}

	return &indirect, nil
}

// checkContent recomputes the pre-hash, when the package has one, and the
// content digest. It reports whether the signature is extended.
func checkContent(file *cfb.File, algo crypto.Hash, signed []byte) (bool, error) {
	extStream, extended := file.Stream(StreamSignatureEx)
	if extended {
		pre, err := preHash(file, algo)
		if err != nil {
			return false, err
		}

		if !bytes.Equal(pre, extStream) {
			return false, errors.New("sign: MsiDigitalSignatureEx does not match the package metadata")
		}
	}

	digest, err := contentDigest(file, algo, false)
	if err != nil {
		return false, err
	}

	if !bytes.Equal(digest, signed) {
		return false, errors.New("sign: the package content does not match its signature")
	}

	return extended, nil
}

// checkUnsigned checks the unauthenticated attributes and returns the
// timestamp, if any.
func checkUnsigned(sdata *parsedSignedData) (time.Time, error) {
	if len(sdata.signer.UnauthenticatedAttributes.FullBytes) == 0 {
		return time.Time{}, nil
	}

	attrs, err := parseAttributes(sdata.signer.UnauthenticatedAttributes.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("sign: %w", err)
	}

	if _, nested := attrs[oidNestedSignature.String()]; nested {
		return time.Time{}, errors.New("sign: nested signatures are not supported")
	}

	token, ok := attrs[oidRFC3161Token.String()]
	if !ok {
		return time.Time{}, nil
	}

	info, err := verifyToken(token, sdata.signer.EncryptedDigest)
	if err != nil {
		return time.Time{}, err
	}

	return info.GenTime, nil
}
