package cfb //nolint:testpackage // the checker reads the format constants

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"unicode/utf16"
)

// conformance checks a compound file against the MUST and SHOULD rules of
// [MS-CFB] sections 2.1 to 2.9 that a writer controls, independently of
// Open: header fields, the FAT, DIFAT and MiniFAT, sector ownership, the
// directory entries, the red-black trees and zeroed padding.
type conformance struct {
	raw      []byte
	fat      []uint32
	miniFAT  []uint32
	owner    []string
	errs     []string
	ss       int
	sectors  int
	balanced bool // also require equal black heights
}

func (c *conformance) errorf(format string, args ...any) {
	c.errs = append(c.errs, fmt.Sprintf(format, args...))
}

func (c *conformance) u16(off int) uint16 { return binary.LittleEndian.Uint16(c.raw[off:]) }
func (c *conformance) u32(off int) uint32 { return binary.LittleEndian.Uint32(c.raw[off:]) }

func (c *conformance) sector(n uint32) []byte {
	off := (int(n) + 1) * c.ss

	return c.raw[off : off+c.ss]
}

// checkConformance returns the rule violations in raw. MS-CFB 2.6.4 lists
// only three tree rules and allows an all-black tree, which is a plain
// binary tree with unequal black heights; balanced adds the black-height
// rule of a real red-black tree, which the default writer keeps.
func checkConformance(raw []byte, balanced bool) []string {
	chk := &conformance{raw: raw, balanced: balanced}
	if !chk.header() {
		return chk.errs
	}

	chk.allocation()

	if len(chk.errs) == 0 {
		chk.directory()
	}

	return chk.errs
}

// header checks section 2.2 and sets the sector size.
func (c *conformance) header() bool {
	if len(c.raw) < 512 || !bytes.Equal(c.raw[:8], signature) {
		c.errorf("bad signature")

		return false
	}

	major := c.u16(0x1A)

	switch {
	case major == 3 && c.u16(0x1E) == 9:
		c.ss = 512
	case major == 4 && c.u16(0x1E) == 12:
		c.ss = 4096
	default:
		c.errorf("version %d with sector shift %d", major, c.u16(0x1E))

		return false
	}

	if len(c.raw)%c.ss != 0 || len(c.raw) < 3*c.ss {
		c.errorf("%d bytes is not at least three whole sectors", len(c.raw))

		return false
	}

	c.sectors = len(c.raw)/c.ss - 1

	for name, ok := range map[string]bool{
		"header CLSID is zero":           bytes.Equal(c.raw[8:0x18], make([]byte, 16)),
		"minor version is 0x3E":          c.u16(0x18) == 0x3E,
		"byte order is 0xFFFE":           c.u16(0x1C) == 0xFFFE,
		"mini sector shift is 6":         c.u16(0x20) == 6,
		"reserved bytes are zero":        bytes.Equal(c.raw[0x22:0x28], make([]byte, 6)),
		"v3 directory sector count is 0": major == 4 || c.u32(0x28) == 0,
		"transaction signature is 0":     c.u32(0x34) == 0,
		"mini stream cutoff is 4096":     c.u32(0x38) == miniCutoff,
		"v4 header padding is zero":      bytes.Equal(c.raw[512:c.ss], make([]byte, c.ss-512)),
	} {
		if !ok {
			c.errorf("header: want %s", name)
		}
	}

	return true
}

// own marks a sector as used by one structure; sections 2.1 and 2.3 allow
// a sector in one chain only.
func (c *conformance) own(sect uint32, what string) bool {
	if int(sect) >= c.sectors {
		c.errorf("%s: sector %#x past the end of the file", what, sect)

		return false
	}

	if c.owner[sect] != "" {
		c.errorf("%s: sector %d already belongs to %s", what, sect, c.owner[sect])

		return false
	}

	c.owner[sect] = what

	return true
}

// chain follows a FAT chain and checks it has exactly want sectors.
func (c *conformance) chain(start uint32, want int, what string) []uint32 {
	var out []uint32

	for sect := start; sect != endOfChain; sect = c.fat[sect] {
		if sect > 0xFFFFFFFA || !c.own(sect, what) {
			c.errorf("%s: bad chain link %#x", what, sect)

			return out
		}

		out = append(out, sect)
	}

	if len(out) != want {
		c.errorf("%s: %d sectors, want %d", what, len(out), want)
	}

	return out
}

// allocation checks the DIFAT (2.5), the FAT (2.3) and that every allocated
// sector belongs to exactly one structure.
func (c *conformance) allocation() {
	c.owner = make([]string, c.sectors)
	numFAT := int(c.u32(0x2C))

	var fatSectors []uint32

	for i := range 109 {
		v := c.u32(0x4C + 4*i)
		if i < numFAT {
			fatSectors = append(fatSectors, v)
		} else if v != freeSect {
			c.errorf("header DIFAT[%d] = %#x past the FAT, want FREESECT", i, v)
		}
	}

	difatCount := 0

	for sect := c.u32(0x44); sect != endOfChain; difatCount++ {
		if !c.own(sect, "DIFAT") {
			return
		}

		sec := c.sector(sect)
		for i := range c.ss/4 - 1 {
			v := binary.LittleEndian.Uint32(sec[4*i:])
			if len(fatSectors) < numFAT {
				fatSectors = append(fatSectors, v)
			} else if v != freeSect {
				c.errorf("DIFAT entry past the FAT is %#x, want FREESECT", v)
			}
		}

		sect = binary.LittleEndian.Uint32(sec[c.ss-4:])
	}

	if difatCount != int(c.u32(0x48)) || len(fatSectors) != numFAT {
		c.errorf("DIFAT: %d sectors listing %d FAT sectors, header says %d and %d", difatCount, len(fatSectors), c.u32(0x48), numFAT)

		return
	}

	for _, n := range fatSectors {
		if !c.own(n, "FAT") {
			return
		}

		sec := c.sector(n)
		for i := range c.ss / 4 {
			c.fat = append(c.fat, binary.LittleEndian.Uint32(sec[4*i:]))
		}
	}

	c.markers()
}

// markers checks FATSECT, DIFSECT and FREESECT entries against the owners
// found so far.
func (c *conformance) markers() {
	for idx, value := range c.fat {
		switch {
		case idx >= c.sectors:
			if value != freeSect {
				c.errorf("FAT[%d] past the end of the file is %#x, want FREESECT", idx, value)
			}
		case c.owner[idx] == "FAT" && value != fatSect, c.owner[idx] == "DIFAT" && value != difSect:
			c.errorf("FAT[%d] of a %s sector is %#x", idx, c.owner[idx], value)
		case c.owner[idx] == "" && (value == fatSect || value == difSect):
			c.errorf("FAT[%d] marks an unlisted sector %#x", idx, value)
		}
	}
}

// dirEntry is the part of a directory entry the tree check needs.
type dirEntry struct {
	name               []uint16
	left, right, child uint32
	start              uint32
	size               uint64
	typ, colour        byte
}

// directory checks sections 2.4 and 2.6 and the user-defined data of 2.7.
func (c *conformance) directory() {
	perSector := c.ss / entrySize
	dirStart := c.u32(0x30)

	count := 0
	for n := dirStart; n != endOfChain && int(n) < len(c.fat) && count <= c.sectors; n = c.fat[n] {
		count++
	}

	if c.ss == 4096 && count != int(c.u32(0x28)) {
		c.errorf("directory: %d sectors, header says %d", count, c.u32(0x28))
	}

	entries := make([]dirEntry, 0, count*perSector)

	for _, n := range c.chain(dirStart, count, "directory") {
		for i := range perSector {
			entries = append(entries, c.entry(c.sector(n)[entrySize*i:entrySize*(i+1)], len(entries)))
		}
	}

	if len(c.errs) > 0 || len(entries) == 0 {
		return
	}

	root := entries[0]
	if string(utf16.Decode(root.name)) != "Root Entry" || root.typ != 5 {
		c.errorf("entry 0 is %q type %d, want Root Entry type 5", string(utf16.Decode(root.name)), root.typ)
	}

	c.tree(entries, root.child)
	c.streams(entries)
}

// entry decodes one directory entry and checks its fields (2.6.1, 2.6.3).
func (c *conformance) entry(rec []byte, sid int) dirEntry {
	order := binary.LittleEndian
	ent := dirEntry{
		typ: rec[0x42], colour: rec[0x43],
		left: order.Uint32(rec[0x44:]), right: order.Uint32(rec[0x48:]), child: order.Uint32(rec[0x4C:]),
		start: order.Uint32(rec[0x74:]), size: order.Uint64(rec[0x78:]),
	}

	zero := func(from, to int) bool { return bytes.Equal(rec[from:to], make([]byte, to-from)) }

	if ent.typ == 0 {
		blank := make([]byte, entrySize)
		copy(blank[0x44:0x50], bytes.Repeat([]byte{0xFF}, 12))

		if !bytes.Equal(rec, blank) {
			c.errorf("entry %d: unused entry is not zero with NOSTREAM links", sid)
		}

		return ent
	}

	nameLen := int(order.Uint16(rec[0x40:]))
	if nameLen%2 != 0 || nameLen < 4 || nameLen > 64 || order.Uint16(rec[nameLen-2:]) != 0 || !zero(nameLen, 64) {
		c.errorf("entry %d: bad name length %d or name padding", sid, nameLen)

		return ent
	}

	for i := range nameLen/2 - 1 {
		ent.name = append(ent.name, order.Uint16(rec[2*i:]))
	}

	switch {
	case ent.typ != 2 && ent.typ != 5:
		c.errorf("entry %d: object type %d, the writer only makes streams", sid, ent.typ)
	case ent.colour > 1:
		c.errorf("entry %d: colour %d", sid, ent.colour)
	case !zero(0x60, 0x74):
		c.errorf("entry %d: state bits or timestamps are not zero", sid)
	case ent.typ == 2 && (!zero(0x50, 0x60) || ent.child != noStream):
		c.errorf("entry %d: stream with a CLSID or child", sid)
	case ent.typ == 5 && (ent.left != noStream || ent.right != noStream || ent.colour != 1):
		c.errorf("entry %d: root with siblings or not black", sid)
	case c.ss == 512 && ent.size > 0x80000000:
		c.errorf("entry %d: v3 size %#x above 2 GB", sid, ent.size)
	}

	return ent
}

// tree checks the red-black rules of 2.6.4 and returns the in-order SIDs.
func (c *conformance) tree(entries []dirEntry, top uint32) {
	if top != noStream && entries[top].colour != 1 {
		c.errorf("tree: top node %d is red", top)
	}

	var (
		walk  func(sid uint32, parentRed bool, depth int) int
		order []uint32
	)

	walk = func(sid uint32, parentRed bool, depth int) int {
		if sid == noStream {
			return 0
		}

		if int(sid) >= len(entries) || depth > len(entries) || entries[sid].typ != 2 {
			c.errorf("tree: bad link %d", sid)

			return 0
		}

		ent := entries[sid]
		red := ent.colour == 0

		if red && parentRed {
			c.errorf("tree: red entry %d has a red parent", sid)
		}

		left := walk(ent.left, red, depth+1)

		order = append(order, sid)
		right := walk(ent.right, red, depth+1)

		if c.balanced && left != right {
			c.errorf("tree: entry %d has black heights %d and %d", sid, left, right)
		}

		if red {
			return left
		}

		return left + 1
	}

	walk(top, false, 0)

	for i := 1; i < len(order); i++ {
		if CompareUnits(entries[order[i-1]].name, entries[order[i]].name) >= 0 {
			c.errorf("tree: %q is not below %q", string(utf16.Decode(entries[order[i-1]].name)), string(utf16.Decode(entries[order[i]].name)))
		}
	}

	streams := 0

	for _, ent := range entries[1:] {
		if ent.typ == 2 {
			streams++
		}
	}

	if streams != len(order) {
		c.errorf("tree: %d of %d streams reachable", len(order), streams)
	}
}

// streams checks the chains of every stream, the mini stream and the zero
// padding behind the data (2.4, 2.7, 4.3).
func (c *conformance) streams(entries []dirEntry) {
	root := entries[0]
	numMiniFAT := int(c.u32(0x40))

	if root.size%miniSectorSize != 0 {
		c.errorf("mini stream size %d is not whole mini sectors", root.size)
	}

	for _, n := range c.chain(c.u32(0x3C), numMiniFAT, "MiniFAT") {
		sec := c.sector(n)
		for i := range c.ss / 4 {
			c.miniFAT = append(c.miniFAT, binary.LittleEndian.Uint32(sec[4*i:]))
		}
	}

	rootChain := c.chain(root.start, int((root.size+uint64(c.ss)-1)/uint64(c.ss)), "mini stream") //nolint:gosec // test sizes
	mini := make([]byte, 0, len(rootChain)*c.ss)

	for _, n := range rootChain {
		mini = append(mini, c.sector(n)...)
	}

	miniOwner := make([]bool, root.size/miniSectorSize)

	for sid, ent := range entries {
		if ent.typ != 2 {
			continue
		}

		switch {
		case ent.size == 0:
			if ent.start != endOfChain {
				c.errorf("entry %d: empty stream starts at %#x, want ENDOFCHAIN", sid, ent.start)
			}
		case ent.size < miniCutoff:
			c.miniChain(ent, sid, mini, miniOwner)
		default:
			data := c.chain(ent.start, int((ent.size+uint64(c.ss)-1)/uint64(c.ss)), fmt.Sprintf("stream %d", sid)) //nolint:gosec // test sizes
			if len(data) > 0 {
				tail := c.sector(data[len(data)-1])[int(ent.size%uint64(c.ss)):] //nolint:gosec // test sizes
				if ent.size%uint64(c.ss) != 0 && !bytes.Equal(tail, make([]byte, len(tail))) {
					c.errorf("entry %d: last sector is not zero-padded", sid)
				}
			}
		}
	}

	for i, used := range miniOwner {
		if !used {
			c.errorf("mini sector %d inside the mini stream is unused", i)
		}
	}

	for n, what := range c.owner {
		if what == "" && c.fat[n] != freeSect {
			c.errorf("FAT[%d] = %#x allocates a sector no structure uses", n, c.fat[n])
		}
	}
}

func (c *conformance) miniChain(ent dirEntry, sid int, mini []byte, owner []bool) {
	want := int((ent.size + miniSectorSize - 1) / miniSectorSize) //nolint:gosec // below 4096
	count := 0
	last := uint32(0)

	for mini := ent.start; mini != endOfChain; mini = c.miniFAT[mini] {
		if int(mini) >= len(owner) || int(mini) >= len(c.miniFAT) || owner[mini] {
			c.errorf("entry %d: bad mini chain link %#x", sid, mini)

			return
		}

		owner[mini] = true
		last = mini
		count++
	}

	if count != want {
		c.errorf("entry %d: %d mini sectors, want %d", sid, count, want)

		return
	}

	if rem := int(ent.size % miniSectorSize); rem != 0 {
		tail := mini[int(last)*miniSectorSize+rem : int(last+1)*miniSectorSize]
		if !bytes.Equal(tail, make([]byte, len(tail))) {
			c.errorf("entry %d: last mini sector is not zero-padded", sid)
		}
	}
}

// TestConformance runs the [MS-CFB] checker over the writer's output for
// every tree size up to a few hundred streams, both versions and both colour
// strategies, and over the vector and DIFAT files.
func TestConformance(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(2)) //nolint:gosec // reproducible test data
	sizes := []int{0, 1, 63, 64, 65, 4095, 4096, 4097, 5000, 600}

	for _, version := range []int{3, 4} {
		for _, allBlack := range []bool{false, true} {
			for count := range 70 {
				streams := make([]Stream, 0, count)

				for i := range count {
					data := make([]byte, sizes[(i*7+count)%len(sizes)])
					rng.Read(data)
					streams = append(streams, Stream{Name: fmt.Sprintf("%c%d", 'A'+rune(i%26), i), Data: data})
				}

				out, err := Write(streams, Options{Version: version, AllBlack: allBlack})
				if err != nil {
					t.Fatal(err)
				}

				for _, e := range checkConformance(out, !allBlack) {
					t.Errorf("v%d all-black=%v %d streams: %s", version, allBlack, count, e)
				}
			}
		}
	}

	for _, version := range []int{3, 4} {
		out, _ := Write(vectorStreams(), Options{Version: version})
		for _, e := range checkConformance(out, true) {
			t.Errorf("vector v%d: %s", version, e)
		}
	}
}

// TestConformanceDIFAT covers a v3 file with DIFAT sectors.
func TestConformanceDIFAT(t *testing.T) {
	t.Parallel()

	out, err := Write([]Stream{{Name: "big", Data: make([]byte, 8<<20)}, {Name: "tiny", Data: []byte("x")}}, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	if readU32(out, 0x48) == 0 {
		t.Fatal("no DIFAT sectors")
	}

	for _, e := range checkConformance(out, true) {
		t.Error(e)
	}
}

// The checker itself must catch the faults it claims to.
func TestConformanceCatchesFaults(t *testing.T) {
	t.Parallel()

	out, _ := Write(vectorStreams(), Options{Version: 3})
	dirOff := (int(readU32(out, 0x30)) + 1) * 512

	for name, mutate := range map[string]func([]byte){
		"header CLSID":   func(b []byte) { b[8] = 1 },
		"cutoff":         func(b []byte) { b[0x39] = 0x20 },
		"red top":        func(b []byte) { b[dirOff+2*entrySize+0x43] = 0 },
		"state bits":     func(b []byte) { b[dirOff+entrySize+0x60] = 1 },
		"unsorted":       func(b []byte) { b[dirOff+entrySize] = 'Z' },
		"padding":        func(b []byte) { b[len(b)-1] = 1 },
		"free FAT entry": func(b []byte) { b[512+4*100] = 0 },
	} {
		bad := bytes.Clone(out)
		mutate(bad)

		if len(checkConformance(bad, true)) == 0 {
			t.Errorf("%s: not detected", name)
		}
	}
}
