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

	off := int64(sec.rawPointer) + int64(rva-sec.virtualAddress)

	// Grow the buffer while reading instead of allocating size up front: a
	// corrupt header can claim gigabytes that the file does not hold, and
	// the read then fails at the real end.
	buf, err := io.ReadAll(io.NewSectionReader(im.r, off, int64(size)))
	if err != nil {
		return nil, fmt.Errorf("pe: reading RVA %#x: %w", rva, err)
	}

	if uint64(len(buf)) != uint64(size) {
		return nil, fmt.Errorf("pe: reading RVA %#x: %w", rva, io.ErrUnexpectedEOF)
	}

	return buf, nil
}

// resources is the resource directory: size bytes at rva. Its records are
// read one at a time, so that memory does not grow with the resources of
// the file (icons, manifests, embedded payloads) that are not the version.
type resources struct {
	img       *image
	rva, size uint32
}

// read returns size bytes at offset off of the resource directory.
func (r *resources) read(off, size uint32) ([]byte, error) {
	// Compare in 64 bits: a corrupt offset near 4 GiB would wrap the sum.
	if uint64(off)+uint64(size) > uint64(r.size) || uint64(r.rva)+uint64(off) > math.MaxUint32 {
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

	if dirOff+8*3 > optSize {
		return info, nil // no resource directory
	}

	resRVA := order.Uint32(opt[dirOff+8*2:])
	resourceSize := order.Uint32(opt[dirOff+8*2+4:])

	secs := make([]byte, 40*numSections)
	if _, err := rdr.ReadAt(secs, optOff+int64(optSize)); err != nil {
		return nil, fmt.Errorf("pe: section headers: %w", err)
	}

	img := &image{r: rdr}

	for i := range numSections {
		s := secs[40*i:]
		img.sections = append(img.sections, section{
			virtualSize:    order.Uint32(s[8:]),
			virtualAddress: order.Uint32(s[12:]),
			rawSize:        order.Uint32(s[16:]),
			rawPointer:     order.Uint32(s[20:]),
		})
	}

	if resRVA == 0 || resourceSize == 0 {
		return info, nil
	}

	if _, ok := img.section(resRVA, resourceSize); !ok {
		return nil, fmt.Errorf("pe: resource directory at RVA %#x is not in any section", resRVA)
	}

	if err := parseVersion(&resources{img: img, rva: resRVA, size: resourceSize}, info); err != nil {
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
		return 0, false, errors.New("pe: resource directory entries out of range")
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

// maxVersionInfo is the largest VS_VERSIONINFO: its length is a 16-bit
// field, so parseVersionInfo never looks past it.
const maxVersionInfo = 0xFFFF

func parseVersion(res *resources, info *Info) error { //nolint:cyclop // one pass over the format; splitting would scatter it
	const rtVersion = 16

	typeDir, ok, err := findEntry(res, 0, rtVersion)
	if err != nil || !ok || typeDir&0x80000000 == 0 {
		return err
	}

	nameDir, ok, err := findEntry(res, typeDir&0x7FFFFFFF, -1)
	if err != nil || !ok || nameDir&0x80000000 == 0 {
		return err
	}

	dataEntry, ok, err := findEntry(res, nameDir&0x7FFFFFFF, -1)
	if err != nil || !ok || dataEntry&0x80000000 != 0 {
		return err
	}

	data, err := versionData(res, dataEntry)
	if err != nil {
		return err
	}

	return parseVersionInfo(data, info)
}

// versionData reads the version resource that the data entry at offset
// dataEntry of the resource directory describes.
func versionData(res *resources, dataEntry uint32) ([]byte, error) {
	entry, err := res.read(dataEntry, 16)
	if err != nil {
		return nil, errors.New("pe: resource data entry out of range")
	}

	le := binary.LittleEndian
	dataRVA := le.Uint32(entry)
	dataSize := le.Uint32(entry[4:])

	// The whole declared range must lie in a section, as before, but only
	// what VS_VERSIONINFO can use is read.
	if _, ok := res.img.section(dataRVA, dataSize); !ok {
		return nil, fmt.Errorf("pe: RVA %#x is not in any section", dataRVA)
	}

	return res.img.read(dataRVA, min(dataSize, maxVersionInfo))
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

	if length > len(data) {
		length = len(data)
	}

	key, off := utf16At(data, 6)
	if key != "VS_VERSION_INFO" {
		return fmt.Errorf("pe: version resource key %q", key)
	}

	off = align4(off)

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

		blockKey, koff := utf16At(data, off+6)

		if blockKey == "VarFileInfo" {
			parseVarFileInfo(data[off:off+blockLen], align4(koff-off), info)
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

		key, koff := utf16At(data, off+6)
		koff = align4(koff)

		if key == "Translation" {
			seen := make(map[int]bool)

			for p := koff; p+4 <= koff+valueLen && p+4 <= len(data); p += 4 {
				lang := int(order.Uint16(data[p:]))
				if !seen[lang] {
					seen[lang] = true
					info.Languages = append(info.Languages, lang)
				}
			}
		}

		off = align4(off + varLen)
	}
}
