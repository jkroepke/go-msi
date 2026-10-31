// Package pe reads the version resource of a Portable Executable file: the
// file version, the translation languages and whether the optional header
// carries a checksum (docs/spec/MSISPEC.md section 11.5). It never executes
// or loads the file.
package pe

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Machine values of the COFF header.
const (
	MachineI386  = 0x014C
	MachineAMD64 = 0x8664
	MachineARM64 = 0xAA64
)

// Info describes a PE file.
type Info struct {
	FileVersion    string
	ProductVersion string
	Languages      []int
	Machine        uint16
	Is64Bit        bool
	HasChecksum    bool
	HasVersion     bool
}

// Language returns the File.Language value: the decimal languages joined
// with commas, or "" when the resource has no translations.
func (i *Info) Language() string {
	parts := make([]string, 0, len(i.Languages))
	for _, l := range i.Languages {
		parts = append(parts, strconv.Itoa(l))
	}

	return strings.Join(parts, ",")
}

// ErrNotPE is returned when the data does not start with a PE image.
var ErrNotPE = errors.New("pe: not a PE file")

// IsPE reports whether data starts with the MZ and PE signatures.
func IsPE(r io.ReaderAt) bool {
	_, err := peOffset(r)

	return err == nil
}

// imageFileDLL is the COFF characteristics flag of a DLL.
const imageFileDLL = 0x2000

// IsDLL reports whether data is a PE image whose COFF header marks it as a
// DLL.
func IsDLL(rdr io.ReaderAt) bool {
	off, err := peOffset(rdr)
	if err != nil {
		return false
	}

	// Signature (4) + Machine, NumberOfSections, TimeDateStamp,
	// PointerToSymbolTable, NumberOfSymbols, SizeOfOptionalHeader (18).
	var chars [2]byte
	if _, err := rdr.ReadAt(chars[:], off+4+18); err != nil {
		return false
	}

	return binary.LittleEndian.Uint16(chars[:])&imageFileDLL != 0
}

func peOffset(rdr io.ReaderAt) (int64, error) {
	var head [0x40]byte
	if _, err := rdr.ReadAt(head[:], 0); err != nil {
		return 0, ErrNotPE
	}

	if head[0] != 'M' || head[1] != 'Z' {
		return 0, ErrNotPE
	}

	off := int64(binary.LittleEndian.Uint32(head[0x3C:]))

	var sig [4]byte
	if _, err := rdr.ReadAt(sig[:], off); err != nil || string(sig[:]) != "PE\x00\x00" {
		return 0, ErrNotPE
	}

	return off, nil
}

type section struct {
	virtualAddress uint32
	virtualSize    uint32
	rawPointer     uint32
	rawSize        uint32
}

type image struct {
	r        io.ReaderAt
	sections []section
	// fileAlign is the optional header's FileAlignment when it is a power
	// of two from 512 to 64 KiB, otherwise 1.
	fileAlign uint64
}

// section returns the section that holds size bytes at rva.
func (im *image) section(rva, size uint32) (section, bool) {
	// 64-bit arithmetic: the 32-bit fields of a corrupt header can wrap
	// rva+size or the section end around and pass a 32-bit range check.
	for _, s := range im.sections {
		start := uint64(s.virtualAddress)
		end := start + uint64(max(s.virtualSize, s.rawSize))

		if uint64(rva) >= start && uint64(rva)+uint64(size) <= end {
			return s, true
		}
	}

	return section{}, false
}

// read returns size bytes at rva from the section that holds them.
func (im *image) read(rva, size uint32) ([]byte, error) {
	sec, ok := im.section(rva, size)
	if !ok {
		return nil, fmt.Errorf("pe: RVA %#x is not in any section", rva)
	}

	// Only the first SizeOfRawData bytes of a section, rounded up to
	// FileAlignment, come from the file; the rest of its VirtualSize reads
	// as zeros, never as the bytes that follow in the file. Windows did
	// both: a raw size of 0 gave no version, one that ended inside the
	// Translation value still gave every language (FILE-14).
	rel := uint64(rva - sec.virtualAddress)
	rawSize := (uint64(sec.rawSize) + im.fileAlign - 1) / im.fileAlign * im.fileAlign
	fileBacked := uint64(0)

	if rel < rawSize {
		fileBacked = min(uint64(size), rawSize-rel)
	}

	off := int64(sec.rawPointer) + int64(rel) //nolint:gosec // both fit in 33 bits

	// Grow the buffer while reading instead of allocating size up front: a
	// corrupt header can claim gigabytes that the file does not hold, and
	// the read then fails at the real end.
	buf, err := io.ReadAll(io.NewSectionReader(im.r, off, int64(fileBacked)))
	if err != nil {
		return nil, fmt.Errorf("pe: reading RVA %#x: %w", rva, err)
	}

	if uint64(len(buf)) != fileBacked {
		return nil, fmt.Errorf("pe: reading RVA %#x: %w", rva, io.ErrUnexpectedEOF)
	}

	// The zero tail is at most what the section's VirtualSize covers, and
	// every caller reads a bounded record (at most about 1 MiB).
	return append(buf, make([]byte, uint64(size)-fileBacked)...), nil
}

// resources is the resource directory at rva. Its records are read one at
// a time, so that memory does not grow with the resources of the file
// (icons, manifests, embedded payloads) that are not the version. The
// directory's declared size does not bound them: Windows reads a record
// wherever the image holds it, so each one is checked against the section
// that backs it instead.
type resources struct {
	img *image
	rva uint32
}

// read returns size bytes at offset off of the resource directory.
func (r *resources) read(off, size uint32) ([]byte, error) {
	// Compare in 64 bits: a corrupt offset near 4 GiB would wrap the sum.
	if uint64(r.rva)+uint64(off)+uint64(size) > math.MaxUint32 {
		return nil, errors.New("pe: resource directory out of range")
	}

	return r.img.read(r.rva+off, size)
}

// Parse reads the headers and the version resource.
func Parse(rdr io.ReaderAt) (*Info, error) { //nolint:cyclop // one pass over the format; splitting would scatter it
	off, err := peOffset(rdr)
	if err != nil {
		return nil, err
	}

	order := binary.LittleEndian

	var coff [20]byte
	if _, err := rdr.ReadAt(coff[:], off+4); err != nil {
		return nil, fmt.Errorf("pe: COFF header: %w", err)
	}

	info := &Info{Machine: order.Uint16(coff[0:])}
	numSections := int(order.Uint16(coff[2:]))
	optSize := int(order.Uint16(coff[16:]))
	optOff := off + 24

	if optSize < 96 {
		return nil, errors.New("pe: optional header too short")
	}

	opt := make([]byte, optSize)
	if _, err := rdr.ReadAt(opt, optOff); err != nil {
		return nil, fmt.Errorf("pe: optional header: %w", err)
	}

	var dirOff int

	switch order.Uint16(opt) {
	case 0x10B:
		dirOff = 96
	case 0x20B:
		dirOff = 112
		info.Is64Bit = true
	default:
		return nil, fmt.Errorf("pe: unknown optional header magic %#x", order.Uint16(opt))
	}

	info.HasChecksum = order.Uint32(opt[64:]) != 0

	// NumberOfRvaAndSizes precedes the data directories; the length check
	// comes first, since a PE32+ header of 96 to 111 bytes does not hold it.
	// Windows reads only
	// the slots it declares: with fewer than three there is no resource
	// directory, whatever bytes follow in the optional header.
	if dirOff+8*3 > optSize || order.Uint32(opt[dirOff-4:]) < 3 {
		return info, nil // no resource directory
	}

	resRVA := order.Uint32(opt[dirOff+8*2:])

	secs := make([]byte, 40*numSections)
	if _, err := rdr.ReadAt(secs, optOff+int64(optSize)); err != nil {
		return nil, fmt.Errorf("pe: section headers: %w", err)
	}

	img := &image{r: rdr, fileAlign: 1}
	if align := uint64(order.Uint32(opt[36:])); align >= 512 && align <= 0x10000 && align&(align-1) == 0 {
		img.fileAlign = align
	}

	for i := range numSections {
		s := secs[40*i:]
		img.sections = append(img.sections, section{
			virtualSize:    order.Uint32(s[8:]),
			virtualAddress: order.Uint32(s[12:]),
			rawSize:        order.Uint32(s[16:]),
			rawPointer:     order.Uint32(s[20:]),
		})
	}

	// The size is not checked at all: Windows read the version through a
	// directory declared with size 0 (FILE-14).
	if resRVA == 0 {
		return info, nil
	}

	// The declared size is not a bound: Windows finds the version through
	// the records it reads, and each of those must lie in a section's data.
	if err := parseVersion(&resources{img: img, rva: resRVA}, info); err != nil {
		return nil, err
	}

	return info, nil
}

// findEntry returns the offset of a resource directory entry's target. With
// id < 0 the first entry is taken.
func findEntry(res *resources, dir uint32, wantID int) (uint32, bool, error) {
	head, err := res.read(dir, 16)
	if err != nil {
		return 0, false, err
	}

	order := binary.LittleEndian
	count := uint32(order.Uint16(head[12:])) + uint32(order.Uint16(head[14:]))

	// At most 2 x 65535 entries of 8 bytes: about 1 MiB.
	entries, err := res.read(dir+16, 8*count)
	if err != nil {
		return 0, false, err
	}

	for i := range count {
		name := order.Uint32(entries[8*i:])
		off := order.Uint32(entries[8*i+4:])

		if wantID < 0 || (name&0x80000000 == 0 && int(name) == wantID) {
			return off, true, nil
		}
	}

	return 0, false, nil
}

// maxVersionLanguages caps the language entries of the version resource
// that are compared: real files hold one, and each costs a read of up to
// 64 KiB.
const maxVersionLanguages = 64

func parseVersion(res *resources, info *Info) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	const (
		rtVersion = 16
		// vsVersionInfo is the resource ID Windows reads the version from
		// (VS_VERSION_INFO); another ID is never used (FILE-14).
		vsVersionInfo = 1
	)

	typeDir, ok, err := findEntry(res, 0, rtVersion)
	if err != nil || !ok || typeDir&0x80000000 == 0 {
		return err
	}

	nameDir, ok, err := findEntry(res, typeDir&0x7FFFFFFF, vsVersionInfo)
	if err != nil || !ok || nameDir&0x80000000 == 0 {
		return err
	}

	langDir := nameDir & 0x7FFFFFFF

	head, err := res.read(langDir, 16)
	if err != nil {
		return err
	}

	count := int(binary.LittleEndian.Uint16(head[12:])) + int(binary.LittleEndian.Uint16(head[14:]))
	if count > maxVersionLanguages {
		return fmt.Errorf("pe: %d version resource languages", count)
	}

	// Windows picks the language entry by the user's UI language, so a file
	// whose entries disagree has a version that depends on the machine
	// (FILE-14: English 9.8.7.6 won over German 1.2.3.4 on an English
	// system). Such a file is refused, which the builder reports like an
	// unversioned one, instead of picking one entry.
	var first *Info

	for i := range count {
		entry, err := res.read(langDir+16+8*uint32(i), 8)
		if err != nil {
			return err
		}

		dataEntry := binary.LittleEndian.Uint32(entry[4:])
		if dataEntry&0x80000000 != 0 {
			return errors.New("pe: version resource language entry is a directory")
		}

		cur := &Info{}
		if err := versionEntry(res, dataEntry, cur); err != nil {
			return err
		}

		if first == nil {
			first = cur
		} else if cur.FileVersion != first.FileVersion || cur.ProductVersion != first.ProductVersion ||
			cur.HasVersion != first.HasVersion || !slices.Equal(cur.Languages, first.Languages) {
			return fmt.Errorf("pe: version resource languages disagree (%s %s and %s %s)",
				first.FileVersion, first.Language(), cur.FileVersion, cur.Language())
		}
	}

	if first != nil {
		info.FileVersion, info.ProductVersion = first.FileVersion, first.ProductVersion
		info.HasVersion, info.Languages = first.HasVersion, first.Languages
	}

	return nil
}

// versionEntry reads one language's version resource into info.
func versionEntry(res *resources, dataEntry uint32, info *Info) error {
	data, rawStart, err := versionData(res, dataEntry)
	if err != nil {
		return err
	}

	if err := parseVersionInfo(data, info); err != nil {
		return err
	}

	// Windows reported the version but no languages when the version data
	// started at or past the section's SizeOfRawData, inside the part that
	// rounding to FileAlignment adds: raw sizes 1 to 0x58 with the data at
	// 0x58 gave no languages, 0x60 and up gave all of them (FILE-14).
	if !rawStart {
		info.Languages = nil
	}

	return nil
}

// versionData reads the version resource that the data entry at offset
// dataEntry of the resource directory describes. rawStart reports whether
// the data starts within its section's unrounded SizeOfRawData.
func versionData(res *resources, dataEntry uint32) ([]byte, bool, error) {
	entry, err := res.read(dataEntry, 16)
	if err != nil {
		return nil, false, err
	}

	le := binary.LittleEndian
	dataRVA := le.Uint32(entry)
	dataSize := le.Uint32(entry[4:])

	// Only the block VS_VERSIONINFO's wLength declares is read, and it must
	// lie in a section; the declared size only has to hold it. Windows read
	// a version whose data entry claimed more than its section (FILE-14).
	if dataSize < 6 {
		return nil, false, errors.New("pe: version resource too short")
	}

	head, err := res.img.read(dataRVA, 2)
	if err != nil {
		return nil, false, err
	}

	length := uint32(le.Uint16(head))
	if length > dataSize {
		return nil, false, fmt.Errorf("pe: version block of %d bytes is longer than its %d-byte resource", length, dataSize)
	}

	data, err := res.img.read(dataRVA, length)
	if err != nil {
		return nil, false, err
	}

	sec, _ := res.img.section(dataRVA, length)

	return data, dataRVA-sec.virtualAddress < sec.rawSize, nil
}

func utf16At(b []byte, off int) (string, int) {
	var units []uint16

	for ; off+2 <= len(b); off += 2 {
		unit := binary.LittleEndian.Uint16(b[off:])
		if unit == 0 {
			off += 2

			break
		}

		units = append(units, unit)
	}

	return string(utf16.Decode(units)), off
}

func align4(n int) int { return (n + 3) &^ 3 }

// parseVersionInfo reads VS_VERSIONINFO.
func parseVersionInfo(data []byte, info *Info) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	if len(data) < 6 {
		return errors.New("pe: version resource too short")
	}

	order := binary.LittleEndian
	length := int(order.Uint16(data))
	valueLen := int(order.Uint16(data[2:]))

	// wType 0 is binary data, as VS_FIXEDFILEINFO is; Windows refuses a
	// root block that claims text.
	if wType := order.Uint16(data[4:]); wType != 0 {
		return fmt.Errorf("pe: version resource type %d is not binary", wType)
	}

	// A block longer than its data is truncated: Windows refuses it
	// (FILE-14).
	if length > len(data) || length < 6 {
		return fmt.Errorf("pe: version block length %d does not fit its %d bytes", length, len(data))
	}

	// Every key is read within its own block: a missing terminator must not
	// pull in the bytes that follow.
	key, off := utf16At(data[:length], 6)
	if key != "VS_VERSION_INFO" {
		return fmt.Errorf("pe: version resource key %q", key)
	}

	off = align4(off)

	// The value must lie inside the block it belongs to: Windows refuses
	// one that runs past it, so it must not count as a version here.
	if off+valueLen > length {
		return fmt.Errorf("pe: version value of %d bytes runs past the %d-byte block", valueLen, length)
	}

	if valueLen >= 52 && off+52 <= length && order.Uint32(data[off:]) == 0xFEEF04BD {
		format := func(ms, ls uint32) string {
			return fmt.Sprintf("%d.%d.%d.%d", ms>>16, ms&0xFFFF, ls>>16, ls&0xFFFF)
		}

		info.HasVersion = true
		info.FileVersion = format(order.Uint32(data[off+8:]), order.Uint32(data[off+12:]))
		info.ProductVersion = format(order.Uint32(data[off+16:]), order.Uint32(data[off+20:]))
	}

	off = align4(off + valueLen)

	// Children: StringFileInfo and VarFileInfo blocks.
	for off+6 <= length {
		blockLen := int(order.Uint16(data[off:]))
		if blockLen < 6 || off+blockLen > length {
			break
		}

		block := data[off : off+blockLen]
		blockKey, koff := utf16At(block, 6)

		if blockKey == "VarFileInfo" {
			parseVarFileInfo(block, align4(koff), info)
		}

		off = align4(off + blockLen)
	}

	return nil
}

func parseVarFileInfo(data []byte, off int, info *Info) {
	order := binary.LittleEndian

	for off+6 <= len(data) {
		varLen := int(order.Uint16(data[off:]))
		valueLen := int(order.Uint16(data[off+2:]))

		if varLen < 6 || off+varLen > len(data) {
			return
		}

		// The Var block ends at off+varLen: its key and value are read
		// within it, never from the Var that follows.
		end := off + varLen

		key, koff := utf16At(data[:end], off+6)
		koff = align4(koff)

		// A value that runs past its Var is invalid as a whole: Windows
		// then reports no languages, not the ones that fit (FILE-14).
		if key == "Translation" && koff+valueLen <= end {
			seen := make(map[int]bool)

			var langs []int

			// A length that is not a multiple of 4 still counts its last,
			// partial entry when the block holds all of it: lengths 5 to 7
			// gave two languages on Windows (FILE-14).
			for p := koff; p < koff+valueLen && p+4 <= end; p += 4 {
				lang := int(order.Uint16(data[p:]))
				if !seen[lang] {
					seen[lang] = true
					langs = append(langs, lang)
				}
			}

			// MsiGetFileVersionW lists the languages last first: stored
			// 1033,1031,2057 came back as 2057,1031,1033 (FILE-14).
			slices.Reverse(langs)
			info.Languages = append(info.Languages, langs...)
		}

		off = align4(off + varLen)
	}
}
