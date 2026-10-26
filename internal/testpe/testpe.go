// Package testpe synthesizes small Portable Executable files for tests: a
// valid PE32+ image with one .rsrc section holding a VS_VERSIONINFO resource.
// The image is not runnable, but msi.dll's version reader and go-msi's pe
// package both accept it.
package testpe

import (
	"encoding/binary"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Options describe the synthesized file.
type Options struct {
	// FileVersion is "a.b.c.d"; "" writes no version resource.
	FileVersion string
	// ProductVersion defaults to FileVersion.
	ProductVersion string
	// Languages are the Translation values; nil writes none.
	Languages []int
	// Machine defaults to 0x8664 (x64).
	Machine uint16
	// Checksum is written to the optional header.
	Checksum uint32
	// Padding adds bytes to the end of the file, to vary its size.
	Padding int
}

func u16s(s string) []byte {
	var out []byte
	for _, u := range utf16.Encode([]rune(s)) {
		out = binary.LittleEndian.AppendUint16(out, u)
	}

	return append(out, 0, 0)
}

func pad4(b []byte) []byte {
	for len(b)%4 != 0 {
		b = append(b, 0)
	}

	return b
}

func parseVersion(s string) (uint32, uint32) {
	var fields [4]uint32

	for i, p := range strings.Split(s, ".") {
		if i < 4 {
			n, _ := strconv.Atoi(p)
			fields[i] = uint32(n) //nolint:gosec // version fields are small
		}
	}

	return fields[0]<<16 | fields[1], fields[2]<<16 | fields[3]
}

// versionInfo builds the VS_VERSIONINFO block.
func versionInfo(opts Options) []byte {
	order := binary.LittleEndian

	fixed := make([]byte, 52)
	order.PutUint32(fixed, 0xFEEF04BD)
	order.PutUint32(fixed[4:], 0x00010000)

	verHi, verLo := parseVersion(opts.FileVersion)
	order.PutUint32(fixed[8:], verHi)
	order.PutUint32(fixed[12:], verLo)

	pv := opts.ProductVersion
	if pv == "" {
		pv = opts.FileVersion
	}

	verHi, verLo = parseVersion(pv)
	order.PutUint32(fixed[16:], verHi)
	order.PutUint32(fixed[20:], verLo)
	order.PutUint32(fixed[36:], 0x40004) // VOS_NT_WINDOWS32
	order.PutUint32(fixed[40:], 1)       // VFT_APP

	var children []byte

	if opts.Languages != nil {
		var trans []byte
		for _, l := range opts.Languages {
			trans = order.AppendUint32(trans, uint32(l)|0x04B0<<16) //nolint:gosec // a language ID is 16 bits
		}

		varBlock := []byte{0, 0, 0, 0, 0, 0}
		varBlock = append(varBlock, u16s("Translation")...)
		varBlock = pad4(varBlock)
		varBlock = append(varBlock, trans...)
		order.PutUint16(varBlock, uint16(len(varBlock)))  //nolint:gosec // a resource is far below 64 KiB
		order.PutUint16(varBlock[2:], uint16(len(trans))) //nolint:gosec // a resource is far below 64 KiB

		vfi := []byte{0, 0, 0, 0, 0, 0}
		vfi[4] = 1
		vfi = append(vfi, u16s("VarFileInfo")...)
		vfi = pad4(vfi)
		vfi = append(vfi, varBlock...)
		order.PutUint16(vfi, uint16(len(vfi))) //nolint:gosec // a resource is far below 64 KiB
		children = append(children, pad4(vfi)...)
	}

	out := []byte{0, 0, 0, 0, 0, 0}
	out = append(out, u16s("VS_VERSION_INFO")...)
	out = pad4(out)
	out = append(out, fixed...)
	out = pad4(out)
	out = append(out, children...)
	order.PutUint16(out, uint16(len(out))) //nolint:gosec // a resource is far below 64 KiB
	order.PutUint16(out[2:], 52)

	return out
}

// resourceSection returns the .rsrc section bytes holding the version
// resource, or nil when opts has no version. The data entry points at
// dataRVA plus the offset of the data within the section; relocOff is the
// offset of that field, for an object file that relocates it.
func resourceSection(opts Options, dataRVA uint32) ([]byte, int) {
	if opts.FileVersion == "" {
		return nil, 0
	}

	var rsrc []byte

	order := binary.LittleEndian

	// Directory (type 16) -> directory (name 1) -> directory (lang) ->
	// data entry -> data.
	verInfo := versionInfo(opts)
	dir := func(entryName, entryOff uint32) []byte {
		d := make([]byte, 16)
		order.PutUint16(d[14:], 1)
		d = order.AppendUint32(d, entryName)
		d = order.AppendUint32(d, entryOff)

		return d
	}

	lang := uint32(1033)
	if len(opts.Languages) > 0 {
		lang = uint32(opts.Languages[0]) //nolint:gosec // a language ID is 16 bits
	}

	// Offsets: root 0 (24 bytes), type dir 24, name dir 48, data entry 72, data 88.
	rsrc = append(rsrc, dir(16, 0x80000000|24)...)
	rsrc = append(rsrc, dir(1, 0x80000000|48)...)
	rsrc = append(rsrc, dir(lang, 72)...)
	rsrc = order.AppendUint32(rsrc, dataRVA+88)
	rsrc = order.AppendUint32(rsrc, uint32(len(verInfo))) //nolint:gosec // a resource is small
	rsrc = order.AppendUint32(rsrc, 0)
	rsrc = order.AppendUint32(rsrc, 0)
	rsrc = append(rsrc, verInfo...)

	return rsrc, 72
}

// Syso returns a COFF object file holding the version resource, for the Go
// linker: dropped into a package directory as *_windows_amd64.syso, it
// gives a Go executable a version resource. The data entry carries an
// image-relative relocation against the section, which the linker resolves
// to the final RVA.
func Syso(opts Options) []byte {
	order := binary.LittleEndian

	if opts.Machine == 0 {
		opts.Machine = 0x8664
	}

	rsrc, relocOff := resourceSection(opts, 0)

	const (
		coffHeader    = 20
		sectionHeader = 40
		relocSize     = 10
		symbolSize    = 18
	)

	const rawOff = coffHeader + sectionHeader

	relocPtr := rawOff + len(rsrc)
	symPtr := relocPtr + relocSize

	header := make([]byte, coffHeader+sectionHeader)
	order.PutUint16(header, opts.Machine)
	order.PutUint16(header[2:], 1)
	order.PutUint32(header[8:], uint32(symPtr)) //nolint:gosec // an object file is small
	order.PutUint32(header[12:], 1)

	sec := header[coffHeader:]
	copy(sec, ".rsrc")
	order.PutUint32(sec[16:], uint32(len(rsrc))) //nolint:gosec // small
	order.PutUint32(sec[20:], rawOff)
	order.PutUint32(sec[24:], uint32(relocPtr)) //nolint:gosec // small
	order.PutUint16(sec[32:], 1)
	order.PutUint32(sec[36:], 0x40000040) // initialized data, readable

	// One relocation: the data entry's OffsetToData against symbol 0, as an
	// image-relative 32-bit address (IMAGE_REL_AMD64_ADDR32NB, or the ARM64
	// equivalent).
	relocType := uint16(3)
	if opts.Machine == 0xAA64 {
		relocType = 2 // IMAGE_REL_ARM64_ADDR32NB
	}

	reloc := order.AppendUint32(nil, uint32(relocOff)) //nolint:gosec // small
	reloc = order.AppendUint32(reloc, 0)
	reloc = order.AppendUint16(reloc, relocType)

	// Symbol 0: the section itself, static.
	sym := make([]byte, symbolSize)
	copy(sym, ".rsrc")
	order.PutUint16(sym[12:], 1) // section number
	sym[16] = 3                  // IMAGE_SYM_CLASS_STATIC

	// The string table holds its length field only.
	strTable := order.AppendUint32(nil, 4)

	return slices.Concat(header, rsrc, reloc, sym, strTable)
}

// Build returns the bytes of a synthetic PE file.
func Build(opts Options) []byte {
	order := binary.LittleEndian

	if opts.Machine == 0 {
		opts.Machine = 0x8664
	}

	const (
		fileAlign    = 0x200
		sectionAlign = 0x1000
		headersSize  = 0x400
		textRVA      = 0x1000
		rsrcRVA      = 0x2000
	)

	rsrc, _ := resourceSection(opts, rsrcRVA)

	text := []byte{0xC3} // ret
	textRaw := roundUp(len(text), fileAlign)
	rsrcRaw := roundUp(len(rsrc), fileAlign)

	out := make([]byte, headersSize+textRaw+rsrcRaw+opts.Padding)

	// DOS header.
	copy(out, "MZ")
	order.PutUint32(out[0x3C:], 0x80)

	header := out[0x80:]
	copy(header, "PE\x00\x00")
	order.PutUint16(header[4:], opts.Machine)
	order.PutUint16(header[6:], 2) // sections
	order.PutUint16(header[20:], 240)
	order.PutUint16(header[22:], 0x0022) // executable, large address aware

	opt := header[24:]
	order.PutUint16(opt, 0x20B)
	opt[2] = 14
	order.PutUint32(opt[4:], uint32(textRaw)) //nolint:gosec // section sizes are small
	order.PutUint32(opt[8:], uint32(rsrcRaw)) //nolint:gosec // section sizes are small
	order.PutUint32(opt[16:], textRVA)
	order.PutUint32(opt[20:], textRVA)
	order.PutUint64(opt[24:], 0x140000000)
	order.PutUint32(opt[32:], sectionAlign)
	order.PutUint32(opt[36:], fileAlign)
	order.PutUint16(opt[40:], 6)
	order.PutUint16(opt[48:], 6)
	order.PutUint32(opt[56:], rsrcRVA+uint32(roundUp(max(len(rsrc), 1), sectionAlign))) //nolint:gosec // bounded by the format limits checked earlier
	order.PutUint32(opt[60:], headersSize)
	order.PutUint32(opt[64:], opts.Checksum)
	order.PutUint16(opt[68:], 3) // console subsystem
	order.PutUint64(opt[72:], 0x100000)
	order.PutUint64(opt[80:], 0x1000)
	order.PutUint64(opt[88:], 0x100000)
	order.PutUint64(opt[96:], 0x1000)
	order.PutUint32(opt[108:], 16) // data directories

	if len(rsrc) > 0 {
		order.PutUint32(opt[112+8*2:], rsrcRVA)
		order.PutUint32(opt[112+8*2+4:], uint32(len(rsrc))) //nolint:gosec // bounded by the format limits checked earlier
	}

	sec := header[24+240:]
	writeSection := func(section []byte, name string, vsize, rva, rawSize, rawPtr uint32, flags uint32) {
		copy(section, name)
		order.PutUint32(section[8:], vsize)
		order.PutUint32(section[12:], rva)
		order.PutUint32(section[16:], rawSize)
		order.PutUint32(section[20:], rawPtr)
		order.PutUint32(section[36:], flags)
	}

	writeSection(sec, ".text", uint32(len(text)), textRVA, uint32(textRaw), headersSize, 0x60000020)                              //nolint:gosec // small
	writeSection(sec[40:], ".rsrc", uint32(max(len(rsrc), 1)), rsrcRVA, uint32(rsrcRaw), uint32(headersSize+textRaw), 0x40000040) //nolint:gosec // small

	copy(out[headersSize:], text)
	copy(out[headersSize+textRaw:], rsrc)

	return out
}

func roundUp(n, align int) int {
	return (n + align - 1) / align * align
}
