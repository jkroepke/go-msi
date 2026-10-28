package sign

import (
	"bytes"
	"crypto"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"slices"
	"unicode/utf16"

	"github.com/jkroepke/msi-builder/pkg/cfb"
)

// Names of the root streams that hold a signature (MSISPEC §12).
const (
	StreamSignature   = "\u0005DigitalSignature"
	StreamSignatureEx = "\u0005MsiDigitalSignatureEx"
)

const (
	typeStorage = 1
	typeStream  = 2
)

// nameBytes is the UTF-16LE form of a name, the key of SigOrder.
func nameBytes(name string) []byte {
	units := utf16.Encode([]rune(name))
	out := make([]byte, 2*len(units))

	for i, u := range units {
		binary.LittleEndian.PutUint16(out[2*i:], u)
	}

	return out
}

// rawName is the UTF-16LE name of an entry as stored.
func rawName(ent *cfb.Entry) []byte {
	if ent.RawName != nil {
		return ent.RawName
	}

	return nameBytes(ent.Name)
}

// tree lists the streams and storages inside each storage, in SigOrder:
// UTF-16LE bytes, unsigned, with no length rule or case mapping, a proper
// prefix first. It is built once per walk.
type tree map[int][]*cfb.Entry

func newTree(file *cfb.File) tree {
	out := make(tree)

	for _, ent := range file.Entries {
		if ent.Parent >= 0 && ent.SID != ent.Parent && (ent.Type == typeStream || ent.Type == typeStorage) {
			out[ent.Parent] = append(out[ent.Parent], ent)
		}
	}

	for _, list := range out {
		slices.SortStableFunc(list, sigOrder)
	}

	return out
}

func sigOrder(a, b *cfb.Entry) int {
	return bytes.Compare(rawName(a), rawName(b))
}

// newHash returns a fresh hash for one of the supported algorithms.
func newHash(algo crypto.Hash) (hash.Hash, error) {
	switch algo { //nolint:exhaustive // only these four are defined for MSI signatures
	case crypto.SHA1, crypto.SHA256, crypto.SHA384, crypto.SHA512:
		if !algo.Available() {
			return nil, fmt.Errorf("sign: hash %v is not linked in", algo)
		}

		return algo.New(), nil
	default:
		return nil, fmt.Errorf("sign: unsupported hash %v", algo)
	}
}

// hashOf hashes data with algo.
func hashOf(algo crypto.Hash, data []byte) ([]byte, error) {
	hsh, err := newHash(algo)
	if err != nil {
		return nil, err
	}

	_, _ = hsh.Write(data)

	return hsh.Sum(nil), nil
}

// contentDigest is D = H(Content(root)): every stream's bytes in SigOrder,
// recursing into storages, each storage's CLSID after its children. Only
// the root's DigitalSignature stream is skipped; MsiDigitalSignatureEx is
// hashed like any other stream unless skipEx is set, which gives the digest
// a basic-mode signature of the same package covers.
func contentDigest(file *cfb.File, algo crypto.Hash, skipEx bool) ([]byte, error) { //nolint:cyclop // one walk over the tree
	hsh, err := newHash(algo)
	if err != nil {
		return nil, err
	}

	kids := newTree(file)

	var walk func(sid, depth int) error

	walk = func(sid, depth int) error {
		if depth > 64 {
			return errors.New("sign: storages nested too deep")
		}

		for _, ent := range kids[sid] {
			switch {
			case sid == 0 && ent.Type == typeStream && (ent.Name == StreamSignature || skipEx && ent.Name == StreamSignatureEx):
				continue
			case ent.Type == typeStream:
				_, _ = hsh.Write(ent.Data)
			default:
				if err := walk(ent.SID, depth+1); err != nil {
					return err
				}
			}
		}

		clsid := file.Entries[sid].CLSID
		_, _ = hsh.Write(clsid[:])

		return nil
	}

	if err := walk(0, 0); err != nil {
		return nil, err
	}

	return hsh.Sum(nil), nil
}

// preHash is P = H(Meta(root)), the content of MsiDigitalSignatureEx: the
// raw directory-entry fields of the tree in pre-order SigOrder, without
// either signature stream at the root.
func preHash(file *cfb.File, algo crypto.Hash) ([]byte, error) { //nolint:cyclop // one walk over the tree
	hsh, err := newHash(algo)
	if err != nil {
		return nil, err
	}

	var buf [8]byte

	putU32 := func(v uint32) {
		binary.LittleEndian.PutUint32(buf[:4], v)
		_, _ = hsh.Write(buf[:4])
	}
	putU64 := func(v uint64) {
		binary.LittleEndian.PutUint64(buf[:], v)
		_, _ = hsh.Write(buf[:])
	}

	root := file.Entries[0]
	_, _ = hsh.Write(root.CLSID[:])
	putU32(root.StateBits)

	kids := newTree(file)

	var walk func(sid, depth int) error

	walk = func(sid, depth int) error {
		if depth > 64 {
			return errors.New("sign: storages nested too deep")
		}

		for _, ent := range kids[sid] {
			if sid == 0 && ent.Type == typeStream && (ent.Name == StreamSignature || ent.Name == StreamSignatureEx) {
				continue
			}

			_, _ = hsh.Write(rawName(ent))

			if ent.Type == typeStream {
				putU32(uint32(ent.Size)) //nolint:gosec // the low 32 bits are hashed by definition
				putU32(ent.StateBits)
				putU64(ent.Created)
				putU64(ent.Modified)

				continue
			}

			_, _ = hsh.Write(ent.CLSID[:])
			putU32(ent.StateBits)
			putU64(ent.Created)
			putU64(ent.Modified)

			if err := walk(ent.SID, depth+1); err != nil {
				return err
			}
		}

		return nil
	}

	if err := walk(0, 0); err != nil {
		return nil, err
	}

	return hsh.Sum(nil), nil
}
