package main

// Compound File Binary reader and writer written from MSISPEC.md section 2.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode/utf16"
)

const (
	freeSect   = 0xFFFFFFFF
	endOfChain = 0xFFFFFFFE
	fatSect    = 0xFFFFFFFD
	difSect    = 0xFFFFFFFC
	noStream   = 0xFFFFFFFF
)

var msiCLSID = [16]byte{0x84, 0x10, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}

type cfbEntry struct {
	SID         int
	Name        []uint16
	Type        byte
	Color       byte
	Left, Right uint32
	Child       uint32
	CLSID       [16]byte
	Start       uint32
	Size        uint64
	dirOffset   int // byte offset of this entry in the file
	Parent      int
	Data        []byte
	DataOffsets []int // file offset of each 64- or SS-byte unit of the stream
	UnitSize    int
}

func (e *cfbEntry) NameString() string { return string(utf16.Decode(e.Name)) }

type cfbFile struct {
	Raw     []byte
	Major   uint16
	SS      int
	Entries []*cfbEntry
	fat     []uint32
	minifat []uint32
	ByName  map[string]*cfbEntry // root-level streams by decoded UTF-16 name
}

func (f *cfbFile) sectorOff(n uint32) int { return (int(n) + 1) * f.SS }

func (f *cfbFile) chain(start uint32, limit int) ([]uint32, error) {
	var out []uint32
	seen := map[uint32]bool{}
	for s := start; s != endOfChain; {
		if int(s) >= len(f.fat) || seen[s] {
			return out, fmt.Errorf("bad FAT chain at %#x", s)
		}
		seen[s] = true
		out = append(out, s)
		if limit > 0 && len(out) > limit {
			return out, errors.New("chain too long")
		}
		s = f.fat[s]
	}
	return out, nil
}

func readCFB(raw []byte) (*cfbFile, error) {
	if len(raw) < 512 || !bytes.Equal(raw[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return nil, errors.New("bad signature")
	}
	le := binary.LittleEndian
	f := &cfbFile{Raw: raw, Major: le.Uint16(raw[0x1A:]), ByName: map[string]*cfbEntry{}}
	if le.Uint16(raw[0x1C:]) != 0xFFFE {
		return nil, errors.New("bad byte order")
	}
	shift := le.Uint16(raw[0x1E:])
	if !(f.Major == 3 && shift == 9) && !(f.Major == 4 && shift == 12) {
		return nil, fmt.Errorf("bad version/shift %d/%d", f.Major, shift)
	}
	f.SS = 1 << shift
	nFat := le.Uint32(raw[0x2C:])
	firstDir := le.Uint32(raw[0x30:])
	firstMini := le.Uint32(raw[0x3C:])
	firstDifat := le.Uint32(raw[0x44:])
	var fatSectors []uint32
	for i := 0; i < 109 && uint32(len(fatSectors)) < nFat; i++ {
		fatSectors = append(fatSectors, le.Uint32(raw[0x4C+4*i:]))
	}
	for d := firstDifat; d != endOfChain && d != freeSect && uint32(len(fatSectors)) < nFat; {
		off := f.sectorOff(d)
		for i := 0; i < f.SS/4-1 && uint32(len(fatSectors)) < nFat; i++ {
			fatSectors = append(fatSectors, le.Uint32(raw[off+4*i:]))
		}
		d = le.Uint32(raw[off+f.SS-4:])
	}
	for _, s := range fatSectors {
		off := f.sectorOff(s)
		if off+f.SS > len(raw) {
			return nil, errors.New("FAT sector out of range")
		}
		for i := 0; i < f.SS/4; i++ {
			f.fat = append(f.fat, le.Uint32(raw[off+4*i:]))
		}
	}
	dirChain, err := f.chain(firstDir, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("directory: %w", err)
	}
	for _, s := range dirChain {
		off := f.sectorOff(s)
		for i := 0; i < f.SS/128; i++ {
			eo := off + 128*i
			e := raw[eo : eo+128]
			nl := int(le.Uint16(e[0x40:]))
			ent := &cfbEntry{SID: len(f.Entries), Type: e[0x42], Color: e[0x43], Left: le.Uint32(e[0x44:]), Right: le.Uint32(e[0x48:]),
				Child: le.Uint32(e[0x4C:]), Start: le.Uint32(e[0x74:]), Size: le.Uint64(e[0x78:]), dirOffset: eo, Parent: -1}
			if f.Major == 3 {
				ent.Size &= 0xFFFFFFFF
			}
			copy(ent.CLSID[:], e[0x50:0x60])
			if nl >= 2 && nl <= 64 {
				for j := 0; j < nl/2-1; j++ {
					ent.Name = append(ent.Name, le.Uint16(e[2*j:]))
				}
			}
			f.Entries = append(f.Entries, ent)
		}
	}
	if len(f.Entries) == 0 || f.Entries[0].Type != 5 {
		return nil, errors.New("no root entry")
	}
	root := f.Entries[0]
	// mini stream
	if firstMini != endOfChain && firstMini != freeSect {
		mc, err := f.chain(firstMini, 1<<20)
		if err != nil {
			return nil, fmt.Errorf("minifat: %w", err)
		}
		for _, s := range mc {
			off := f.sectorOff(s)
			for i := 0; i < f.SS/4; i++ {
				f.minifat = append(f.minifat, le.Uint32(raw[off+4*i:]))
			}
		}
	}
	var rootChain []uint32
	if root.Size > 0 {
		rootChain, err = f.chain(root.Start, 1<<22)
		if err != nil {
			return nil, fmt.Errorf("root chain: %w", err)
		}
	}
	miniOff := func(m uint32) int {
		pos := int(m) * 64
		return f.sectorOff(rootChain[pos/f.SS]) + pos%f.SS
	}
	// tree walk with visited set
	visited := map[uint32]bool{}
	var walk func(sid uint32, parent int) error
	walk = func(sid uint32, parent int) error {
		if sid == noStream {
			return nil
		}
		if int(sid) >= len(f.Entries) || visited[sid] {
			return fmt.Errorf("bad sibling link %d", sid)
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
	visited[0] = true
	if err := walk(root.Child, 0); err != nil {
		return nil, err
	}
	for sid := range visited {
		e := f.Entries[sid]
		if e.Type != 2 {
			continue
		}
		if e.Size == 0 {
			if e.Parent == 0 {
				f.ByName[e.NameString()] = e
			}
			continue
		}
		if e.Size < 4096 {
			e.UnitSize = 64
			for m, n := e.Start, 0; uint64(n)*64 < e.Size; n++ {
				if m == endOfChain || int(m) >= len(f.minifat) {
					return nil, fmt.Errorf("bad minifat chain for %q", e.NameString())
				}
				o := miniOff(m)
				e.DataOffsets = append(e.DataOffsets, o)
				e.Data = append(e.Data, raw[o:o+64]...)
				m = f.minifat[m]
			}
		} else {
			e.UnitSize = f.SS
			c, err := f.chain(e.Start, int(e.Size)/f.SS+2)
			if err != nil {
				return nil, fmt.Errorf("stream %q: %w", e.NameString(), err)
			}
			for _, s := range c {
				o := f.sectorOff(s)
				e.DataOffsets = append(e.DataOffsets, o)
				e.Data = append(e.Data, raw[o:o+f.SS]...)
			}
		}
		e.Data = e.Data[:e.Size]
		if e.Parent == 0 {
			f.ByName[e.NameString()] = e
		}
	}
	return f, nil
}

// patchStream overwrites a stream in place with same-length content and
// returns the new file bytes.
func (f *cfbFile) patchStream(e *cfbEntry, data []byte) []byte {
	if uint64(len(data)) != e.Size {
		panic("patchStream: size change")
	}
	out := append([]byte(nil), f.Raw...)
	for i, o := range e.DataOffsets {
		lo := i * e.UnitSize
		hi := lo + e.UnitSize
		if hi > len(data) {
			hi = len(data)
		}
		copy(out[o:], data[lo:hi])
	}
	return out
}

// ---- writer ----

type cfbStream struct {
	Name []uint16
	Data []byte
}

func cfbUpper(u uint16) uint16 {
	if u >= 'a' && u <= 'z' {
		return u - 32
	}
	return u
}

func cfbCompare(a, b []uint16) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	for i := range a {
		x, y := cfbUpper(a[i]), cfbUpper(b[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

type cfbWriteOpts struct {
	Major         int
	CLSID         [16]byte
	AllBlack      bool
	EmptyStart    uint32 // start sector written for size-0 streams
	EmptyStartSet bool   // use EmptyStart; otherwise ENDOFCHAIN
}

func writeCFB(streams []cfbStream, o cfbWriteOpts) ([]byte, error) {
	if o.Major == 0 {
		o.Major = 3
	}
	if !o.EmptyStartSet {
		o.EmptyStart = endOfChain
	}
	ss := 512
	if o.Major == 4 {
		ss = 4096
	}
	for _, s := range streams {
		if len(s.Name) < 1 || len(s.Name) > 31 {
			return nil, fmt.Errorf("bad name length %d", len(s.Name))
		}
	}
	sorted := append([]cfbStream(nil), streams...)
	sort.Slice(sorted, func(i, j int) bool { return cfbCompare(sorted[i].Name, sorted[j].Name) < 0 })
	for i := 1; i < len(sorted); i++ {
		if cfbCompare(sorted[i-1].Name, sorted[i].Name) == 0 {
			return nil, errors.New("duplicate sibling name")
		}
	}
	n := len(sorted)
	type node struct {
		idx         int // index into sorted
		left, right int // node index into sorted or -1
		depth       int
	}
	nodes := make([]node, n)
	var build func(lo, hi, depth int) int
	build = func(lo, hi, depth int) int {
		if lo >= hi {
			return -1
		}
		mid := lo + (hi-lo)/2
		nodes[mid] = node{idx: mid, depth: depth}
		nodes[mid].left = build(lo, mid, depth+1)
		nodes[mid].right = build(mid+1, hi, depth+1)
		return mid
	}
	top := build(0, n, 0)
	F := 0
	for (1 << (F + 1)) <= n+1 {
		F++
	}
	// SIDs: root 0, pre-order (node, left, right)? spec: depth-first pre-order with children in Compare order.
	// We assign SIDs in sorted (in-order) order after the root, which is "children in Compare order".
	sid := make([]int, n)
	for i := range sorted {
		sid[i] = i + 1
	}
	E := n + 1
	perSec := ss / 128
	D := (E + perSec - 1) / perSec
	// small/large
	var miniData []byte
	miniStart := make([]uint32, n)
	for i, s := range sorted {
		if len(s.Data) == 0 {
			miniStart[i] = o.EmptyStart
			continue
		}
		if len(s.Data) < 4096 {
			miniStart[i] = uint32(len(miniData) / 64)
			miniData = append(miniData, s.Data...)
			if pad := len(miniData) % 64; pad != 0 {
				miniData = append(miniData, make([]byte, 64-pad)...)
			}
		}
	}
	nMini := len(miniData) / 64
	M := (nMini + ss/4 - 1) / (ss / 4)
	S := (nMini*64 + ss - 1) / ss
	L := 0
	for _, s := range sorted {
		if len(s.Data) >= 4096 {
			L += (len(s.Data) + ss - 1) / ss
		}
	}
	Fs, X := 0, 0
	T := 0
	for iter := 0; iter < 100; iter++ {
		T = Fs + X + D + M + S + L
		nf := (T + ss/4 - 1) / (ss / 4)
		nx := 0
		if nf > 109 {
			nx = (nf - 109 + ss/4 - 2) / (ss/4 - 1)
		}
		if nf == Fs && nx == X {
			break
		}
		Fs, X = nf, nx
	}
	T = Fs + X + D + M + S + L
	out := make([]byte, (T+1)*ss)
	le := binary.LittleEndian
	copy(out, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	le.PutUint16(out[0x18:], 0x3E)
	le.PutUint16(out[0x1A:], uint16(o.Major))
	le.PutUint16(out[0x1C:], 0xFFFE)
	if o.Major == 4 {
		le.PutUint16(out[0x1E:], 12)
		le.PutUint32(out[0x28:], uint32(D))
	} else {
		le.PutUint16(out[0x1E:], 9)
	}
	le.PutUint16(out[0x20:], 6)
	le.PutUint32(out[0x2C:], uint32(Fs))
	fatStart := 0
	difStart := Fs
	dirStart := Fs + X
	mfStart := dirStart + D
	msStart := mfStart + M
	lgStart := msStart + S
	le.PutUint32(out[0x30:], uint32(dirStart))
	le.PutUint32(out[0x38:], 4096)
	if M > 0 {
		le.PutUint32(out[0x3C:], uint32(mfStart))
	} else {
		le.PutUint32(out[0x3C:], endOfChain)
	}
	le.PutUint32(out[0x40:], uint32(M))
	if X > 0 {
		le.PutUint32(out[0x44:], uint32(difStart))
	} else {
		le.PutUint32(out[0x44:], endOfChain)
	}
	le.PutUint32(out[0x48:], uint32(X))
	for i := 0; i < 109; i++ {
		v := uint32(freeSect)
		if i < Fs {
			v = uint32(fatStart + i)
		}
		le.PutUint32(out[0x4C+4*i:], v)
	}
	soff := func(s int) int { return (s + 1) * ss }
	fat := make([]uint32, Fs*ss/4)
	for i := range fat {
		fat[i] = freeSect
	}
	for i := 0; i < Fs; i++ {
		fat[fatStart+i] = fatSect
	}
	for i := 0; i < X; i++ {
		fat[difStart+i] = difSect
	}
	linkRun := func(start, count int) {
		for i := 0; i < count; i++ {
			if i == count-1 {
				fat[start+i] = endOfChain
			} else {
				fat[start+i] = uint32(start + i + 1)
			}
		}
	}
	linkRun(dirStart, D)
	linkRun(mfStart, M)
	linkRun(msStart, S)
	largeStart := make([]uint32, n)
	cur := lgStart
	for i, s := range sorted {
		if len(s.Data) >= 4096 {
			c := (len(s.Data) + ss - 1) / ss
			largeStart[i] = uint32(cur)
			linkRun(cur, c)
			copy(out[soff(cur):], s.Data)
			cur += c
		}
	}
	// DIFAT sectors
	for x := 0; x < X; x++ {
		off := soff(difStart + x)
		for i := 0; i < ss/4-1; i++ {
			k := 109 + x*(ss/4-1) + i
			v := uint32(freeSect)
			if k < Fs {
				v = uint32(fatStart + k)
			}
			le.PutUint32(out[off+4*i:], v)
		}
		if x == X-1 {
			le.PutUint32(out[off+ss-4:], endOfChain)
		} else {
			le.PutUint32(out[off+ss-4:], uint32(difStart+x+1))
		}
	}
	for i, v := range fat {
		le.PutUint32(out[soff(0)+4*i:], v) // FAT sectors are contiguous from sector 0
	}
	// minifat
	if M > 0 {
		mf := make([]uint32, M*ss/4)
		for i := range mf {
			mf[i] = freeSect
		}
		for i, s := range sorted {
			if len(s.Data) > 0 && len(s.Data) < 4096 {
				c := (len(s.Data) + 63) / 64
				st := int(miniStart[i])
				for k := 0; k < c; k++ {
					if k == c-1 {
						mf[st+k] = endOfChain
					} else {
						mf[st+k] = uint32(st + k + 1)
					}
				}
			}
		}
		for i, v := range mf {
			le.PutUint32(out[soff(mfStart)+4*i:], v)
		}
		copy(out[soff(msStart):], miniData)
	}
	// directory
	dirOff := soff(dirStart)
	putEntry := func(idx int, name []uint16, typ, color byte, left, right, child uint32, clsid []byte, start uint32, size uint64) {
		e := out[dirOff+128*idx : dirOff+128*idx+128]
		for i, u := range name {
			le.PutUint16(e[2*i:], u)
		}
		le.PutUint16(e[0x40:], uint16(2*(len(name)+1)))
		e[0x42] = typ
		e[0x43] = color
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
		return uint32(sid[k])
	}
	rootStart := uint32(endOfChain)
	if S > 0 {
		rootStart = uint32(msStart)
	}
	topSID := uint32(noStream)
	if top >= 0 {
		topSID = uint32(sid[top])
	}
	putEntry(0, utf16.Encode([]rune("Root Entry")), 5, 1, noStream, noStream, topSID, o.CLSID[:], rootStart, uint64(nMini*64))
	for i, s := range sorted {
		color := byte(1)
		if !o.AllBlack && nodes[i].depth >= F {
			color = 0
		}
		var start uint32
		switch {
		case len(s.Data) == 0:
			start = o.EmptyStart
		case len(s.Data) < 4096:
			start = miniStart[i]
		default:
			start = largeStart[i]
		}
		putEntry(sid[i], s.Name, 2, color, link(nodes[i].left), link(nodes[i].right), noStream, nil, start, uint64(len(s.Data)))
	}
	for k := E; k < D*perSec; k++ {
		e := out[dirOff+128*k:]
		le.PutUint32(e[0x44:], noStream)
		le.PutUint32(e[0x48:], noStream)
		le.PutUint32(e[0x4C:], noStream)
	}
	return out, nil
}

// streamsOf returns the root-level streams of a CFB file in SID order.
func (f *cfbFile) streamsOf() []cfbStream {
	var out []cfbStream
	for _, e := range f.Entries {
		if e.Type == 2 && e.Parent == 0 {
			out = append(out, cfbStream{Name: e.Name, Data: e.Data})
		}
	}
	return out
}
