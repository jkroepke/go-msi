// Package testpe synthesizes small Portable Executable files for tests: a
// valid PE32+ image with one .rsrc section holding a VS_VERSIONINFO resource.
// The image is not runnable, but msi.dll's version reader and go-msi's pe
// package both accept it.
package testpe

import (
	"encoding/binary"
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

func parseVersion(s string) (ms, ls uint32) {
	var f [4]uint32

	for i, p := range strings.Split(s, ".") {
		if i < 4 {
			n, _ := strconv.Atoi(p)
			f[i] = uint32(n)
		}
	}

	return f[0]<<16 | f[1], f[2]<<16 | f[3]
}

// versionInfo builds the VS_VERSIONINFO block.
func versionInfo(o Options) []byte {
	le := binary.LittleEndian

	fixed := make([]byte, 52)
	le.PutUint32(fixed, 0xFEEF04BD)
	le.PutUint32(fixed[4:], 0x00010000)

	ms, ls := parseVersion(o.FileVersion)
	le.PutUint32(fixed[8:], ms)
	le.PutUint32(fixed[12:], ls)

	pv := o.ProductVersion
	if pv == "" {
		pv = o.FileVersion
	}

	ms, ls = parseVersion(pv)
	le.PutUint32(fixed[16:], ms)
	le.PutUint32(fixed[20:], ls)
	le.PutUint32(fixed[36:], 0x40004) // VOS_NT_WINDOWS32
	le.PutUint32(fixed[40:], 1)       // VFT_APP

	var children []byte

	if o.Languages != nil {
		var trans []byte
		for _, l := range o.Languages {
			trans = le.AppendUint32(trans, uint32(l)|0x04B0<<16)
		}

		v := make([]byte, 6)
		v = append(v, u16s("Translation")...)
		v = pad4(v)
		v = append(v, trans...)
		le.PutUint16(v, uint16(len(v)))
		le.PutUint16(v[2:], uint16(len(trans)))

		vfi := make([]byte, 6)
		vfi[4] = 1
		vfi = append(vfi, u16s("VarFileInfo")...)
		vfi = pad4(vfi)
		vfi = append(vfi, v...)
		le.PutUint16(vfi, uint16(len(vfi)))
		children = append(children, pad4(vfi)...)
	}

	out := make([]byte, 6)
	out = append(out, u16s("VS_VERSION_INFO")...)
	out = pad4(out)
	out = append(out, fixed...)
	out = pad4(out)
	out = append(out, children...)
	le.PutUint16(out, uint16(len(out)))
	le.PutUint16(out[2:], 52)

	return out
}

// Build returns the bytes of a synthetic PE file.
func Build(o Options) []byte {
	le := binary.LittleEndian

	if o.Machine == 0 {
		o.Machine = 0x8664
	}

	const (
		fileAlign    = 0x200
		sectionAlign = 0x1000
		headersSize  = 0x400
		textRVA      = 0x1000
		rsrcRVA      = 0x2000
	)

	// Resource section: directory (type 16) -> directory (name 1) ->
	// directory (lang) -> data entry -> data.
	var rsrc []byte

	if o.FileVersion != "" {
		vi := versionInfo(o)
		dir := func(entryName, entryOff uint32) []byte {
			d := make([]byte, 16)
			le.PutUint16(d[14:], 1)
			d = le.AppendUint32(d, entryName)
			d = le.AppendUint32(d, entryOff)

			return d
		}

		lang := uint32(1033)
		if len(o.Languages) > 0 {
			lang = uint32(o.Languages[0])
		}

		// Offsets: root 0 (24 bytes), type dir 24, name dir 48, data entry 72, data 88.
		rsrc = append(rsrc, dir(16, 0x80000000|24)...)
		rsrc = append(rsrc, dir(1, 0x80000000|48)...)
		rsrc = append(rsrc, dir(lang, 72)...)
		rsrc = le.AppendUint32(rsrc, rsrcRVA+88)
		rsrc = le.AppendUint32(rsrc, uint32(len(vi)))
		rsrc = le.AppendUint32(rsrc, 0)
		rsrc = le.AppendUint32(rsrc, 0)
		rsrc = append(rsrc, vi...)
	}

	text := []byte{0xC3} // ret
	textRaw := roundUp(len(text), fileAlign)
	rsrcRaw := roundUp(len(rsrc), fileAlign)

	out := make([]byte, headersSize+textRaw+rsrcRaw+o.Padding)

	// DOS header.
	copy(out, "MZ")
	le.PutUint32(out[0x3C:], 0x80)

	pe := out[0x80:]
	copy(pe, "PE\x00\x00")
	le.PutUint16(pe[4:], o.Machine)
	le.PutUint16(pe[6:], 2) // sections
	le.PutUint16(pe[20:], 240)
	le.PutUint16(pe[22:], 0x0022) // executable, large address aware

	opt := pe[24:]
	le.PutUint16(opt, 0x20B)
	opt[2] = 14
	le.PutUint32(opt[4:], uint32(textRaw))
	le.PutUint32(opt[8:], uint32(rsrcRaw))
	le.PutUint32(opt[16:], textRVA)
	le.PutUint32(opt[20:], textRVA)
	le.PutUint64(opt[24:], 0x140000000)
	le.PutUint32(opt[32:], sectionAlign)
	le.PutUint32(opt[36:], fileAlign)
	le.PutUint16(opt[40:], 6)
	le.PutUint16(opt[48:], 6)
	le.PutUint32(opt[56:], rsrcRVA+uint32(roundUp(max(len(rsrc), 1), sectionAlign)))
	le.PutUint32(opt[60:], headersSize)
	le.PutUint32(opt[64:], o.Checksum)
	le.PutUint16(opt[68:], 3) // console subsystem
	le.PutUint64(opt[72:], 0x100000)
	le.PutUint64(opt[80:], 0x1000)
	le.PutUint64(opt[88:], 0x100000)
	le.PutUint64(opt[96:], 0x1000)
	le.PutUint32(opt[108:], 16) // data directories

	if len(rsrc) > 0 {
		le.PutUint32(opt[112+8*2:], rsrcRVA)
		le.PutUint32(opt[112+8*2+4:], uint32(len(rsrc)))
	}

	sec := pe[24+240:]
	writeSection := func(s []byte, name string, vsize, rva, rawSize, rawPtr uint32, flags uint32) {
		copy(s, name)
		le.PutUint32(s[8:], vsize)
		le.PutUint32(s[12:], rva)
		le.PutUint32(s[16:], rawSize)
		le.PutUint32(s[20:], rawPtr)
		le.PutUint32(s[36:], flags)
	}

	writeSection(sec, ".text", uint32(len(text)), textRVA, uint32(textRaw), headersSize, 0x60000020)
	writeSection(sec[40:], ".rsrc", uint32(max(len(rsrc), 1)), rsrcRVA, uint32(rsrcRaw), uint32(headersSize+textRaw), 0x40000040)

	copy(out[headersSize:], text)
	copy(out[headersSize+textRaw:], rsrc)

	return out
}

func roundUp(n, align int) int {
	return (n + align - 1) / align * align
}
