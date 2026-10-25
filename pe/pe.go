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
	// Machine is the COFF machine type.
	Machine uint16
	// Is64Bit reports a PE32+ optional header.
	Is64Bit bool
	// HasChecksum reports a non-zero optional-header checksum.
	HasChecksum bool
	// HasVersion reports an RT_VERSION resource with VS_FIXEDFILEINFO.
	HasVersion bool
	// FileVersion is "a.b.c.d" from VS_FIXEDFILEINFO, or "".
	FileVersion string
	// ProductVersion is "a.b.c.d" from VS_FIXEDFILEINFO, or "".
	ProductVersion string
	// Languages are the low words of the VarFileInfo Translation values in
	// stored order without duplicates.
	Languages []int
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

func peOffset(r io.ReaderAt) (int64, error) {
	var head [0x40]byte
	if _, err := r.ReadAt(head[:], 0); err != nil {
		return 0, ErrNotPE
	}

	if head[0] != 'M' || head[1] != 'Z' {
		return 0, ErrNotPE
	}

	off := int64(binary.LittleEndian.Uint32(head[0x3C:]))

	var sig [4]byte
	if _, err := r.ReadAt(sig[:], off); err != nil || string(sig[:]) != "PE\x00\x00" {
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

func (im *image) read(rva uint32, size uint32) ([]byte, error) {
	for _, s := range im.sections {
		end := s.virtualAddress + max(s.virtualSize, s.rawSize)
		if rva >= s.virtualAddress && rva+size <= end {
			off := int64(s.rawPointer) + int64(rva-s.virtualAddress)
			buf := make([]byte, size)

			if _, err := im.r.ReadAt(buf, off); err != nil {
				return nil, fmt.Errorf("pe: reading RVA %#x: %w", rva, err)
			}

			return buf, nil
		}
	}

	return nil, fmt.Errorf("pe: RVA %#x is not in any section", rva)
}

// Parse reads the headers and the version resource.
func Parse(r io.ReaderAt) (*Info, error) {
	off, err := peOffset(r)
	if err != nil {
		return nil, err
	}

	le := binary.LittleEndian

	var coff [20]byte
	if _, err := r.ReadAt(coff[:], off+4); err != nil {
		return nil, fmt.Errorf("pe: COFF header: %w", err)
	}

	info := &Info{Machine: le.Uint16(coff[0:])}
	numSections := int(le.Uint16(coff[2:]))
	optSize := int(le.Uint16(coff[16:]))
	optOff := off + 24

	if optSize < 96 {
		return nil, errors.New("pe: optional header too short")
	}

	opt := make([]byte, optSize)
	if _, err := r.ReadAt(opt, optOff); err != nil {
		return nil, fmt.Errorf("pe: optional header: %w", err)
	}

	var dirOff int

	switch le.Uint16(opt) {
	case 0x10B:
		dirOff = 96
	case 0x20B:
		dirOff = 112
		info.Is64Bit = true
	default:
		return nil, fmt.Errorf("pe: unknown optional header magic %#x", le.Uint16(opt))
	}

	info.HasChecksum = le.Uint32(opt[64:]) != 0

	if dirOff+8*3 > optSize {
		return info, nil // no resource directory
	}

	resRVA := le.Uint32(opt[dirOff+8*2:])
	resSize := le.Uint32(opt[dirOff+8*2+4:])

	secs := make([]byte, 40*numSections)
	if _, err := r.ReadAt(secs, optOff+int64(optSize)); err != nil {
		return nil, fmt.Errorf("pe: section headers: %w", err)
	}

	im := &image{r: r}

	for i := 0; i < numSections; i++ {
		s := secs[40*i:]
		im.sections = append(im.sections, section{
			virtualSize:    le.Uint32(s[8:]),
			virtualAddress: le.Uint32(s[12:]),
			rawSize:        le.Uint32(s[16:]),
			rawPointer:     le.Uint32(s[20:]),
		})
	}

	if resRVA == 0 || resSize == 0 {
		return info, nil
	}

	res, err := im.read(resRVA, resSize)
	if err != nil {
		return nil, err
	}

	if err := parseVersion(im, res, resRVA, info); err != nil {
		return nil, err
	}

	return info, nil
}

// findEntry returns the offset of a resource directory entry's target. With
// id < 0 the first entry is taken.
func findEntry(res []byte, dir uint32, id int) (target uint32, ok bool, err error) {
	if int(dir)+16 > len(res) {
		return 0, false, errors.New("pe: resource directory out of range")
	}

	le := binary.LittleEndian
	named := int(le.Uint16(res[dir+12:]))
	ids := int(le.Uint16(res[dir+14:]))
	entries := res[dir+16:]

	if 8*(named+ids) > len(entries) {
		return 0, false, errors.New("pe: resource directory entries out of range")
	}

	for i := 0; i < named+ids; i++ {
		name := le.Uint32(entries[8*i:])
		off := le.Uint32(entries[8*i+4:])

		if id < 0 || (name&0x80000000 == 0 && int(name) == id) {
			return off, true, nil
		}
	}

	return 0, false, nil
}

func parseVersion(im *image, res []byte, resRVA uint32, info *Info) error {
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

	if int(dataEntry)+16 > len(res) {
		return errors.New("pe: resource data entry out of range")
	}

	le := binary.LittleEndian
	dataRVA := le.Uint32(res[dataEntry:])
	dataSize := le.Uint32(res[dataEntry+4:])

	var data []byte

	if dataRVA >= resRVA && dataRVA+dataSize <= resRVA+uint32(len(res)) {
		data = res[dataRVA-resRVA : dataRVA-resRVA+dataSize]
	} else {
		data, err = im.read(dataRVA, dataSize)
		if err != nil {
			return err
		}
	}

	return parseVersionInfo(data, info)
}

func utf16At(b []byte, off int) (string, int) {
	var units []uint16

	for ; off+2 <= len(b); off += 2 {
		u := binary.LittleEndian.Uint16(b[off:])
		if u == 0 {
			off += 2

			break
		}

		units = append(units, u)
	}

	return string(utf16.Decode(units)), off
}

func align4(n int) int { return (n + 3) &^ 3 }

// parseVersionInfo reads VS_VERSIONINFO.
func parseVersionInfo(b []byte, info *Info) error {
	if len(b) < 6 {
		return errors.New("pe: version resource too short")
	}

	le := binary.LittleEndian
	length := int(le.Uint16(b))
	valueLen := int(le.Uint16(b[2:]))

	if length > len(b) {
		length = len(b)
	}

	key, off := utf16At(b, 6)
	if key != "VS_VERSION_INFO" {
		return fmt.Errorf("pe: version resource key %q", key)
	}

	off = align4(off)

	if valueLen >= 52 && off+52 <= length && le.Uint32(b[off:]) == 0xFEEF04BD {
		fv := func(ms, ls uint32) string {
			return fmt.Sprintf("%d.%d.%d.%d", ms>>16, ms&0xFFFF, ls>>16, ls&0xFFFF)
		}

		info.HasVersion = true
		info.FileVersion = fv(le.Uint32(b[off+8:]), le.Uint32(b[off+12:]))
		info.ProductVersion = fv(le.Uint32(b[off+16:]), le.Uint32(b[off+20:]))
	}

	off = align4(off + valueLen)

	// Children: StringFileInfo and VarFileInfo blocks.
	for off+6 <= length {
		blockLen := int(le.Uint16(b[off:]))
		if blockLen < 6 || off+blockLen > length {
			break
		}

		blockKey, koff := utf16At(b, off+6)

		if blockKey == "VarFileInfo" {
			parseVarFileInfo(b[off:off+blockLen], align4(koff-off), info)
		}

		off = align4(off + blockLen)
	}

	return nil
}

func parseVarFileInfo(b []byte, off int, info *Info) {
	le := binary.LittleEndian

	for off+6 <= len(b) {
		varLen := int(le.Uint16(b[off:]))
		valueLen := int(le.Uint16(b[off+2:]))

		if varLen < 6 || off+varLen > len(b) {
			return
		}

		key, koff := utf16At(b, off+6)
		koff = align4(koff)

		if key == "Translation" {
			seen := map[int]bool{}

			for p := koff; p+4 <= koff+valueLen && p+4 <= len(b); p += 4 {
				lang := int(le.Uint16(b[p:]))
				if !seen[lang] {
					seen[lang] = true
					info.Languages = append(info.Languages, lang)
				}
			}
		}

		off = align4(off + varLen)
	}
}
