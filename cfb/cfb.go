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
	"slices"
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

var signature = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1} //nolint:gochecknoglobals // lookup table

// CLSIDs of the root storage, stored little-endian.
var (
	// CLSIDMSI is {000C1084-0000-0000-C000-000000000046}: .msi, .msm and .pcp.
	CLSIDMSI = [16]byte{0x84, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46} //nolint:gochecknoglobals // lookup table
	// CLSIDTransform is {000C1082-…}: .mst.
	CLSIDTransform = [16]byte{0x82, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46} //nolint:gochecknoglobals // table
	// CLSIDPatch is {000C1086-…}: .msp.
	CLSIDPatch = [16]byte{0x86, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46} //nolint:gochecknoglobals // lookup table
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

	for _, unit := range units {
		switch unit {
		case 0, '/', '\\', ':', '!':
			return nil, fmt.Errorf("cfb: name %q contains %q", name, rune(unit))
		}

		if unit > 0x7F && unit < 0xD800 {
			r := rune(unit)
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
func CompareUnits(first, second []uint16) int {
	if len(first) != len(second) {
		if len(first) < len(second) {
			return -1
		}

		return 1
	}

	for i := range first {
		x, y := upper(first[i]), upper(second[i])
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
func Write(streams []Stream, opts Options) ([]byte, error) { //nolint:maintidx,cyclop,gocyclo,gocognit // one pass over the format; splitting would scatter it
	switch opts.Version {
	case 0:
		opts.Version = 4
	case 3, 4:
	default:
		return nil, fmt.Errorf("cfb: version %d, want 3 or 4", opts.Version)
	}

	if opts.CLSID == [16]byte{} {
		opts.CLSID = CLSIDMSI
	}

	sectorSize := 512
	if opts.Version == 4 {
		sectorSize = 4096
	}

	entries := make([]*entry, 0, len(streams))

	for _, stream := range streams {
		name, err := EncodeName(stream.Name)
		if err != nil {
			return nil, err
		}

		entries = append(entries, &entry{name: name, data: stream.Data, left: -1, right: -1})
	}

	slices.SortStableFunc(entries, func(a, b *entry) int { return CompareUnits(a.name, b.name) })

	for i := 1; i < len(entries); i++ {
		if CompareUnits(entries[i-1].name, entries[i].name) == 0 {
			return nil, fmt.Errorf("cfb: duplicate stream name %q", string(utf16.Decode(entries[i].name)))
		}
	}

	count := len(entries)

	// Balanced tree: the middle element is the top node, recursively.
	var build func(lo, hi, depth int) int

	build = func(low, high, depth int) int {
		if low >= high {
			return -1
		}

		mid := low + (high-low)/2
		e := entries[mid]
		e.depth = depth
		e.left = build(low, mid, depth+1)
		e.right = build(mid+1, high, depth+1)

		return mid
	}

	top := build(0, count, 0)

	blackDepth := 0
	for (1 << (blackDepth + 1)) <= count+1 {
		blackDepth++
	}

	// SID of sorted entry i is i+1; the root is SID 0.
	numEntries := count + 1
	perSector := sectorSize / entrySize
	dirSectors := (numEntries + perSector - 1) / perSector

	// Pack small streams into the mini stream in SID order.
	var mini []byte

	largeSectors := 0

	for _, ent := range entries {
		switch {
		case len(ent.data) == 0:
			ent.start = endOfChain
		case len(ent.data) < miniCutoff:
			ent.start = uint32(len(mini) / miniSectorSize) //nolint:gosec // bounded by the format limits checked earlier
			mini = append(mini, ent.data...)

			if pad := len(mini) % miniSectorSize; pad != 0 {
				mini = append(mini, make([]byte, miniSectorSize-pad)...)
			}
		default:
			largeSectors += (len(ent.data) + sectorSize - 1) / sectorSize
		}
	}

	numMini := len(mini) / miniSectorSize
	miniFATSectors := (numMini + sectorSize/4 - 1) / (sectorSize / 4)
	miniStreamSectors := (len(mini) + sectorSize - 1) / sectorSize

	// Fixed point for the FAT and DIFAT sector counts.
	fatSectors, difatSectors := 0, 0

	for {
		total := fatSectors + difatSectors + dirSectors + miniFATSectors + miniStreamSectors + largeSectors
		fatCount := (total + sectorSize/4 - 1) / (sectorSize / 4)

		next := 0
		if fatCount > 109 {
			next = (fatCount - 109 + sectorSize/4 - 2) / (sectorSize/4 - 1)
		}

		if fatCount == fatSectors && next == difatSectors {
			break
		}

		fatSectors, difatSectors = fatCount, next
	}

	total := fatSectors + difatSectors + dirSectors + miniFATSectors + miniStreamSectors + largeSectors
	if int64(total+1)*int64(sectorSize) > maxFileSize {
		return nil, errors.New("cfb: file would exceed 2 GB")
	}

	out := make([]byte, (total+1)*sectorSize)
	order := binary.LittleEndian

	fatStart := 0
	difatStart := fatSectors
	dirStart := difatStart + difatSectors
	miniFATStart := dirStart + dirSectors
	miniStreamStart := miniFATStart + miniFATSectors
	largeStart := miniStreamStart + miniStreamSectors

	// Header.
	copy(out, signature)
	order.PutUint16(out[0x18:], 0x3E)
	order.PutUint16(out[0x1A:], uint16(opts.Version)) //nolint:gosec // bounded by the format limits checked earlier
	order.PutUint16(out[0x1C:], 0xFFFE)

	if opts.Version == 4 {
		order.PutUint16(out[0x1E:], 12)
		order.PutUint32(out[0x28:], uint32(dirSectors)) //nolint:gosec // bounded by the format limits checked earlier
	} else {
		order.PutUint16(out[0x1E:], 9)
	}

	order.PutUint16(out[0x20:], 6)
	order.PutUint32(out[0x2C:], uint32(fatSectors))
	order.PutUint32(out[0x30:], uint32(dirStart))
	order.PutUint32(out[0x38:], miniCutoff)

	if miniFATSectors > 0 {
		order.PutUint32(out[0x3C:], uint32(miniFATStart)) //nolint:gosec // bounded by the format limits checked earlier
	} else {
		order.PutUint32(out[0x3C:], endOfChain)
	}

	order.PutUint32(out[0x40:], uint32(miniFATSectors)) //nolint:gosec // bounded by the format limits checked earlier

	if difatSectors > 0 {
		order.PutUint32(out[0x44:], uint32(difatStart))
	} else {
		order.PutUint32(out[0x44:], endOfChain)
	}

	order.PutUint32(out[0x48:], uint32(difatSectors))

	for i := range 109 {
		v := uint32(freeSect)
		if i < fatSectors {
			v = uint32(fatStart + i)
		}

		order.PutUint32(out[0x4C+4*i:], v)
	}

	sectorOffset := func(s int) int { return (s + 1) * sectorSize }

	// FAT.
	fat := make([]uint32, fatSectors*sectorSize/4)
	for i := range fat {
		fat[i] = freeSect
	}

	for i := range fatSectors {
		fat[fatStart+i] = fatSect
	}

	for i := range difatSectors {
		fat[difatStart+i] = difSect
	}

	chain := func(start, count int) {
		for i := range count {
			if i == count-1 {
				fat[start+i] = endOfChain
			} else {
				fat[start+i] = uint32(start + i + 1) //nolint:gosec // bounded by the format limits checked earlier
			}
		}
	}

	chain(dirStart, dirSectors)
	chain(miniFATStart, miniFATSectors)
	chain(miniStreamStart, miniStreamSectors)

	cur := largeStart

	for _, e := range entries {
		if len(e.data) >= miniCutoff {
			c := (len(e.data) + sectorSize - 1) / sectorSize
			e.start = uint32(cur) //nolint:gosec // bounded by the format limits checked earlier
			chain(cur, c)
			copy(out[sectorOffset(cur):], e.data)
			cur += c
		}
	}

	for difatIdx := range difatSectors {
		off := sectorOffset(difatStart + difatIdx)

		for i := range sectorSize/4 - 1 {
			k := 109 + difatIdx*(sectorSize/4-1) + i

			v := uint32(freeSect)
			if k < fatSectors {
				v = uint32(fatStart + k) //nolint:gosec // bounded by the format limits checked earlier
			}

			order.PutUint32(out[off+4*i:], v)
		}

		if difatIdx == difatSectors-1 {
			order.PutUint32(out[off+sectorSize-4:], endOfChain)
		} else {
			order.PutUint32(out[off+sectorSize-4:], uint32(difatStart+difatIdx+1))
		}
	}

	for i, v := range fat {
		order.PutUint32(out[sectorOffset(fatStart)+4*i:], v)
	}

	// MiniFAT and mini stream.
	if miniFATSectors > 0 {
		miniFAT := make([]uint32, miniFATSectors*sectorSize/4)
		for i := range miniFAT {
			miniFAT[i] = freeSect
		}

		for _, e := range entries {
			if len(e.data) > 0 && len(e.data) < miniCutoff {
				c := (len(e.data) + miniSectorSize - 1) / miniSectorSize
				startSector := int(e.start)

				for k := range c {
					if k == c-1 {
						miniFAT[startSector+k] = endOfChain
					} else {
						miniFAT[startSector+k] = uint32(startSector + k + 1) //nolint:gosec // bounded by the format limits checked earlier
					}
				}
			}
		}

		for i, v := range miniFAT {
			order.PutUint32(out[sectorOffset(miniFATStart)+4*i:], v)
		}

		copy(out[sectorOffset(miniStreamStart):], mini)
	}

	// Directory.
	dirOffset := sectorOffset(dirStart)

	putEntry := func(sid int, name []uint16, typ, colour byte, left, right, child uint32, clsid []byte, start uint32, size uint64) {
		rec := out[dirOffset+entrySize*sid : dirOffset+entrySize*(sid+1)]

		for i, u := range name {
			order.PutUint16(rec[2*i:], u)
		}

		order.PutUint16(rec[0x40:], uint16(2*(len(name)+1))) //nolint:gosec // bounded by the format limits checked earlier
		rec[0x42] = typ
		rec[0x43] = colour
		order.PutUint32(rec[0x44:], left)
		order.PutUint32(rec[0x48:], right)
		order.PutUint32(rec[0x4C:], child)

		if clsid != nil {
			copy(rec[0x50:], clsid)
		}

		order.PutUint32(rec[0x74:], start)
		order.PutUint64(rec[0x78:], size)
	}

	link := func(k int) uint32 {
		if k < 0 {
			return noStream
		}

		return uint32(k + 1) //nolint:gosec // bounded by the format limits checked earlier
	}

	rootStart := uint32(endOfChain)
	if miniStreamSectors > 0 {
		rootStart = uint32(miniStreamStart) //nolint:gosec // bounded by the format limits checked earlier
	}

	putEntry(0, utf16.Encode([]rune("Root Entry")), 5, 1, noStream, noStream, link(top), opts.CLSID[:], rootStart, uint64(len(mini)))

	for i, ent := range entries {
		colour := byte(1)
		if !opts.AllBlack && ent.depth >= blackDepth {
			colour = 0
		}

		putEntry(i+1, ent.name, 2, colour, link(ent.left), link(ent.right), noStream, nil, ent.start, uint64(len(ent.data)))
	}

	for k := numEntries; k < dirSectors*perSector; k++ {
		e := out[dirOffset+entrySize*k:]
		order.PutUint32(e[0x44:], noStream)
		order.PutUint32(e[0x48:], noStream)
		order.PutUint32(e[0x4C:], noStream)
	}

	return out, nil
}
