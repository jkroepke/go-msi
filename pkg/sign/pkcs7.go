package sign

import (
	"bytes"
	"crypto"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"unicode/utf16"
)

// Object identifiers of MSISPEC §12.
var (
	oidSignedData      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}        //nolint:gochecknoglobals // OID
	oidContentType     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}        //nolint:gochecknoglobals // OID
	oidMessageDigest   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}        //nolint:gochecknoglobals // OID
	oidRSAEncryption   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}        //nolint:gochecknoglobals // OID
	oidSpcIndirectData = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 4}    //nolint:gochecknoglobals // OID
	oidSpcSipInfo      = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 30}   //nolint:gochecknoglobals // OID
	oidSpcOpusInfo     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 12}   //nolint:gochecknoglobals // OID
	oidSpcStatement    = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 11}   //nolint:gochecknoglobals // OID
	oidSpcIndividual   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 21}   //nolint:gochecknoglobals // OID
	oidNestedSignature = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 4, 1}    //nolint:gochecknoglobals // OID
	oidRFC3161Token    = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 3, 3, 1}    //nolint:gochecknoglobals // OID
	oidTSTInfo         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4} //nolint:gochecknoglobals // OID

	oidSHA1   = asn1.ObjectIdentifier{1, 3, 14, 3, 2, 26}             //nolint:gochecknoglobals // OID
	oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1} //nolint:gochecknoglobals // OID
	oidSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2} //nolint:gochecknoglobals // OID
	oidSHA512 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3} //nolint:gochecknoglobals // OID

	oidECDSASHA1   = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 1}    //nolint:gochecknoglobals // OID
	oidECDSASHA256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2} //nolint:gochecknoglobals // OID
	oidECDSASHA384 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3} //nolint:gochecknoglobals // OID
	oidECDSASHA512 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4} //nolint:gochecknoglobals // OID
)

// sipGUID is the Windows Installer SIP {000C10F1-0000-0000-C000-000000000046}
// as stored in SpcSipInfo.
var sipGUID = []byte{0xF1, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46} //nolint:gochecknoglobals // constant

var asn1Null = asn1.RawValue{Tag: asn1.TagNull} //nolint:gochecknoglobals // constant

func hashOID(algo crypto.Hash) (asn1.ObjectIdentifier, error) {
	switch algo { //nolint:exhaustive // only these four are defined for MSI signatures
	case crypto.SHA1:
		return oidSHA1, nil
	case crypto.SHA256:
		return oidSHA256, nil
	case crypto.SHA384:
		return oidSHA384, nil
	case crypto.SHA512:
		return oidSHA512, nil
	default:
		return nil, fmt.Errorf("sign: unsupported hash %v", algo)
	}
}

func hashFromOID(oid asn1.ObjectIdentifier) (crypto.Hash, error) {
	for _, algo := range []crypto.Hash{crypto.SHA1, crypto.SHA256, crypto.SHA384, crypto.SHA512} {
		if o, _ := hashOID(algo); o.Equal(oid) {
			return algo, nil
		}
	}

	return 0, fmt.Errorf("sign: unsupported digest algorithm %v", oid)
}

func ecdsaOID(algo crypto.Hash) asn1.ObjectIdentifier {
	switch algo { //nolint:exhaustive // hashOID has rejected the rest
	case crypto.SHA1:
		return oidECDSASHA1
	case crypto.SHA384:
		return oidECDSASHA384
	case crypto.SHA512:
		return oidECDSASHA512
	default:
		return oidECDSASHA256
	}
}

// digestAlgorithm is an AlgorithmIdentifier with NULL parameters.
func digestAlgorithm(algo crypto.Hash) (pkix.AlgorithmIdentifier, error) {
	oid, err := hashOID(algo)
	if err != nil {
		return pkix.AlgorithmIdentifier{}, err
	}

	return pkix.AlgorithmIdentifier{Algorithm: oid, Parameters: asn1Null}, nil
}

// contentInfo is ContentInfo with its [0] EXPLICIT wrapper still on;
// inner unwraps it.
type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"optional,tag:0"`
}

// inner returns the value inside the [0] EXPLICIT content.
func (c *contentInfo) inner() (asn1.RawValue, error) {
	var raw asn1.RawValue

	if len(c.Content.Bytes) == 0 {
		return raw, errors.New("content is missing")
	}

	if err := unmarshalExact(c.Content.Bytes, &raw); err != nil {
		return raw, fmt.Errorf("content: %w", err)
	}

	return raw, nil
}

type signedData struct { //nolint:govet // field order is the ASN.1 order
	Version          int
	DigestAlgorithms []pkix.AlgorithmIdentifier `asn1:"set"`
	ContentInfo      contentInfo
	Certificates     asn1.RawValue `asn1:"optional,tag:0"`
	CRLs             asn1.RawValue `asn1:"optional,tag:1"`
	SignerInfos      []signerInfo  `asn1:"set"`
}

type issuerAndSerial struct { //nolint:govet // field order is the ASN.1 order
	Issuer asn1.RawValue
	Serial *big.Int
}

type signerInfo struct { //nolint:govet // field order is the ASN.1 order
	Version                   int
	SID                       asn1.RawValue
	DigestAlgorithm           pkix.AlgorithmIdentifier
	AuthenticatedAttributes   asn1.RawValue `asn1:"optional,tag:0"`
	DigestEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedDigest           []byte
	UnauthenticatedAttributes asn1.RawValue `asn1:"optional,tag:1"`
}

type attribute struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue
}

type spcAttributeValue struct {
	Type  asn1.ObjectIdentifier
	Value asn1.RawValue
}

type spcIndirectDataContent struct {
	Data          spcAttributeValue
	MessageDigest digestInfo
}

type digestInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	Digest    []byte
}

type spcSipInfo struct { //nolint:govet // field order is the ASN.1 order
	Version int
	GUID    []byte
	R1      int
	R2      int
	R3      int
	R4      int
	R5      int
}

// tlv encodes one DER value from its already encoded parts.
func tlv(class, tag int, compound bool, parts ...[]byte) []byte {
	return mustMarshal(asn1.RawValue{Class: class, Tag: tag, IsCompound: compound, Bytes: bytes.Join(parts, nil)})
}

// sequence encodes a SEQUENCE of already encoded elements.
func sequence(parts ...[]byte) []byte {
	return tlv(asn1.ClassUniversal, asn1.TagSequence, true, parts...)
}

// marshalSet encodes elements as a DER SET OF, sorted by their encodings,
// with the given class and tag ([0] IMPLICIT and friends).
func marshalSet(tag, class int, elems [][]byte) []byte {
	sorted := slices.Clone(elems)
	slices.SortFunc(sorted, bytes.Compare)

	return tlv(class, tag, true, sorted...)
}

func mustMarshal(v any) []byte {
	out, err := asn1.Marshal(v)
	if err != nil {
		// Every value marshalled here is built by this package from
		// well-formed parts; a failure is a programming error.
		panic(fmt.Sprintf("sign: asn1: %v", err))
	}

	return out
}

func newAttribute(oid asn1.ObjectIdentifier, values ...[]byte) []byte {
	return sequence(mustMarshal(oid), marshalSet(asn1.TagSet, asn1.ClassUniversal, values))
}

// indirectData returns the DER SpcIndirectDataContent for the digest d.
func indirectData(algo crypto.Hash, digest []byte) ([]byte, error) {
	alg, err := digestAlgorithm(algo)
	if err != nil {
		return nil, err
	}

	sip := mustMarshal(spcSipInfo{Version: 1, GUID: sipGUID})

	return sequence(
		sequence(mustMarshal(oidSpcSipInfo), sip),
		mustMarshal(digestInfo{Algorithm: alg, Digest: digest}),
	), nil
}

// opusInfo is SpcSpOpusInfo: programName as a BMPString and moreInfo as a
// URL, both optional.
func opusInfo(description, url string) []byte {
	var parts [][]byte

	if description != "" {
		units := utf16.Encode([]rune(description))
		bmp := make([]byte, 0, 2*len(units))

		for _, u := range units {
			bmp = binary.BigEndian.AppendUint16(bmp, u)
		}

		inner := tlv(asn1.ClassContextSpecific, 0, false, bmp)
		parts = append(parts, tlv(asn1.ClassContextSpecific, 0, true, inner))
	}

	if url != "" {
		inner := tlv(asn1.ClassContextSpecific, 0, false, []byte(url))
		parts = append(parts, tlv(asn1.ClassContextSpecific, 1, true, inner))
	}

	return sequence(parts...)
}

// unmarshalExact decodes der into v and refuses trailing data.
func unmarshalExact(der []byte, v any) error {
	rest, err := asn1.Unmarshal(der, v)
	if err != nil {
		return fmt.Errorf("asn1: %w", err)
	}

	if len(rest) != 0 {
		return errors.New("asn1: trailing data")
	}

	return nil
}

// contents returns the bytes of a DER value without its tag and length.
func contents(der []byte) ([]byte, error) {
	var raw asn1.RawValue

	if err := unmarshalExact(der, &raw); err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	return raw.Bytes, nil
}

// retagSet turns the contents of the [0] IMPLICIT authenticated attributes
// into the SET OF form that the signature covers.
func retagSet(attrs []byte) []byte {
	return tlv(asn1.ClassUniversal, asn1.TagSet, true, attrs)
}

// parseAttributes splits a SET OF Attribute into a map keyed by OID string.
// Every attribute must hold exactly one value.
func parseAttributes(set []byte) (map[string][]byte, error) {
	out := make(map[string][]byte)

	for rest := set; len(rest) > 0; {
		var attr attribute

		var err error

		rest, err = asn1.Unmarshal(rest, &attr)
		if err != nil {
			return nil, fmt.Errorf("sign: attribute: %w", err)
		}

		key := attr.Type.String()

		if attr.Values.Class != asn1.ClassUniversal || attr.Values.Tag != asn1.TagSet {
			return nil, fmt.Errorf("sign: attribute %s values are not a SET", key)
		}

		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("sign: attribute %s appears twice", key)
		}

		var value asn1.RawValue

		tail, err := asn1.Unmarshal(attr.Values.Bytes, &value)
		if err != nil || len(tail) != 0 {
			return nil, fmt.Errorf("sign: attribute %s must hold one value", key)
		}

		out[key] = value.FullBytes
	}

	return out, nil
}
