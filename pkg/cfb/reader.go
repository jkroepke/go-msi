package cfb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
)

// File is a parsed compound file.
type File struct {
	byName     map[string]*Entry
	Entries    []*Entry
	streams    []Stream
	Version    int
	SectorSize int
	CLSID      [16]byte
}

// Entry is one directory entry.
type Entry struct {
	Name string
	// RawName is the stored name without its terminator, UTF-16LE. It
	// keeps units that Name cannot hold, such as lone surrogates.
	RawName []byte
	Data    []byte
	SID     int
	Size    uint64
	Parent  int
	// Created and Modified are the raw FILETIME fields at 0x64 and 0x6C;
	// StateBits is the field at 0x60. The signature pre-hash covers them.
	Created   uint64
	Modified  uint64
	CLSID     [16]byte
	Left      uint32
	Right     uint32
	Child     uint32
	Start     uint32
	StateBits uint32
	Type      byte
	Colour    byte
}

// Streams returns the root-level streams in SID order.
func (f *File) Streams() []Stream { return f.streams }

// Stream returns the data of the root-level stream with the given name.
func (f *File) Stream(name string) ([]byte, bool) {
	e, ok := f.byName[name]
	if !ok {
		return nil, false
	}

	return e.Data, true
}

type reader struct {
	raw     []byte
	fat     []uint32
	miniFAT []uint32
	ss      int
}

func (r *reader) sectorOffset(n uint32) (int, error) {
	off := (int(n) + 1) * r.ss
	if n >= 0xFFFFFFFA || off+r.ss > len(r.raw) {
		return 0, fmt.Errorf("cfb: sector %#x out of range", n)
	}

	return off, nil
}

func (r *reader) chain(start uint32, limit int) ([]uint32, error) {
	var out []uint32

	seen := make(map[uint32]bool)

	for sector := start; sector != endOfChain; {
		if sector >= 0xFFFFFFFA || int(sector) >= len(r.fat) || seen[sector] {
			return nil, fmt.Errorf("cfb: bad FAT chain at %#x", sector)
		}

		seen[sector] = true
		out = append(out, sector)

		if len(out) > limit {
			return nil, errors.New("cfb: chain too long")
		}

		sector = r.fat[sector]
	}

	return out, nil
}

// Read parses a compound file and loads every root-level stream.
func Read(raw []byte) (*File, error) { //nolint:maintidx,cyclop,gocyclo,gocognit // one pass over the format; splitting would scatter it
	if len(raw) < 512 || !bytes.Equal(raw[:8], signature) {
		return nil, errors.New("cfb: bad signature")
	}

	order := binary.LittleEndian

	if order.Uint16(raw[0x1C:]) != 0xFFFE {
		return nil, errors.New("cfb: bad byte order mark")
	}

	major := int(order.Uint16(raw[0x1A:]))
	shift := order.Uint16(raw[0x1E:])

	if (major != 3 || shift != 9) && (major != 4 || shift != 12) {
		return nil, fmt.Errorf("cfb: unsupported version %d with sector shift %d", major, shift)
	}

	if order.Uint16(raw[0x20:]) != 6 {
		return nil, errors.New("cfb: unsupported mini sector shift")
	}

	rdr := &reader{raw: raw, ss: 1 << shift}
	file := &File{Version: major, SectorSize: rdr.ss, byName: make(map[string]*Entry)}
	folded := make(map[string]bool)

	numFAT := order.Uint32(raw[0x2C:])
	firstDir := order.Uint32(raw[0x30:])
	firstMiniFAT := order.Uint32(raw[0x3C:])
	firstDIFAT := order.Uint32(raw[0x44:])

	var fatSectors []uint32

	for i := 0; i < 109 && uint32(len(fatSectors)) < numFAT; i++ { //nolint:gosec // bounded by the format limits checked earlier
		fatSectors = append(fatSectors, order.Uint32(raw[0x4C+4*i:]))
	}

	seenDIFAT := make(map[uint32]bool)

	for difat := firstDIFAT; difat != endOfChain && difat != freeSect && uint32(len(fatSectors)) < numFAT; { //nolint:gosec // bounded
		if seenDIFAT[difat] {
			return nil, errors.New("cfb: DIFAT loop")
		}

		seenDIFAT[difat] = true

		off, err := rdr.sectorOffset(difat)
		if err != nil {
			return nil, fmt.Errorf("cfb: DIFAT: %w", err)
		}

		for i := 0; i < rdr.ss/4-1 && uint32(len(fatSectors)) < numFAT; i++ { //nolint:gosec // bounded by the format limits checked earlier
			fatSectors = append(fatSectors, order.Uint32(raw[off+4*i:]))
		}

		difat = order.Uint32(raw[off+rdr.ss-4:])
	}

	for _, s := range fatSectors {
		off, err := rdr.sectorOffset(s)
		if err != nil {
			return nil, fmt.Errorf("cfb: FAT: %w", err)
		}

		for i := range rdr.ss / 4 {
			rdr.fat = append(rdr.fat, order.Uint32(raw[off+4*i:]))
		}
	}

	dirChain, err := rdr.chain(firstDir, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("cfb: directory: %w", err)
	}

	for _, s := range dirChain {
		off, err := rdr.sectorOffset(s)
		if err != nil {
			return nil, fmt.Errorf("cfb: directory: %w", err)
		}

		for i := range rdr.ss / entrySize {
			rec := raw[off+entrySize*i : off+entrySize*(i+1)]
			nameLen := int(order.Uint16(rec[0x40:]))

			ent := &Entry{
				SID:    len(file.Entries),
				Type:   rec[0x42],
				Colour: rec[0x43],
				Left:   order.Uint32(rec[0x44:]),
				Right:  order.Uint32(rec[0x48:]),
				Child:  order.Uint32(rec[0x4C:]),
				Start:  order.Uint32(rec[0x74:]),
				Size:   order.Uint64(rec[0x78:]),
				Parent: -1,

				StateBits: order.Uint32(rec[0x60:]),
				Created:   order.Uint64(rec[0x64:]),
				Modified:  order.Uint64(rec[0x6C:]),
			}

			if major == 3 {
				ent.Size &= 0xFFFFFFFF
			}

			copy(ent.CLSID[:], rec[0x50:0x60])

			if nameLen >= 2 && nameLen <= 64 {
				units := make([]uint16, 0, nameLen/2-1)
				for j := range nameLen/2 - 1 {
					units = append(units, order.Uint16(rec[2*j:]))
				}

				ent.Name = string(utf16.Decode(units))
				ent.RawName = bytes.Clone(rec[:nameLen-2])
			}

			file.Entries = append(file.Entries, ent)
		}
	}

	if len(file.Entries) == 0 || file.Entries[0].Type != 5 {
		return nil, errors.New("cfb: no root entry")
	}

	root := file.Entries[0]
	file.CLSID = root.CLSID

	if firstMiniFAT != endOfChain && firstMiniFAT != freeSect {
		mc, err := rdr.chain(firstMiniFAT, 1<<20)
		if err != nil {
			return nil, fmt.Errorf("cfb: MiniFAT: %w", err)
		}

		for _, s := range mc {
			off, err := rdr.sectorOffset(s)
			if err != nil {
				return nil, fmt.Errorf("cfb: MiniFAT: %w", err)
			}

			for i := range rdr.ss / 4 {
				rdr.miniFAT = append(rdr.miniFAT, order.Uint32(raw[off+4*i:]))
			}
		}
	}

	var rootChain []uint32

	if root.Size > 0 {
		rootChain, err = rdr.chain(root.Start, 1<<22)
		if err != nil {
			return nil, fmt.Errorf("cfb: mini stream: %w", err)
		}
	}

	// Walk the tree from the root with a visited set.
	visited := map[uint32]bool{0: true}

	var walk func(sid uint32, parent int) error

	walk = func(sid uint32, parent int) error {
		if sid == noStream {
			return nil
		}

		if int(sid) >= len(file.Entries) || visited[sid] {
			return fmt.Errorf("cfb: bad sibling link %d", sid)
		}

		visited[sid] = true
		ent := file.Entries[sid]
		ent.Parent = parent

		if err := walk(ent.Left, parent); err != nil {
			return err
		}

		if err := walk(ent.Right, parent); err != nil {
			return err
		}

		if ent.Type == 1 {
			return walk(ent.Child, int(sid))
		}

		return nil
	}

	if err := walk(root.Child, 0); err != nil {
		return nil, err
	}

	for _, ent := range file.Entries {
		if ent.Type != 2 || ent.Parent < 0 {
			continue
		}

		if ent.Size > uint64(len(raw)) {
			return nil, fmt.Errorf("cfb: stream %q larger than the file", ent.Name)
		}

		switch {
		case ent.Size == 0:
			// Any start sector is fine for an empty stream.
		case ent.Size < miniCutoff:
			ent.Data = make([]byte, 0, ent.Size)

			for mini, n := ent.Start, uint64(0); n*miniSectorSize < ent.Size; n++ {
				if mini == endOfChain || int(mini) >= len(rdr.miniFAT) || mini >= 0xFFFFFFFA {
					return nil, fmt.Errorf("cfb: bad MiniFAT chain for %q", ent.Name)
				}

				pos := int(mini) * miniSectorSize
				if pos/rdr.ss >= len(rootChain) {
					return nil, fmt.Errorf("cfb: mini sector %d of %q outside the mini stream", mini, ent.Name)
				}

				off, err := rdr.sectorOffset(rootChain[pos/rdr.ss])
				if err != nil {
					return nil, err
				}

				off += pos % rdr.ss
				ent.Data = append(ent.Data, raw[off:off+miniSectorSize]...)
				mini = rdr.miniFAT[mini]
			}

			ent.Data = ent.Data[:ent.Size]
		default:
			chain, err := rdr.chain(ent.Start, int(ent.Size)/rdr.ss+2) //nolint:gosec // sizes were checked against the file length
			if err != nil {
				return nil, fmt.Errorf("cfb: stream %q: %w", ent.Name, err)
			}

			if uint64(len(chain))*uint64(rdr.ss) < ent.Size { //nolint:gosec // both are non-negative
				return nil, fmt.Errorf("cfb: stream %q is shorter than its size", ent.Name)
			}

			ent.Data = make([]byte, 0, ent.Size)

			for _, s := range chain {
				off, err := rdr.sectorOffset(s)
				if err != nil {
					return nil, err
				}

				ent.Data = append(ent.Data, raw[off:off+rdr.ss]...)
			}

			ent.Data = ent.Data[:ent.Size]
		}

		if ent.Parent == 0 {
			// Siblings are compared ignoring ASCII case, like Write does.
			key := foldName(ent.Name)
			if folded[key] {
				return nil, fmt.Errorf("cfb: duplicate stream name %q", ent.Name)
			}

			folded[key] = true

			file.byName[ent.Name] = ent
			file.streams = append(file.streams, Stream{Name: ent.Name, Data: ent.Data})
		}
	}

	return file, nil
}

// foldName is name with ASCII letters in upper case, the key under which
// Compare treats two names as equal.
func foldName(name string) string {
	units := utf16.Encode([]rune(name))
	for i, unit := range units {
		units[i] = upper(unit)
	}

	return string(utf16.Decode(units))
}
