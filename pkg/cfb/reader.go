package cfb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
)

// File is a parsed compound file. Read loads every stream into Entry.Data;
// Open leaves the data in the underlying reader and serves it on demand
// through Stream, Streams and Reader.
type File struct {
	byName     map[string]*Entry
	rdr        *reader
	Entries    []*Entry
	rootChain  []uint32
	Version    int
	SectorSize int
	CLSID      [16]byte
}

// Entry is one directory entry. Data is set by Read, and by Stream or
// Streams on an opened file once they load it.
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
	loaded    bool
}

// Streams returns the root-level streams in SID order, loading any that are
// not in memory yet. A stream whose chain cannot be read is left out; Reader
// reports the error for a single stream.
func (f *File) Streams() []Stream {
	out := make([]Stream, 0, len(f.byName))

	for _, ent := range f.Entries {
		if ent.Type != 2 || ent.Parent != 0 {
			continue
		}

		data, err := f.load(ent)
		if err != nil {
			continue
		}

		out = append(out, Stream{Name: ent.Name, Data: data})
	}

	return out
}

// Stream returns the data of the root-level stream with the given name,
// loading it if necessary.
func (f *File) Stream(name string) ([]byte, bool) {
	ent, ok := f.byName[name]
	if !ok {
		return nil, false
	}

	data, err := f.load(ent)
	if err != nil {
		return nil, false
	}

	return data, true
}

// Reader returns a reader over the bytes of a root-level stream that reads
// from the underlying file on demand, so a large stream never has to be in
// memory.
func (f *File) Reader(name string) (*StreamReader, error) {
	ent, ok := f.byName[name]
	if !ok {
		return nil, fmt.Errorf("cfb: no stream %q", name)
	}

	return f.readerFor(ent)
}

// readerFor builds the reader of any stream entry, root-level or nested.
func (f *File) readerFor(ent *Entry) (*StreamReader, error) {
	name := ent.Name

	if ent.loaded {
		return &StreamReader{data: ent.Data, size: int64(ent.Size)}, nil //nolint:gosec // checked against the file size
	}

	stream := &StreamReader{file: f, size: int64(ent.Size)} //nolint:gosec // checked against the file size

	switch {
	case ent.Size == 0:
	case ent.Size < miniCutoff:
		chain, err := f.miniChain(ent)
		if err != nil {
			return nil, err
		}

		stream.unit, stream.chain = miniSectorSize, chain
	default:
		chain, err := f.rdr.chain(ent.Start, int(ent.Size)/f.SectorSize+2) //nolint:gosec // sizes were checked against the file length
		if err != nil {
			return nil, fmt.Errorf("cfb: stream %q: %w", name, err)
		}

		if uint64(len(chain))*uint64(f.SectorSize) < ent.Size { //nolint:gosec // both are non-negative
			return nil, fmt.Errorf("cfb: stream %q is shorter than its size", name)
		}

		stream.unit, stream.chain = f.SectorSize, chain
	}

	return stream, nil
}

// load returns the entry's data, reading it once.
func (f *File) load(ent *Entry) ([]byte, error) {
	if ent.loaded {
		return ent.Data, nil
	}

	stream, err := f.readerFor(ent)
	if err != nil {
		return nil, err
	}

	data := make([]byte, ent.Size)

	if _, err := stream.ReadAt(data, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	ent.Data, ent.loaded = data, true

	return data, nil
}

// miniChain follows the MiniFAT chain of a small stream.
func (f *File) miniChain(ent *Entry) ([]uint32, error) {
	var chain []uint32

	for mini, n := ent.Start, uint64(0); n*miniSectorSize < ent.Size; n++ {
		if mini == endOfChain || int(mini) >= len(f.rdr.miniFAT) || mini >= 0xFFFFFFFA {
			return nil, fmt.Errorf("cfb: bad MiniFAT chain for %q", ent.Name)
		}

		if int(mini)*miniSectorSize/f.SectorSize >= len(f.rootChain) {
			return nil, fmt.Errorf("cfb: mini sector %d of %q outside the mini stream", mini, ent.Name)
		}

		chain = append(chain, mini)
		mini = f.rdr.miniFAT[mini]
	}

	return chain, nil
}

// link is one pending step of walkTree: an entry and the storage it is in.
type link struct {
	sid    uint32
	parent int
}

// walkTree sets Parent on every entry reachable from the root, rejecting
// links that point outside the directory or form a cycle. It keeps its own
// stack, so a degenerate directory cannot exhaust the goroutine stack.
func (f *File) walkTree() error {
	visited := map[uint32]bool{0: true}
	stack := []link{{sid: f.Entries[0].Child, parent: 0}}

	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if top.sid == noStream {
			continue
		}

		if int(top.sid) >= len(f.Entries) || visited[top.sid] {
			return fmt.Errorf("cfb: bad sibling link %d", top.sid)
		}

		visited[top.sid] = true
		ent := f.Entries[top.sid]
		ent.Parent = top.parent

		stack = append(stack, link{sid: ent.Left, parent: top.parent}, link{sid: ent.Right, parent: top.parent})

		if ent.Type == 1 {
			stack = append(stack, link{sid: ent.Child, parent: int(top.sid)})
		}
	}

	return nil
}

// StreamReader reads one stream of an opened file. It implements io.Reader,
// io.ReaderAt and io.Seeker.
type StreamReader struct {
	file  *File
	data  []byte // set when the stream is already in memory
	chain []uint32
	size  int64
	pos   int64
	unit  int
}

// Size returns the stream length.
func (s *StreamReader) Size() int64 { return s.size }

// ReadAt reads from the stream at off. It returns io.EOF only when buf
// reaches past the end of the stream.
func (s *StreamReader) ReadAt(buf []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("cfb: negative offset")
	}

	if off >= s.size {
		return 0, io.EOF
	}

	short := false

	if int64(len(buf)) > s.size-off {
		buf, short = buf[:s.size-off], true
	}

	var (
		total int
		err   error
	)

	if s.data != nil {
		total = copy(buf, s.data[off:])
	} else {
		total, err = s.readChain(buf, off)
	}

	if err == nil && short {
		err = io.EOF
	}

	return total, err
}

// Read reads from the current position.
func (s *StreamReader) Read(p []byte) (int, error) {
	n, err := s.ReadAt(p, s.pos)
	s.pos += int64(n)

	return n, err
}

// Seek sets the position for Read.
func (s *StreamReader) Seek(offset int64, whence int) (int64, error) {
	var pos int64

	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = s.pos + offset
	case io.SeekEnd:
		pos = s.size + offset
	default:
		return 0, fmt.Errorf("cfb: whence %d", whence)
	}

	if pos < 0 {
		return 0, errors.New("cfb: negative position")
	}

	s.pos = pos

	return pos, nil
}

// readChain fills buf from the stream's sectors starting at off.
func (s *StreamReader) readChain(buf []byte, off int64) (int, error) {
	total := 0

	for total < len(buf) {
		cur := off + int64(total)

		fileOff, err := s.offset(cur)
		if err != nil {
			return total, err
		}

		want := min(len(buf)-total, s.unit-int(cur%int64(s.unit)))

		read, err := s.file.rdr.r.ReadAt(buf[total:total+want], fileOff)
		total += read

		if err != nil && (!errors.Is(err, io.EOF) || read != want) {
			return total, fmt.Errorf("cfb: read: %w", err)
		}
	}

	return total, nil
}

// offset returns the file offset of the stream byte at off.
func (s *StreamReader) offset(off int64) (int64, error) {
	idx := off / int64(s.unit)
	if idx >= int64(len(s.chain)) {
		return 0, errors.New("cfb: stream chain is too short")
	}

	within := off % int64(s.unit)

	if s.unit == miniSectorSize {
		pos := int64(s.chain[idx]) * miniSectorSize

		sector, err := s.file.rdr.sectorOffset(s.file.rootChain[pos/int64(s.file.SectorSize)])
		if err != nil {
			return 0, err
		}

		return sector + pos%int64(s.file.SectorSize) + within, nil
	}

	sector, err := s.file.rdr.sectorOffset(s.chain[idx])
	if err != nil {
		return 0, err
	}

	return sector + within, nil
}

type reader struct {
	r       io.ReaderAt
	fat     []uint32
	miniFAT []uint32
	size    int64
	ss      int
}

func (r *reader) sectorOffset(n uint32) (int64, error) {
	off := (int64(n) + 1) * int64(r.ss)
	if n >= 0xFFFFFFFA || off+int64(r.ss) > r.size {
		return 0, fmt.Errorf("cfb: sector %#x out of range", n)
	}

	return off, nil
}

// sector reads one sector.
func (r *reader) sector(index uint32) ([]byte, error) {
	off, err := r.sectorOffset(index)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, r.ss)
	if _, err := r.r.ReadAt(buf, off); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("cfb: read sector %d: %w", index, err)
	}

	return buf, nil
}

// words reads one sector as little-endian 32-bit values.
func (r *reader) words(index uint32) ([]uint32, error) {
	buf, err := r.sector(index)
	if err != nil {
		return nil, err
	}

	out := make([]uint32, r.ss/4)
	for i := range out {
		out[i] = binary.LittleEndian.Uint32(buf[4*i:])
	}

	return out, nil
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
func Read(raw []byte) (*File, error) {
	file, err := Open(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}

	for _, ent := range file.Entries {
		if ent.Type != 2 || ent.Parent < 0 {
			continue
		}

		if _, err := file.load(ent); err != nil {
			return nil, err
		}
	}

	return file, nil
}

// Open parses the structure of a compound file of the given size without
// loading its streams. Stream, Streams and Reader read them from r.
func Open(src io.ReaderAt, size int64) (*File, error) { //nolint:maintidx,cyclop,gocyclo,gocognit // one pass over the format; splitting would scatter it
	raw := make([]byte, 512)

	if size < 512 {
		return nil, errors.New("cfb: bad signature")
	}

	if _, err := src.ReadAt(raw, 0); err != nil {
		return nil, fmt.Errorf("cfb: header: %w", err)
	}

	if !bytes.Equal(raw[:8], signature) {
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

	rdr := &reader{r: src, size: size, ss: 1 << shift}
	file := &File{Version: major, SectorSize: rdr.ss, byName: make(map[string]*Entry), rdr: rdr}
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

		words, err := rdr.words(difat)
		if err != nil {
			return nil, fmt.Errorf("cfb: DIFAT: %w", err)
		}

		for i := 0; i < rdr.ss/4-1 && uint32(len(fatSectors)) < numFAT; i++ { //nolint:gosec // bounded by the format limits checked earlier
			fatSectors = append(fatSectors, words[i])
		}

		difat = words[rdr.ss/4-1]
	}

	for _, s := range fatSectors {
		words, err := rdr.words(s)
		if err != nil {
			return nil, fmt.Errorf("cfb: FAT: %w", err)
		}

		rdr.fat = append(rdr.fat, words...)
	}

	dirChain, err := rdr.chain(firstDir, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("cfb: directory: %w", err)
	}

	for _, s := range dirChain {
		sector, err := rdr.sector(s)
		if err != nil {
			return nil, fmt.Errorf("cfb: directory: %w", err)
		}

		for i := range rdr.ss / entrySize {
			rec := sector[entrySize*i : entrySize*(i+1)]
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
			words, err := rdr.words(s)
			if err != nil {
				return nil, fmt.Errorf("cfb: MiniFAT: %w", err)
			}

			rdr.miniFAT = append(rdr.miniFAT, words...)
		}
	}

	if root.Size > 0 {
		file.rootChain, err = rdr.chain(root.Start, 1<<22)
		if err != nil {
			return nil, fmt.Errorf("cfb: mini stream: %w", err)
		}
	}

	if err := file.walkTree(); err != nil {
		return nil, err
	}

	for _, ent := range file.Entries {
		if ent.Type != 2 || ent.Parent < 0 {
			continue
		}

		if ent.Size > uint64(size) {
			return nil, fmt.Errorf("cfb: stream %q larger than the file", ent.Name)
		}

		if ent.Parent == 0 {
			// Siblings are compared ignoring ASCII case, like Write does.
			key := foldName(ent.Name)
			if folded[key] {
				return nil, fmt.Errorf("cfb: duplicate stream name %q", ent.Name)
			}

			folded[key] = true

			file.byName[ent.Name] = ent
		}

		// Check the chain now so that a corrupt file fails at Open.
		if _, err := file.readerFor(ent); err != nil {
			return nil, err
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
