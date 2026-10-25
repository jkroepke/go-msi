// Package cfb writes and reads Compound File Binary (CFB) containers, the
// structured storage format that every .msi file uses.
//
// The writer fixes every free choice the way docs/spec/MSISPEC.md section 2
// describes: siblings are sorted with Compare, the directory tree is a
// balanced red-black tree, SIDs follow the sorted order, sectors are laid out
// as FAT, DIFAT, directory, MiniFAT, mini stream and then the large streams,
// timestamps and state bits are zero, and an empty stream starts at
// ENDOFCHAIN. Identical input gives identical bytes.
//
// Only root-level streams are supported, which is all a package needs.
package cfb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode"
	"unicode/utf16"
)

// Special sector values.
const (
	freeSect   = 0xFFFFFFFF
	endOfChain = 0xFFFFFFFE
	fatSect    = 0xFFFFFFFD
	difSect    = 0xFFFFFFFC
	noStream   = 0xFFFFFFFF

	miniCutoff     = 4096
	miniSectorSize = 64
	entrySize      = 128
	maxNameUnits   = 31
	// maxFileSize is the largest file the writer produces. Windows Installer
	// cannot install a package above 2 GB, so the v4 range-lock sector is
	// never needed.
	maxFileSize = 0x7FFFFE00
)

var signature = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// CLSIDs of the root storage, stored little-endian.
var (
	// CLSIDMSI is {000C1084-0000-0000-C000-000000000046}: .msi, .msm and .pcp.
	CLSIDMSI = [16]byte{0x84, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}
	// CLSIDTransform is {000C1082-…}: .mst.
	CLSIDTransform = [16]byte{0x82, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}
	// CLSIDPatch is {000C1086-…}: .msp.
	CLSIDPatch = [16]byte{0x86, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}
)

// Stream is one named root-level stream. Name is 1 to 31 UTF-16 units and
// may contain the characters Windows Installer uses for encoded names
// (U+3800–U+4840) and the U+0005 prefix of system streams.
type Stream struct {
	Name string
	Data []byte
}

// Options control the container layout.
type Options struct {
	// Version is 3 (512-byte sectors) or 4 (4096-byte sectors). 0 means 4,
	// which is what the native writer emits (MSISPEC-RESULTS CFB-01).
	Version int
	// CLSID of the root storage. Zero means CLSIDMSI.
	CLSID [16]byte
	// AllBlack colours every directory entry black instead of building a
	// red-black tree. Both are valid; the engine ignores colours.
	AllBlack bool
}

// EncodeName converts a name into UTF-16 units and checks the CFB rules.
func EncodeName(name string) ([]uint16, error) {
	units := utf16.Encode([]rune(name))
	if len(units) == 0 || len(units) > maxNameUnits {
		return nil, fmt.Errorf("cfb: name %q has %d UTF-16 units, want 1 to %d", name, len(units), maxNameUnits)
	}

	for _, u := range units {
		switch u {
		case 0, '/', '\\', ':', '!':
			return nil, fmt.Errorf("cfb: name %q contains %q", name, rune(u))
		}

		if u > 0x7F && u < 0xD800 {
			r := rune(u)
			if unicode.ToUpper(r) != r || unicode.ToLower(r) != r {
				// Compare only knows the ASCII case folding; MSI names never
				// need more.
				return nil, fmt.Errorf("cfb: name %q contains the cased character %q", name, r)
			}
		}
	}

	return units, nil
}

func upper(u uint16) uint16 {
	if u >= 'a' && u <= 'z' {
		return u - 32
	}

	return u
}

// CompareUnits orders two directory entry names: the shorter name first, then
// by upper-cased units as unsigned 16-bit values.
func CompareUnits(a, b []uint16) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}

		return 1
	}

	for i := range a {
		x, y := upper(a[i]), upper(b[i])
		if x != y {
			if x < y {
				return -1
			}

			return 1
		}
	}

	return 0
}

// Compare orders two names as CompareUnits does.
func Compare(a, b string) int {
	return CompareUnits(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
}

type entry struct {
	name  []uint16
	data  []byte
	left  int // index into the sorted entries, -1 for none
	right int
	depth int
	start uint32
}

// Write builds a compound file holding the streams.
func Write(streams []Stream, o Options) ([]byte, error) {
	switch o.Version {
	case 0:
		o.Version = 4
	case 3, 4:
	default:
		return nil, fmt.Errorf("cfb: version %d, want 3 or 4", o.Version)
	}

	if o.CLSID == [16]byte{} {
		o.CLSID = CLSIDMSI
	}

	ss := 512
	if o.Version == 4 {
		ss = 4096
	}

	entries := make([]*entry, 0, len(streams))

	for _, s := range streams {
		name, err := EncodeName(s.Name)
		if err != nil {
			return nil, err
		}

		entries = append(entries, &entry{name: name, data: s.Data, left: -1, right: -1})
	}

	sort.SliceStable(entries, func(i, j int) bool { return CompareUnits(entries[i].name, entries[j].name) < 0 })

	for i := 1; i < len(entries); i++ {
		if CompareUnits(entries[i-1].name, entries[i].name) == 0 {
			return nil, fmt.Errorf("cfb: duplicate stream name %q", string(utf16.Decode(entries[i].name)))
		}
	}

	n := len(entries)

	// Balanced tree: the middle element is the top node, recursively.
	var build func(lo, hi, depth int) int

	build = func(lo, hi, depth int) int {
		if lo >= hi {
			return -1
		}

		mid := lo + (hi-lo)/2
		e := entries[mid]
		e.depth = depth
		e.left = build(lo, mid, depth+1)
		e.right = build(mid+1, hi, depth+1)

		return mid
	}

	top := build(0, n, 0)

	blackDepth := 0
	for (1 << (blackDepth + 1)) <= n+1 {
		blackDepth++
	}

	// SID of sorted entry i is i+1; the root is SID 0.
	numEntries := n + 1
	perSector := ss / entrySize
	dirSectors := (numEntries + perSector - 1) / perSector

	// Pack small streams into the mini stream in SID order.
	var mini []byte

	largeSectors := 0

	for _, e := range entries {
		switch {
		case len(e.data) == 0:
			e.start = endOfChain
		case len(e.data) < miniCutoff:
			e.start = uint32(len(mini) / miniSectorSize)
			mini = append(mini, e.data...)

			if pad := len(mini) % miniSectorSize; pad != 0 {
				mini = append(mini, make([]byte, miniSectorSize-pad)...)
			}
		default:
			largeSectors += (len(e.data) + ss - 1) / ss
		}
	}

	numMini := len(mini) / miniSectorSize
	miniFATSectors := (numMini + ss/4 - 1) / (ss / 4)
	miniStreamSectors := (len(mini) + ss - 1) / ss

	// Fixed point for the FAT and DIFAT sector counts.
	fatSectors, difatSectors := 0, 0

	for {
		total := fatSectors + difatSectors + dirSectors + miniFATSectors + miniStreamSectors + largeSectors
		nf := (total + ss/4 - 1) / (ss / 4)

		nx := 0
		if nf > 109 {
			nx = (nf - 109 + ss/4 - 2) / (ss/4 - 1)
		}

		if nf == fatSectors && nx == difatSectors {
			break
		}

		fatSectors, difatSectors = nf, nx
	}

	total := fatSectors + difatSectors + dirSectors + miniFATSectors + miniStreamSectors + largeSectors
	if int64(total+1)*int64(ss) > maxFileSize {
		return nil, errors.New("cfb: file would exceed 2 GB")
	}

	out := make([]byte, (total+1)*ss)
	le := binary.LittleEndian

	fatStart := 0
	difatStart := fatSectors
	dirStart := difatStart + difatSectors
	miniFATStart := dirStart + dirSectors
	miniStreamStart := miniFATStart + miniFATSectors
	largeStart := miniStreamStart + miniStreamSectors

	// Header.
	copy(out, signature)
	le.PutUint16(out[0x18:], 0x3E)
	le.PutUint16(out[0x1A:], uint16(o.Version))
	le.PutUint16(out[0x1C:], 0xFFFE)

	if o.Version == 4 {
		le.PutUint16(out[0x1E:], 12)
		le.PutUint32(out[0x28:], uint32(dirSectors))
	} else {
		le.PutUint16(out[0x1E:], 9)
	}

	le.PutUint16(out[0x20:], 6)
	le.PutUint32(out[0x2C:], uint32(fatSectors))
	le.PutUint32(out[0x30:], uint32(dirStart))
	le.PutUint32(out[0x38:], miniCutoff)

	if miniFATSectors > 0 {
		le.PutUint32(out[0x3C:], uint32(miniFATStart))
	} else {
		le.PutUint32(out[0x3C:], endOfChain)
	}

	le.PutUint32(out[0x40:], uint32(miniFATSectors))

	if difatSectors > 0 {
		le.PutUint32(out[0x44:], uint32(difatStart))
	} else {
		le.PutUint32(out[0x44:], endOfChain)
	}

	le.PutUint32(out[0x48:], uint32(difatSectors))

	for i := 0; i < 109; i++ {
		v := uint32(freeSect)
		if i < fatSectors {
			v = uint32(fatStart + i)
		}

		le.PutUint32(out[0x4C+4*i:], v)
	}

	sectorOffset := func(s int) int { return (s + 1) * ss }

	// FAT.
	fat := make([]uint32, fatSectors*ss/4)
	for i := range fat {
		fat[i] = freeSect
	}

	for i := 0; i < fatSectors; i++ {
		fat[fatStart+i] = fatSect
	}

	for i := 0; i < difatSectors; i++ {
		fat[difatStart+i] = difSect
	}

	chain := func(start, count int) {
		for i := 0; i < count; i++ {
			if i == count-1 {
				fat[start+i] = endOfChain
			} else {
				fat[start+i] = uint32(start + i + 1)
			}
		}
	}

	chain(dirStart, dirSectors)
	chain(miniFATStart, miniFATSectors)
	chain(miniStreamStart, miniStreamSectors)

	cur := largeStart

	for _, e := range entries {
		if len(e.data) >= miniCutoff {
			c := (len(e.data) + ss - 1) / ss
			e.start = uint32(cur)
			chain(cur, c)
			copy(out[sectorOffset(cur):], e.data)
			cur += c
		}
	}

	for x := 0; x < difatSectors; x++ {
		off := sectorOffset(difatStart + x)

		for i := 0; i < ss/4-1; i++ {
			k := 109 + x*(ss/4-1) + i

			v := uint32(freeSect)
			if k < fatSectors {
				v = uint32(fatStart + k)
			}

			le.PutUint32(out[off+4*i:], v)
		}

		if x == difatSectors-1 {
			le.PutUint32(out[off+ss-4:], endOfChain)
		} else {
			le.PutUint32(out[off+ss-4:], uint32(difatStart+x+1))
		}
	}

	for i, v := range fat {
		le.PutUint32(out[sectorOffset(fatStart)+4*i:], v)
	}

	// MiniFAT and mini stream.
	if miniFATSectors > 0 {
		miniFAT := make([]uint32, miniFATSectors*ss/4)
		for i := range miniFAT {
			miniFAT[i] = freeSect
		}

		for _, e := range entries {
			if len(e.data) > 0 && len(e.data) < miniCutoff {
				c := (len(e.data) + miniSectorSize - 1) / miniSectorSize
				st := int(e.start)

				for k := 0; k < c; k++ {
					if k == c-1 {
						miniFAT[st+k] = endOfChain
					} else {
						miniFAT[st+k] = uint32(st + k + 1)
					}
				}
			}
		}

		for i, v := range miniFAT {
			le.PutUint32(out[sectorOffset(miniFATStart)+4*i:], v)
		}

		copy(out[sectorOffset(miniStreamStart):], mini)
	}

	// Directory.
	dirOffset := sectorOffset(dirStart)

	putEntry := func(sid int, name []uint16, typ, colour byte, left, right, child uint32, clsid []byte, start uint32, size uint64) {
		e := out[dirOffset+entrySize*sid : dirOffset+entrySize*(sid+1)]

		for i, u := range name {
			le.PutUint16(e[2*i:], u)
		}

		le.PutUint16(e[0x40:], uint16(2*(len(name)+1)))
		e[0x42] = typ
		e[0x43] = colour
		le.PutUint32(e[0x44:], left)
		le.PutUint32(e[0x48:], right)
		le.PutUint32(e[0x4C:], child)

		if clsid != nil {
			copy(e[0x50:], clsid)
		}

		le.PutUint32(e[0x74:], start)
		le.PutUint64(e[0x78:], size)
	}

	link := func(k int) uint32 {
		if k < 0 {
			return noStream
		}

		return uint32(k + 1)
	}

	rootStart := uint32(endOfChain)
	if miniStreamSectors > 0 {
		rootStart = uint32(miniStreamStart)
	}

	putEntry(0, utf16.Encode([]rune("Root Entry")), 5, 1, noStream, noStream, link(top), o.CLSID[:], rootStart, uint64(len(mini)))

	for i, e := range entries {
		colour := byte(1)
		if !o.AllBlack && e.depth >= blackDepth {
			colour = 0
		}

		putEntry(i+1, e.name, 2, colour, link(e.left), link(e.right), noStream, nil, e.start, uint64(len(e.data)))
	}

	for k := numEntries; k < dirSectors*perSector; k++ {
		e := out[dirOffset+entrySize*k:]
		le.PutUint32(e[0x44:], noStream)
		le.PutUint32(e[0x48:], noStream)
		le.PutUint32(e[0x4C:], noStream)
	}

	return out, nil
}
