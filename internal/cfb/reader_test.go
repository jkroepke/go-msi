package cfb //nolint:testpackage // the tests patch FAT, MiniFAT and directory entries

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
)

// layout finds the parts of a written file that the tests below patch.
type layout struct {
	file *File
	raw  []byte
}

func parse(t *testing.T, raw []byte) layout {
	t.Helper()

	file, err := Read(raw)
	if err != nil {
		t.Fatal(err)
	}

	return layout{file: file, raw: raw}
}

// entry returns the file offset of the directory entry with the given name.
func (l layout) entry(t *testing.T, name string) int {
	t.Helper()

	dir, err := l.file.rdr.chain(readU32(l.raw, 0x30), 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	for _, ent := range l.file.Entries {
		if ent.Name == name {
			pos := ent.SID * entrySize

			return (int(dir[pos/l.file.SectorSize])+1)*l.file.SectorSize + pos%l.file.SectorSize
		}
	}

	t.Fatalf("no entry %q", name)

	return 0
}

func (l layout) start(t *testing.T, name string) uint32 {
	t.Helper()

	return readU32(l.raw, l.entry(t, name)+0x74)
}

// fat returns the file offset of the FAT entry of a sector; the files here
// have no more than 109 FAT sectors.
func (l layout) fat(sector uint32) int {
	per := uint32(l.file.SectorSize / 4) //nolint:gosec // 128 or 1024

	return (int(readU32(l.raw, 0x4C+4*int(sector/per)))+1)*l.file.SectorSize + 4*int(sector%per)
}

// miniFAT returns the file offset of the MiniFAT entry of a mini sector.
func (l layout) miniFAT(t *testing.T, mini uint32) int {
	t.Helper()

	chain, err := l.file.rdr.chain(readU32(l.raw, 0x3C), 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	per := uint32(l.file.SectorSize / 4) //nolint:gosec // 128 or 1024

	return (int(chain[mini/per])+1)*l.file.SectorSize + 4*int(mini%per)
}

func put32(raw []byte, off int, value uint32) { binary.LittleEndian.PutUint32(raw[off:], value) }

// wantError checks that both Read and Open reject raw with an error that
// mentions want.
func wantError(t *testing.T, name string, raw []byte, want string) {
	t.Helper()

	if _, err := Read(raw); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s: Read = %v, want an error about %q", name, err, want)
	}

	if _, err := Open(bytes.NewReader(raw), int64(len(raw))); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s: Open = %v, want an error about %q", name, err, want)
	}
}

// TestRepeatedFATSectors: each FAT sector is read once, so a small file
// cannot list one sector many times and expand it into a large FAT.
func TestRepeatedFATSectors(t *testing.T) {
	t.Parallel()

	// An empty v4 file with one DIFAT sector appended whose 1023 entries,
	// like the 109 in the header, all name FAT sector 0: 1132 FAT sectors
	// would expand 16 KB into a FAT of 4.6 MB.
	empty, err := Write(nil, Options{Version: 4})
	if err != nil {
		t.Fatal(err)
	}

	amplified := append(bytes.Clone(empty), make([]byte, 4096)...)
	difat := uint32(len(empty)/4096 - 1) //nolint:gosec // a few sectors

	for i := range 109 {
		put32(amplified, 0x4C+4*i, 0)
	}

	for i := range 1023 {
		put32(amplified, len(empty)+4*i, 0)
	}

	put32(amplified, len(empty)+4*1023, endOfChain)
	put32(amplified, 0x2C, 1132)
	put32(amplified, 0x44, difat)
	put32(amplified, 0x48, 1)
	wantError(t, "amplified", amplified, "FAT sectors in a file of")

	// A v3 file of more than 110 sectors, so that the FAT count alone is
	// plausible and the DIFAT is needed.
	out, err := Write([]Stream{{Name: "big", Data: make([]byte, 120*512)}}, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	if readU32(out, 0x2C) != 1 {
		t.Fatalf("the test file has %d FAT sectors, want 1", readU32(out, 0x2C))
	}

	fat := readU32(out, 0x4C)
	physical := uint32(len(out)/512 - 1) //nolint:gosec // about 123

	// spare lists the sectors other than the FAT sector, all inside the
	// file: 108 for the header, then the DIFAT sector and its entries.
	spare := make([]uint32, 0, physical)

	for id := range physical {
		if id != fat {
			spare = append(spare, id)
		}
	}

	// header lists numFAT sectors in the header: the real FAT sector first,
	// then distinct sectors of the stream.
	header := func(numFAT uint32) []byte {
		raw := bytes.Clone(out)
		put32(raw, 0x2C, numFAT)

		for i := 1; i < 109 && uint32(i) < numFAT; i++ {
			put32(raw, 0x4C+4*i, spare[i-1])
		}

		return raw
	}

	raw := header(2)
	put32(raw, 0x4C+4, fat)
	wantError(t, "repeated header ID", raw, "repeated FAT sector")

	// withDIFAT adds a DIFAT sector holding the given entries after the
	// 109 in the header.
	extra := spare[108]

	withDIFAT := func(entries ...uint32) []byte {
		raw := header(109 + uint32(len(entries))) //nolint:gosec // two entries
		put32(raw, 0x44, extra)
		put32(raw, 0x48, 1)

		for i := range 127 {
			put32(raw, (int(extra)+1)*512+4*i, freeSect)
		}

		for i, id := range entries {
			put32(raw, (int(extra)+1)*512+4*i, id)
		}

		put32(raw, (int(extra)+1)*512+4*127, endOfChain)

		return raw
	}

	wantError(t, "repeated DIFAT ID", withDIFAT(spare[109], spare[109]), "repeated FAT sector")
	wantError(t, "DIFAT repeats a header ID", withDIFAT(spare[109], fat), "repeated FAT sector")
	wantError(t, "DIFAT lists itself", withDIFAT(spare[109], extra), "repeated FAT sector")

	// The header promises more FAT sectors than it and the DIFAT list.
	missing := header(110)
	put32(missing, 0x44, endOfChain)
	wantError(t, "missing IDs", missing, "list 109 of 110 FAT sectors")

	raw = header(2)
	put32(raw, 0x4C+4, freeSect)
	wantError(t, "free ID", raw, "bad or repeated FAT sector")
}

// TestSharedChains: two chains never share a sector, so the streams of a
// file never load more bytes than the file holds (MSISPEC section 2).
func TestSharedChains(t *testing.T) {
	t.Parallel()

	streams := []Stream{
		{Name: "A", Data: bytes.Repeat([]byte("A"), 16*512)},
		{Name: "B", Data: bytes.Repeat([]byte("B"), 9*512)},
		{Name: "c", Data: bytes.Repeat([]byte("c"), 128)},
		{Name: "d", Data: bytes.Repeat([]byte("d"), 128)},
	}

	out, err := Write(streams, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	base := parse(t, out)

	// B starts where A starts and has A's size.
	raw := bytes.Clone(out)
	put32(raw, base.entry(t, "B")+0x74, base.start(t, "A"))
	put32(raw, base.entry(t, "B")+0x78, 16*512)
	wantError(t, "same start", raw, "in two chains")

	// B's first sector links into A's ninth: both chains end in A's tail.
	raw = bytes.Clone(out)
	put32(raw, base.fat(base.start(t, "B")), base.start(t, "A")+8)
	wantError(t, "merged chains", raw, "in two chains")

	// The same for two streams in the mini stream.
	raw = bytes.Clone(out)
	put32(raw, base.entry(t, "d")+0x74, base.start(t, "c"))
	wantError(t, "same mini start", raw, "in two chains")

	raw = bytes.Clone(out)
	put32(raw, base.miniFAT(t, base.start(t, "d")), base.start(t, "c")+1)
	wantError(t, "merged mini chains", raw, "in two chains")

	// 64 streams of 128 KiB that all start at the first stream's sector
	// load 8 MiB from a file of 160 KB.
	many := make([]Stream, 64)
	for i := range many {
		many[i] = Stream{Name: string(rune('A'+i/26)) + string(rune('a'+i%26)), Data: make([]byte, 128<<10)}
	}

	out, err = Write(many, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	base = parse(t, out)
	first := base.start(t, many[0].Name)

	for _, s := range many[1:] {
		put32(out, base.entry(t, s.Name)+0x74, first)
	}

	wantError(t, "64 streams on one chain", out, "in two chains")
}

// TestMiniChainEnds: a small stream's MiniFAT chain visits each mini sector
// once and ends right after the sectors its size needs.
func TestMiniChainEnds(t *testing.T) {
	t.Parallel()

	data := append(bytes.Repeat([]byte("A"), 64), bytes.Repeat([]byte("B"), 64)...)

	out, err := Write([]Stream{
		{Name: "s", Data: data},
		{Name: "t", Data: bytes.Repeat([]byte("C"), 192)},
		{Name: "u", Data: []byte("u")},
	}, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	base := parse(t, out)
	first, second, third := base.start(t, "s"), base.start(t, "t"), base.start(t, "u")

	for name, patch := range map[string][2]uint32{
		"self loop":       {first, first},         // 128 bytes of A before the fix
		"two-sector loop": {second + 1, second},   // t: 0 → 1 → 0
		"extra tail":      {first + 1, third},     // s ends in u's sector
		"free tail":       {second + 2, freeSect}, // t ends in nothing
	} {
		raw := bytes.Clone(out)
		put32(raw, base.miniFAT(t, patch[0]), patch[1])

		want := "bad MiniFAT chain"
		if name == "extra tail" || name == "free tail" {
			want = "does not end after its size"
		}

		wantError(t, name, raw, want)
	}

	// An empty stream still accepts any start (TestEmptyStreamAnyStartSector).
	if _, err := Read(out); err != nil {
		t.Fatal(err)
	}
}

// TestRootMiniStreamSize: small streams are read only from inside the root
// entry's declared mini stream, and the root's chain holds that size.
func TestRootMiniStreamSize(t *testing.T) {
	t.Parallel()

	out, err := Write([]Stream{{Name: "s", Data: bytes.Repeat([]byte("x"), 128)}}, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	base := parse(t, out)
	root := base.entry(t, "Root Entry")

	if size := readU32(out, root+0x78); size != 128 {
		t.Fatalf("root size %d, want 128", size)
	}

	// The root ends inside its only regular sector, before s's second
	// mini sector.
	raw := bytes.Clone(out)
	put32(raw, root+0x78, 64)
	wantError(t, "root ends in its sector", raw, "outside the mini stream")

	// The root claims more than its one-sector chain holds.
	raw = bytes.Clone(out)
	put32(raw, root+0x78, 513)
	wantError(t, "root longer than its chain", raw, "mini stream is shorter than its size")

	// A stream that ends inside a mini sector needs only its bytes in the
	// mini stream.
	out, err = Write([]Stream{{Name: "s", Data: bytes.Repeat([]byte("x"), 100)}}, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	raw = bytes.Clone(out)
	put32(raw, root+0x78, 100)

	if f, err := Read(raw); err != nil {
		t.Errorf("root of the stream's exact size: %v", err)
	} else if got, _ := f.Stream("s"); len(got) != 100 {
		t.Errorf("read %d bytes, want 100", len(got))
	}

	put32(raw, root+0x78, 99)
	wantError(t, "root one byte short", raw, "outside the mini stream")
}

// eofAt returns io.EOF with every read that reaches the end of its data,
// complete or not, as io.ReaderAt allows.
type eofAt struct{ data []byte }

func (r eofAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}

	n := copy(p, r.data[off:])
	if off+int64(n) == int64(len(r.data)) {
		return n, io.EOF
	}

	return n, nil
}

// TestShortMetadataRead: a reader that holds less than the size given to
// Open fails with io.ErrUnexpectedEOF instead of zero-filling a sector.
func TestShortMetadataRead(t *testing.T) {
	t.Parallel()

	out, err := Write(nil, Options{Version: 4})
	if err != nil {
		t.Fatal(err)
	}

	dir := (int(readU32(out, 0x30)) + 1) * 4096

	if _, err := Open(bytes.NewReader(out[:dir+100]), int64(len(out))); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("Open of a short reader: %v, want io.ErrUnexpectedEOF", err)
	}

	// io.EOF with a complete read is the end of the data, not an error.
	if _, err := Open(eofAt{data: out[:dir+4096]}, int64(dir+4096)); err != nil {
		t.Errorf("Open with io.EOF on the last sector: %v", err)
	}
}

// TestSiblingNames: names are unique, ignoring ASCII case, among the
// children of every storage, streams and storages alike.
func TestSiblingNames(t *testing.T) {
	t.Parallel()

	src := func(name string) Source { return Stream{Name: name, Data: []byte("data")}.Source() }

	write := func(sources []Source, storages []Storage) []byte {
		var buf bytes.Buffer
		if _, err := WriteStoragesTo(&buf, sources, storages, Options{Version: 3}); err != nil {
			t.Fatal(err)
		}

		return buf.Bytes()
	}

	rename := func(raw []byte) []byte {
		name := []byte{'B', 0, 'b', 0}
		if bytes.Count(raw, name) != 1 {
			t.Fatal("cannot find the directory entry")
		}

		return bytes.Replace(raw, name, []byte{'A', 0, 'A', 0}, 1)
	}

	for name, raw := range map[string][]byte{
		"streams in a storage": write(nil, []Storage{{Name: "S", Streams: []Source{src("Aa"), src("Bb")}}}),
		"stream and storage":   write([]Source{src("Aa")}, []Storage{{Name: "Bb", Streams: []Source{src("x")}}}),
		"two storages": write(nil, []Storage{
			{Name: "Aa", Streams: []Source{src("x")}}, {Name: "Bb", Streams: []Source{src("x")}},
		}),
	} {
		parse(t, raw)
		wantError(t, name, rename(raw), "duplicate name")
	}

	// Names that differ only in a lone surrogate are different names,
	// although both decode to U+FFFD.
	raw := write([]Source{src("x\U0001F600"), src("y\U0001F600")}, nil)
	high, low := []byte{0x3D, 0xD8}, []byte{0x00, 0xDE}

	if bytes.Count(raw, append([]byte{'y', 0}, high...)) != 1 {
		t.Fatal("cannot find the directory entry")
	}

	raw = bytes.Replace(raw, append(append([]byte{'x', 0}, high...), low...), append(append([]byte{'z', 0}, high...), 'a', 0), 1)
	raw = bytes.Replace(raw, append(append([]byte{'y', 0}, high...), low...), append(append([]byte{'z', 0}, low...), 'a', 0), 1)

	if _, err := Read(raw); err != nil {
		t.Errorf("names with different lone surrogates: %v", err)
	}

	// The same name under different parents is fine.
	raw = write([]Source{src("x"), src("S2")}, []Storage{{Name: "S", Streams: []Source{src("x"), src("S2")}}})
	if _, err := Read(raw); err != nil {
		t.Errorf("one name in two storages: %v", err)
	}
}

// TestHugeSizeAllocatesLittle: the size given to Open may be far larger than
// the data, as for a file cut short after it was measured. A FAT count that
// fits that size must not allocate for it before its sectors are read.
func TestHugeSizeAllocatesLittle(t *testing.T) { //nolint:paralleltest // measures the allocations of the whole process
	out, err := Write(nil, Options{Version: 4})
	if err != nil {
		t.Fatal(err)
	}

	put32(out, 0x2C, 1<<26)

	var stats runtime.MemStats

	runtime.ReadMemStats(&stats)
	before := stats.TotalAlloc

	if _, err := Open(bytes.NewReader(out), 1<<40); err == nil {
		t.Error("Open accepted 2^26 FAT sectors that the data does not hold")
	}

	runtime.ReadMemStats(&stats)

	if grown := stats.TotalAlloc - before; grown > 1<<20 {
		t.Errorf("Open allocated %d bytes for a FAT count it had not read", grown)
	}
}
