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
	// Version is 3 or 4.
	Version int
	// SectorSize is 512 or 4096.
	SectorSize int
	// CLSID of the root storage.
	CLSID [16]byte
	// Entries are the directory entries in SID order, unused ones included.
	Entries []*Entry

	streams []Stream
	byName  map[string]*Entry
}

// Entry is one directory entry.
type Entry struct {
	SID    int
	Name   string
	Type   byte // 0 unused, 1 storage, 2 stream, 5 root
	Colour byte
	Left   uint32
	Right  uint32
	Child  uint32
	CLSID  [16]byte
	Start  uint32
	Size   uint64
	// Parent is the SID of the containing storage, or -1 when the entry is
	// not reachable from the root.
	Parent int
	// Data holds the stream's bytes for streams reachable from the root.
	Data []byte
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
	ss      int
	fat     []uint32
	miniFAT []uint32
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

	seen := map[uint32]bool{}

	for s := start; s != endOfChain; {
		if s >= 0xFFFFFFFA || int(s) >= len(r.fat) || seen[s] {
			return nil, fmt.Errorf("cfb: bad FAT chain at %#x", s)
		}

		seen[s] = true
		out = append(out, s)

		if len(out) > limit {
			return nil, errors.New("cfb: chain too long")
		}

		s = r.fat[s]
	}

	return out, nil
}

// Read parses a compound file and loads every root-level stream.
func Read(raw []byte) (*File, error) {
	if len(raw) < 512 || !bytes.Equal(raw[:8], signature) {
		return nil, errors.New("cfb: bad signature")
	}

	le := binary.LittleEndian

	if le.Uint16(raw[0x1C:]) != 0xFFFE {
		return nil, errors.New("cfb: bad byte order mark")
	}

	major := int(le.Uint16(raw[0x1A:]))
	shift := le.Uint16(raw[0x1E:])

	if !(major == 3 && shift == 9) && !(major == 4 && shift == 12) {
		return nil, fmt.Errorf("cfb: unsupported version %d with sector shift %d", major, shift)
	}

	if le.Uint16(raw[0x20:]) != 6 {
		return nil, errors.New("cfb: unsupported mini sector shift")
	}

	r := &reader{raw: raw, ss: 1 << shift}
	f := &File{Version: major, SectorSize: r.ss, byName: map[string]*Entry{}}

	numFAT := le.Uint32(raw[0x2C:])
	firstDir := le.Uint32(raw[0x30:])
	firstMiniFAT := le.Uint32(raw[0x3C:])
	firstDIFAT := le.Uint32(raw[0x44:])

	var fatSectors []uint32

	for i := 0; i < 109 && uint32(len(fatSectors)) < numFAT; i++ {
		fatSectors = append(fatSectors, le.Uint32(raw[0x4C+4*i:]))
	}

	seenDIFAT := map[uint32]bool{}

	for d := firstDIFAT; d != endOfChain && d != freeSect && uint32(len(fatSectors)) < numFAT; {
		if seenDIFAT[d] {
			return nil, errors.New("cfb: DIFAT loop")
		}

		seenDIFAT[d] = true

		off, err := r.sectorOffset(d)
		if err != nil {
			return nil, fmt.Errorf("cfb: DIFAT: %w", err)
		}

		for i := 0; i < r.ss/4-1 && uint32(len(fatSectors)) < numFAT; i++ {
			fatSectors = append(fatSectors, le.Uint32(raw[off+4*i:]))
		}

		d = le.Uint32(raw[off+r.ss-4:])
	}

	for _, s := range fatSectors {
		off, err := r.sectorOffset(s)
		if err != nil {
			return nil, fmt.Errorf("cfb: FAT: %w", err)
		}

		for i := 0; i < r.ss/4; i++ {
			r.fat = append(r.fat, le.Uint32(raw[off+4*i:]))
		}
	}

	dirChain, err := r.chain(firstDir, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("cfb: directory: %w", err)
	}

	for _, s := range dirChain {
		off, err := r.sectorOffset(s)
		if err != nil {
			return nil, fmt.Errorf("cfb: directory: %w", err)
		}

		for i := 0; i < r.ss/entrySize; i++ {
			e := raw[off+entrySize*i : off+entrySize*(i+1)]
			nameLen := int(le.Uint16(e[0x40:]))

			ent := &Entry{
				SID:    len(f.Entries),
				Type:   e[0x42],
				Colour: e[0x43],
				Left:   le.Uint32(e[0x44:]),
				Right:  le.Uint32(e[0x48:]),
				Child:  le.Uint32(e[0x4C:]),
				Start:  le.Uint32(e[0x74:]),
				Size:   le.Uint64(e[0x78:]),
				Parent: -1,
			}

			if major == 3 {
				ent.Size &= 0xFFFFFFFF
			}

			copy(ent.CLSID[:], e[0x50:0x60])

			if nameLen >= 2 && nameLen <= 64 {
				units := make([]uint16, 0, nameLen/2-1)
				for j := 0; j < nameLen/2-1; j++ {
					units = append(units, le.Uint16(e[2*j:]))
				}

				ent.Name = string(utf16.Decode(units))
			}

			f.Entries = append(f.Entries, ent)
		}
	}

	if len(f.Entries) == 0 || f.Entries[0].Type != 5 {
		return nil, errors.New("cfb: no root entry")
	}

	root := f.Entries[0]
	f.CLSID = root.CLSID

	if firstMiniFAT != endOfChain && firstMiniFAT != freeSect {
		mc, err := r.chain(firstMiniFAT, 1<<20)
		if err != nil {
			return nil, fmt.Errorf("cfb: MiniFAT: %w", err)
		}

		for _, s := range mc {
			off, err := r.sectorOffset(s)
			if err != nil {
				return nil, fmt.Errorf("cfb: MiniFAT: %w", err)
			}

			for i := 0; i < r.ss/4; i++ {
				r.miniFAT = append(r.miniFAT, le.Uint32(raw[off+4*i:]))
			}
		}
	}

	var rootChain []uint32

	if root.Size > 0 {
		rootChain, err = r.chain(root.Start, 1<<22)
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

		if int(sid) >= len(f.Entries) || visited[sid] {
			return fmt.Errorf("cfb: bad sibling link %d", sid)
		}

		visited[sid] = true
		e := f.Entries[sid]
		e.Parent = parent

		if err := walk(e.Left, parent); err != nil {
			return err
		}

		if err := walk(e.Right, parent); err != nil {
			return err
		}

		if e.Type == 1 {
			return walk(e.Child, int(sid))
		}

		return nil
	}

	if err := walk(root.Child, 0); err != nil {
		return nil, err
	}

	for _, e := range f.Entries {
		if e.Type != 2 || e.Parent < 0 {
			continue
		}

		if e.Size > uint64(len(raw)) {
			return nil, fmt.Errorf("cfb: stream %q larger than the file", e.Name)
		}

		switch {
		case e.Size == 0:
			// Any start sector is fine for an empty stream.
		case e.Size < miniCutoff:
			e.Data = make([]byte, 0, e.Size)

			for m, n := e.Start, uint64(0); n*miniSectorSize < e.Size; n++ {
				if m == endOfChain || int(m) >= len(r.miniFAT) || m >= 0xFFFFFFFA {
					return nil, fmt.Errorf("cfb: bad MiniFAT chain for %q", e.Name)
				}

				pos := int(m) * miniSectorSize
				if pos/r.ss >= len(rootChain) {
					return nil, fmt.Errorf("cfb: mini sector %d of %q outside the mini stream", m, e.Name)
				}

				off, err := r.sectorOffset(rootChain[pos/r.ss])
				if err != nil {
					return nil, err
				}

				off += pos % r.ss
				e.Data = append(e.Data, raw[off:off+miniSectorSize]...)
				m = r.miniFAT[m]
			}

			e.Data = e.Data[:e.Size]
		default:
			c, err := r.chain(e.Start, int(e.Size)/r.ss+2)
			if err != nil {
				return nil, fmt.Errorf("cfb: stream %q: %w", e.Name, err)
			}

			if uint64(len(c))*uint64(r.ss) < e.Size {
				return nil, fmt.Errorf("cfb: stream %q is shorter than its size", e.Name)
			}

			e.Data = make([]byte, 0, e.Size)

			for _, s := range c {
				off, err := r.sectorOffset(s)
				if err != nil {
					return nil, err
				}

				e.Data = append(e.Data, raw[off:off+r.ss]...)
			}

			e.Data = e.Data[:e.Size]
		}

		if e.Parent == 0 {
			if _, dup := f.byName[e.Name]; dup {
				return nil, fmt.Errorf("cfb: duplicate stream name %q", e.Name)
			}

			f.byName[e.Name] = e
			f.streams = append(f.streams, Stream{Name: e.Name, Data: e.Data})
		}
	}

	return f, nil
}
