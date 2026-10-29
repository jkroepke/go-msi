package sign

import (
	"bytes"
	"context"
	"crypto"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// A signed cabinet (docs/spec/MSISPEC.md §12) sets the reserve flag and
// holds a 20-byte per-cabinet reserve after the 36-byte header: a signature
// header that gives the cabinet size and the size of the PKCS #7, which
// follows the cabinet at cbCabinet, padded with zeros to 8 bytes. The
// cabinet must have been written with that reserve (cab.Options in the
// builder), so signing only fills the reserve and appends the signature.
const (
	cabHeaderSize = 36
	// cabSigHeader is where the signature header starts, after the
	// cbCFHeader, cbCFFolder and cbCFData fields.
	cabSigHeader     = cabHeaderSize + 4
	cabSigHeaderSize = 20
	// cabFolders is where the CFFOLDER records start.
	cabFolders    = cabSigHeader + cabSigHeaderSize
	cabFolderSize = 8
	cabFlagMask   = 0x7
	cabFlagReserv = 0x4
	// cabSigMagic is the first field of the signature header.
	cabSigMagic = 0x00100000
	// cabSigAlign pads the signature.
	cabSigAlign = 8
	// cabSigMax bounds the signature size a cabinet may declare; a
	// certificate chain and a timestamp take a few kilobytes.
	cabSigMax = 1 << 20
)

// oidSpcCabImageData is the indirect data type of a cabinet signature;
// signtool writes oidSpcLink instead (SIG-08), which VerifyCabinet accepts
// too.
var (
	oidSpcCabImageData = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 25} //nolint:gochecknoglobals // OID
	oidSpcLink         = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 28} //nolint:gochecknoglobals // OID
)

// ErrCabinetNotSigned is returned by VerifyCabinet for a cabinet without a
// signature.
var ErrCabinetNotSigned = errors.New("sign: cabinet is not signed")

// CabinetFile is a cabinet that SignCabinet completes in place, such as an
// *os.File opened for reading and writing.
type CabinetFile interface {
	io.ReaderAt
	io.WriterAt
}

// cabinetHeader is the start of a cabinet with a signature reserve.
type cabinetHeader struct {
	raw [cabFolders]byte
	// size is cbCabinet, where the signature starts.
	size int64
	// signature is the padded size of the signature; 0 when unsigned.
	signature int64
}

// readCabinetHeader reads and checks the header of a cabinet that has room
// for a signature.
func readCabinetHeader(src io.ReaderAt) (*cabinetHeader, error) {
	hdr := &cabinetHeader{}
	order := binary.LittleEndian

	if err := readFull(src, hdr.raw[:], 0); err != nil {
		return nil, fmt.Errorf("sign: cabinet header: %w", err)
	}

	if err := checkCabinetHeader(hdr.raw[:]); err != nil {
		return nil, err
	}

	hdr.size = int64(order.Uint32(hdr.raw[0x08:]))
	sig := hdr.raw[cabSigHeader:cabFolders]

	if bytes.Equal(sig, make([]byte, cabSigHeaderSize)) {
		return hdr, nil
	}

	if order.Uint32(sig) != cabSigMagic || int64(order.Uint32(sig[4:])) != hdr.size {
		return nil, errors.New("sign: the cabinet's signature header does not match the cabinet")
	}

	hdr.signature = int64(order.Uint32(sig[8:]))
	if hdr.signature == 0 || hdr.signature%cabSigAlign != 0 || hdr.signature > cabSigMax {
		return nil, fmt.Errorf("sign: the cabinet declares a signature of %d bytes", hdr.signature)
	}

	return hdr, nil
}

// checkCabinetHeader checks the fixed fields of a cabinet with a signature
// reserve.
func checkCabinetHeader(raw []byte) error {
	order := binary.LittleEndian
	coffFiles := int64(order.Uint32(raw[0x10:]))

	switch {
	case string(raw[:4]) != "MSCF":
		return errors.New("sign: not a cabinet")
	case order.Uint16(raw[0x1E:])&cabFlagMask != cabFlagReserv:
		return errors.New("sign: the cabinet has no signature reserve, or is part of a set")
	case order.Uint16(raw[cabHeaderSize:]) != cabSigHeaderSize || raw[cabHeaderSize+2] != 0 || raw[cabHeaderSize+3] != 0:
		return errors.New("sign: the cabinet reserve is not a 20-byte signature reserve")
	case coffFiles != cabFolders+cabFolderSize*int64(order.Uint16(raw[0x1A:])):
		return errors.New("sign: the cabinet's file records do not follow its folders")
	case int64(order.Uint32(raw[0x08:])) < coffFiles:
		return errors.New("sign: the cabinet size is smaller than its header")
	}

	return nil
}

// digest hashes the cabinet as its signature covers it: the header without
// its reserved field at 0x04 and without iCabinet at 0x22, then the last
// field of the signature header, then everything from the folder records to
// cbCabinet.
func (hdr *cabinetHeader) digest(src io.ReaderAt, algo crypto.Hash) ([]byte, error) {
	hasher, err := newHash(algo)
	if err != nil {
		return nil, err
	}

	hasher.Write(hdr.raw[:0x04])
	hasher.Write(hdr.raw[0x08:0x22])
	hasher.Write(hdr.raw[cabFolders-4 : cabFolders])

	if _, err := io.CopyN(hasher, io.NewSectionReader(src, cabFolders, hdr.size-cabFolders), hdr.size-cabFolders); err != nil {
		return nil, fmt.Errorf("sign: read the cabinet: %w", err)
	}

	return hasher.Sum(nil), nil
}

// CabinetDigest returns the digest of the cabinet in src as its Authenticode
// signature covers it, ignoring any existing signature. The cabinet needs a
// signature reserve.
func CabinetDigest(src io.ReaderAt, algo crypto.Hash) ([]byte, error) {
	hdr, err := readCabinetHeader(src)
	if err != nil {
		return nil, err
	}

	return hdr.digest(src, algo)
}

// cabIndirectData returns the DER SpcIndirectDataContent of a cabinet:
// SpcCabImageData with an empty SpcPeImageData, as for PE files.
func cabIndirectData(algo crypto.Hash, digest []byte) ([]byte, error) {
	alg, err := digestAlgorithm(algo)
	if err != nil {
		return nil, err
	}

	// flags: empty BIT STRING; file: [0] SpcLink { [2] SpcString { [0] "" } }.
	link := tlv(asn1.ClassContextSpecific, 0, true, tlv(asn1.ClassContextSpecific, 2, true, tlv(asn1.ClassContextSpecific, 0, false)))
	image := sequence(mustMarshal(asn1.BitString{}), link)

	return sequence(
		sequence(mustMarshal(oidSpcCabImageData), image),
		mustMarshal(digestInfo{Algorithm: alg, Digest: digest}),
	), nil
}

// SignCabinet signs the unsigned cabinet in file, which must have been
// written with a signature reserve: it appends the PKCS #7 at cbCabinet and
// fills the signature header. It returns the signer's signature value, which
// MsiDigitalSignature.Hash holds: Windows Installer compares it with what
// MsiGetFileSignatureInformation reports for the cabinet (SIG-09), not with
// the content digest.
func SignCabinet(ctx context.Context, file CabinetFile, opts Options) ([]byte, error) {
	if err := opts.Check(); err != nil {
		return nil, err
	}

	hdr, err := readCabinetHeader(file)
	if err != nil {
		return nil, err
	}

	if hdr.signature != 0 {
		return nil, errors.New("sign: the cabinet is already signed")
	}

	digest, err := hdr.digest(file, opts.hash())
	if err != nil {
		return nil, err
	}

	content, err := cabIndirectData(opts.hash(), digest)
	if err != nil {
		return nil, err
	}

	signature, err := buildSignature(ctx, content, &opts)
	if err != nil {
		return nil, err
	}

	sdata, err := parseSignedData(signature)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	if err := hdr.attach(file, signature); err != nil {
		return nil, err
	}

	return sdata.signer.EncryptedDigest, nil
}

// attach writes the PKCS #7 signature after the cabinet, padded to 8 bytes,
// and fills the signature header.
func (hdr *cabinetHeader) attach(file io.WriterAt, signature []byte) error {
	padded := make([]byte, (len(signature)+cabSigAlign-1)/cabSigAlign*cabSigAlign)
	if len(padded) > cabSigMax {
		return fmt.Errorf("sign: the cabinet signature takes %d bytes, more than %d", len(padded), cabSigMax)
	}

	copy(padded, signature)

	var sig [cabSigHeaderSize]byte

	binary.LittleEndian.PutUint32(sig[:], cabSigMagic)
	binary.LittleEndian.PutUint32(sig[4:], uint32(hdr.size))    //nolint:gosec // read from a u32
	binary.LittleEndian.PutUint32(sig[8:], uint32(len(padded))) //nolint:gosec // at most cabSigMax

	if _, err := file.WriteAt(padded, hdr.size); err != nil {
		return fmt.Errorf("sign: write the signature: %w", err)
	}

	if _, err := file.WriteAt(sig[:], cabSigHeader); err != nil {
		return fmt.Errorf("sign: write the signature header: %w", err)
	}

	return nil
}

// VerifyCabinet checks the Authenticode signature of the cabinet in src
// against its content, like Verify for a package. It returns
// ErrCabinetNotSigned for a cabinet with an empty signature reserve.
// Certificate trust is not checked.
func VerifyCabinet(src io.ReaderAt) (*Signature, error) {
	hdr, err := readCabinetHeader(src)
	if err != nil {
		return nil, err
	}

	sdata, err := hdr.signedData(src)
	if err != nil {
		return nil, err
	}

	indirect, err := parseIndirect(sdata, oidSpcCabImageData, oidSpcLink)
	if err != nil {
		return nil, err
	}

	algo, err := hashFromOID(indirect.MessageDigest.Algorithm.Algorithm)
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

	digest, err := hdr.digest(src, algo)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(digest, indirect.MessageDigest.Digest) {
		return nil, errors.New("sign: the cabinet content does not match its signature")
	}

	sig := &Signature{Signer: cert, Certificates: sdata.certs, Hash: algo, Digest: digest, Value: sdata.signer.EncryptedDigest}

	if sig.Timestamp, err = checkUnsigned(sdata); err != nil {
		return nil, err
	}

	return sig, nil
}

// signedData reads the PKCS #7 after the cabinet, which only zero padding
// to 8 bytes may follow.
func (hdr *cabinetHeader) signedData(src io.ReaderAt) (*parsedSignedData, error) {
	if hdr.signature == 0 {
		return nil, ErrCabinetNotSigned
	}

	padded := make([]byte, hdr.signature)

	if err := readFull(src, padded, hdr.size); err != nil {
		return nil, fmt.Errorf("sign: read the signature: %w", err)
	}

	var outer asn1.RawValue

	rest, err := asn1.Unmarshal(padded, &outer)
	if err != nil || len(rest) >= cabSigAlign || !bytes.Equal(rest, make([]byte, len(rest))) {
		return nil, errors.New("sign: the cabinet signature is malformed")
	}

	sdata, err := parseSignedData(outer.FullBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	return sdata, nil
}

// readFull fills p from off; io.EOF with a full read is success.
func readFull(src io.ReaderAt, p []byte, off int64) error {
	n, err := src.ReadAt(p, off)
	if n == len(p) {
		return nil
	}

	if err == nil || errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}

	return err //nolint:wrapcheck // the caller adds context
}
