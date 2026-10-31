package pe //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

func TestSyntheticPE(t *testing.T) {
	t.Parallel()

	data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", ProductVersion: "1.2.0.0", Languages: []int{1033, 1031, 1033}, Checksum: 0x1234})

	info, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if !info.HasVersion || info.FileVersion != "1.2.3.4" || info.ProductVersion != "1.2.0.0" {
		t.Errorf("version: %+v", info)
	}

	// Stored 1033,1031,1033: duplicates dropped, last first, as Windows
	// lists them (FILE-14).
	if info.Language() != "1031,1033" {
		t.Errorf("language %q", info.Language())
	}

	if !info.HasChecksum || !info.Is64Bit || info.Machine != MachineAMD64 {
		t.Errorf("header: %+v", info)
	}

	if !IsPE(bytes.NewReader(data)) {
		t.Error("IsPE false")
	}
}

func TestNoVersionResource(t *testing.T) {
	t.Parallel()

	data := testpe.Build(testpe.Options{Machine: MachineI386})

	info, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if info.HasVersion || info.FileVersion != "" || info.Language() != "" || info.HasChecksum {
		t.Errorf("%+v", info)
	}
}

func TestNoTranslations(t *testing.T) {
	t.Parallel()

	data := testpe.Build(testpe.Options{FileVersion: "0.32.0.5"})

	info, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if info.FileVersion != "0.32.0.5" || info.Language() != "" {
		t.Errorf("%+v", info)
	}
}

func TestNotPE(t *testing.T) {
	t.Parallel()

	for _, data := range [][]byte{nil, []byte("hello"), append([]byte("MZ"), make([]byte, 100)...)} {
		if _, err := Parse(bytes.NewReader(data)); !errors.Is(err, ErrNotPE) {
			t.Errorf("%q: %v", data, err)
		}

		if IsPE(bytes.NewReader(data)) {
			t.Errorf("%q: IsPE true", data)
		}
	}
}

// Offsets into a testpe image, derived from its fixed layout: headers are
// 0x400 bytes, .text takes one 0x200 file-aligned block and .rsrc follows.
const (
	testSectionTable = 0x80 + 24 + 240 // PE header, COFF header, optional header
	testRsrcSection  = testSectionTable + 40
	testRsrcRaw      = 0x400 + 0x200
	testRootEntry    = testRsrcRaw + 16 + 4 // offset field of the root directory's only entry
	testDataEntry    = testRsrcRaw + 72     // data entry: RVA, size
	testResourceDir  = 0x80 + 24 + 112 + 8*2
)

// corruptImage builds a synthetic image and overwrites one 32-bit header
// field, checking first that the layout constants still point where they
// claim to.
func corruptImage(t *testing.T, off int, value uint32) []byte {
	t.Helper()

	data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}})

	if binary.LittleEndian.Uint32(data[testDataEntry:]) != 0x2000+88 {
		t.Fatalf("data entry RVA %#x: layout constants are stale", binary.LittleEndian.Uint32(data[testDataEntry:]))
	}

	if binary.LittleEndian.Uint32(data[testRootEntry:]) != 0x80000000|24 {
		t.Fatalf("root entry %#x: layout constants are stale", binary.LittleEndian.Uint32(data[testRootEntry:]))
	}

	if string(data[testRsrcSection:testRsrcSection+5]) != ".rsrc" {
		t.Fatal("section table offset is stale")
	}

	binary.LittleEndian.PutUint32(data[off:], value)

	return data
}

// TestCorruptSizes feeds the parser header fields whose 32-bit values wrap
// around when added to their offsets, or that point outside the file. Each
// must come back as an error, never as a panic or a huge allocation.
func TestCorruptSizes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		off   int
		value uint32
	}{
		"version data size short":    {testDataEntry + 4, 0x20},
		"version data RVA wraps":     {testDataEntry, 0xFFFFFFF0},
		"version data RVA elsewhere": {testDataEntry, 0x1000},
		"directory entry wraps int":  {testRootEntry, 0x80000000 | 0x7FFFFFF8},
		"directory entry past end":   {testRootEntry, 0x80000000 | 0x10000},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse(bytes.NewReader(corruptImage(t, tc.off, tc.value))); err == nil {
				t.Error("corrupt image parsed without error")
			}
		})
	}
}

// TestOversizedSections patches section sizes that wrap around in 32 bits.
// The resource data still lies inside the file, so the parser may either
// read it or refuse the section, but it must not panic.
func TestOversizedSections(t *testing.T) {
	t.Parallel()

	for name, off := range map[string]int{"virtual size": testRsrcSection + 8, "raw size": testRsrcSection + 16} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			info, err := Parse(bytes.NewReader(corruptImage(t, off, 0xFFFFFFFF)))
			if err == nil && info.FileVersion != "1.2.3.4" {
				t.Errorf("parsed but lost the version: %+v", info)
			}
		})
	}
}

func TestIsDLL(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{})
	if IsDLL(bytes.NewReader(exe)) {
		t.Error("an EXE reads as a DLL")
	}

	// COFF Characteristics: PE offset + signature + 18 bytes; 0x2000 is
	// IMAGE_FILE_DLL.
	dll := slices.Clone(exe)
	off := int(binary.LittleEndian.Uint32(dll[0x3C:]))
	dll[off+4+18+1] |= 0x20

	if !IsDLL(bytes.NewReader(dll)) {
		t.Error("a DLL does not read as one")
	}

	if IsDLL(bytes.NewReader([]byte("MZ"))) {
		t.Error("a truncated file reads as a DLL")
	}
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r    io.ReaderAt
	read int64
}

func (c *countingReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(p, off)
	c.read += int64(n)

	return n, err
}

// TestLargeResourceRange parses a PE whose resource directory covers a
// 32 MiB section: only the directory records and the version resource are
// read, not the whole range.
func TestLargeResourceRange(t *testing.T) {
	t.Parallel()

	const padding = 32 << 20

	data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}, Padding: padding})
	size := uint32(len(data) - testRsrcRaw) //nolint:gosec // 32 MiB

	if string(data[testRsrcSection:testRsrcSection+5]) != ".rsrc" {
		t.Fatal("section table offset is stale")
	}

	binary.LittleEndian.PutUint32(data[testRsrcSection+8:], size)  // virtual size
	binary.LittleEndian.PutUint32(data[testRsrcSection+16:], size) // raw size
	binary.LittleEndian.PutUint32(data[testResourceDir+4:], size)

	rdr := &countingReader{r: bytes.NewReader(data)}

	info, err := Parse(rdr)
	if err != nil {
		t.Fatal(err)
	}

	if info.FileVersion != "1.2.3.4" {
		t.Errorf("version %q", info.FileVersion)
	}

	if rdr.read > 64<<10 {
		t.Errorf("read %d bytes of a %d-byte resource directory", rdr.read, size)
	}
}

// TestRawDataRoundedToFileAlignment ends the resource section's raw data
// before and right after the start of the version data (at 0x58). Windows
// rounds SizeOfRawData up to FileAlignment (0x200 here), so the version is
// read either way, but it drops the languages when the data starts past
// the unrounded raw size (FILE-14).
func TestRawDataRoundedToFileAlignment(t *testing.T) {
	t.Parallel()

	for raw, lang := range map[uint32]string{1: "", 0x58: "", 0x60: "1033", 0x200: "1033"} {
		info, err := Parse(bytes.NewReader(corruptImage(t, testRsrcSection+16, raw)))
		if err != nil {
			t.Fatal(err)
		}

		if info.FileVersion != "1.2.3.4" || info.Language() != lang {
			t.Errorf("raw size %#x: %+v, want language %q", raw, info, lang)
		}
	}
}

// TestResourceBeyondRawData clears the SizeOfRawData of the resource
// section. Its VirtualSize still covers the directory, but the loader maps
// zeros there, so the version bytes that stay in the file must not be read.
func TestResourceBeyondRawData(t *testing.T) {
	t.Parallel()

	info, err := Parse(bytes.NewReader(corruptImage(t, testRsrcSection+16, 0)))
	if err != nil {
		t.Fatal(err)
	}

	if info.HasVersion || info.FileVersion != "" || info.Language() != "" {
		t.Errorf("read past the section's raw data: %+v", info)
	}
}

// TestTranslationBoundedByVar shrinks the Translation Var block to one
// language while its value length still claims two. The value runs past
// the block, so no language is reported, as by MsiGetFileVersionW.
func TestTranslationBoundedByVar(t *testing.T) {
	t.Parallel()

	data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033, 1031}})

	translation := blockOffset(t, data, "Translation")
	if binary.LittleEndian.Uint16(data[translation+2:]) != 8 {
		t.Fatal("Translation value length is not two languages")
	}

	// Header 6, key 24 with terminator, padded to 32, one language: 36.
	binary.LittleEndian.PutUint16(data[translation:], 36)

	info, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if info.FileVersion != "1.2.3.4" || info.Language() != "" {
		t.Errorf("version %q language %q, want 1.2.3.4 and none", info.FileVersion, info.Language())
	}
}

// TestOversizedResourceDirectory declares a resource directory far larger
// than its section. Windows still reads the version through the records it
// visits, which all lie in the section, so the parser must too.
func TestOversizedResourceDirectory(t *testing.T) {
	t.Parallel()

	for name, size := range map[string]uint32{"wraps": 0xFFFFF000, "huge": 0x7FFFFFFF} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			info, err := Parse(bytes.NewReader(corruptImage(t, testResourceDir+4, size)))
			if err != nil {
				t.Fatal(err)
			}

			if info.FileVersion != "1.2.3.4" || info.Language() != "1033" {
				t.Errorf("%+v", info)
			}
		})
	}
}

// TestNoResourceSlot declares fewer data directories than the resource
// slot needs. Windows ignores the slot's bytes, so there is no version.
func TestNoResourceSlot(t *testing.T) {
	t.Parallel()

	for _, count := range []uint32{0, 2} {
		info, err := Parse(bytes.NewReader(corruptImage(t, testResourceDir-20, count)))
		if err != nil {
			t.Fatal(err)
		}

		if info.HasVersion || info.FileVersion != "" {
			t.Errorf("NumberOfRvaAndSizes %d: version read from an undeclared slot: %+v", count, info)
		}
	}
}

// blockOffset returns the file offset of the version block whose key is
// key: its 6-byte header precedes the UTF-16 key.
func blockOffset(t *testing.T, data []byte, key string) int {
	t.Helper()

	units := utf16.Encode([]rune(key))
	encoded := make([]byte, 0, 2*len(units))

	for _, u := range units {
		encoded = binary.LittleEndian.AppendUint16(encoded, u)
	}

	off := bytes.Index(data, encoded) - 6
	if off < 0 {
		t.Fatalf("version block %q not found", key)
	}

	return off
}

// TestInvalidVersionHeader corrupts the VS_VERSIONINFO header in ways
// Windows refuses. The parser must not report a version, so the builder's
// unversioned-executable check still applies.
func TestInvalidVersionHeader(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field int
		value uint16
	}{
		"value runs past the block":  {2, 0xFFFF},
		"block longer than its data": {0, 0xFFFF},
		"text type":                  {4, 1},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}})
			binary.LittleEndian.PutUint16(data[blockOffset(t, data, "VS_VERSION_INFO")+tc.field:], tc.value)

			info, err := Parse(bytes.NewReader(data))
			if err == nil && info.HasVersion {
				t.Errorf("invalid header accepted: %+v", info)
			}
		})
	}
}

// TestShortOptionalHeader declares a PE32+ optional header long enough for
// the checksum but too short for NumberOfRvaAndSizes and the directories.
func TestShortOptionalHeader(t *testing.T) {
	t.Parallel()

	for _, size := range []uint16{96, 100, 111} {
		data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4"})
		binary.LittleEndian.PutUint16(data[0x80+4+16:], size) // SizeOfOptionalHeader

		info, err := Parse(bytes.NewReader(data))
		if err == nil && info.HasVersion {
			t.Errorf("size %d: version read without a resource directory: %+v", size, info)
		}
	}
}

// movedVersion builds an image whose version resource data lies at RVA
// 0x2200, past the resource section's 0x200 bytes of raw data but inside
// its VirtualSize of 0x400, with the bytes in the file right after the
// raw data. rawSize sets the section's SizeOfRawData.
func movedVersion(t *testing.T, rawSize uint32) []byte {
	t.Helper()

	data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}, Padding: 0x200})

	src := blockOffset(t, data, "VS_VERSION_INFO")
	size := int(binary.LittleEndian.Uint32(data[testDataEntry+4:]))
	copy(data[testRsrcRaw+0x200:], data[src:src+size])
	clear(data[src : src+size])

	binary.LittleEndian.PutUint32(data[testDataEntry:], 0x2200)
	binary.LittleEndian.PutUint32(data[testRsrcSection+8:], 0x400) // VirtualSize
	binary.LittleEndian.PutUint32(data[testRsrcSection+16:], rawSize)

	return data
}

// TestRecordPastRawData reads a version resource that lies past the
// section's raw data rounded to FileAlignment: it reads as zeros, so there
// is no version even though the file holds its bytes. With the raw data
// covering it, the version is read.
func TestRecordPastRawData(t *testing.T) {
	t.Parallel()

	info, err := Parse(bytes.NewReader(movedVersion(t, 0x200)))
	if err == nil && info.HasVersion {
		t.Errorf("read past the raw data: %+v", info)
	}

	info, err = Parse(bytes.NewReader(movedVersion(t, 0x400)))
	if err != nil || info.FileVersion != "1.2.3.4" || info.Language() != "1033" {
		t.Errorf("covered by the raw data: %+v, %v", info, err)
	}
}

// TestInvalidFileAlignment falls back to the unrounded raw size when
// FileAlignment is not a power of two from 512 to 64 KiB.
func TestInvalidFileAlignment(t *testing.T) {
	t.Parallel()

	data := corruptImage(t, testRsrcSection+16, 1)
	binary.LittleEndian.PutUint32(data[0x80+24+36:], 0) // FileAlignment

	info, err := Parse(bytes.NewReader(data))
	if err == nil && info.HasVersion {
		t.Errorf("one raw byte gave a version: %+v", info)
	}
}

// TestOversizedVersionData declares more version data than the section
// holds, and a resource directory of size 0. Windows reads only the block
// VS_VERSIONINFO declares and ignores the directory size (FILE-14).
func TestOversizedVersionData(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		off   int
		value uint32
	}{
		"version data size 512":   {testDataEntry + 4, 512},
		"version data size huge":  {testDataEntry + 4, 0x7FFFFFFF},
		"version data size wraps": {testDataEntry + 4, 0xFFFFFFF8},
		"directory size 0":        {testResourceDir + 4, 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			info, err := Parse(bytes.NewReader(corruptImage(t, tc.off, tc.value)))
			if err != nil {
				t.Fatal(err)
			}

			if info.FileVersion != "1.2.3.4" || info.Language() != "1033" {
				t.Errorf("%+v", info)
			}
		})
	}
}

// resEntry is one RT_VERSION resource for versionResources.
type resEntry struct {
	id, lang  uint32
	version   string
	languages []int
}

// versionResources builds an image whose resource section holds the given
// RT_VERSION resources, sorted by ID and then language as a resource
// compiler writes them.
func versionResources(t *testing.T, entries []resEntry) []byte {
	t.Helper()

	const area = 0x1000 // the section's raw size: 0x200 plus the padding

	data := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Padding: area - 0x200})
	order := binary.LittleEndian

	slices.SortFunc(entries, func(a, b resEntry) int {
		if a.id != b.id {
			return int(a.id) - int(b.id)
		}

		return int(a.lang) - int(b.lang)
	})

	var ids []uint32
	for _, e := range entries {
		if len(ids) == 0 || ids[len(ids)-1] != e.id {
			ids = append(ids, e.id)
		}
	}

	dirHead := func(n int) []byte {
		d := make([]byte, 16)
		order.PutUint16(d[14:], uint16(n)) //nolint:gosec // a few entries

		return d
	}

	// Layout: root, name directory, one language directory per ID, the
	// data entries, then the data.
	rootLen := 16 + 8
	nameLen := 16 + 8*len(ids)
	langOff := make(map[uint32]int)
	off := rootLen + nameLen

	for _, id := range ids {
		langOff[id] = off
		count := 0

		for _, e := range entries {
			if e.id == id {
				count++
			}
		}

		off += 16 + 8*count
	}

	entryOff := off
	dataOff := entryOff + 16*len(entries)

	rsrc := dirHead(1)
	rsrc = order.AppendUint32(rsrc, 16)
	rsrc = order.AppendUint32(rsrc, 0x80000000|uint32(rootLen))
	rsrc = append(rsrc, dirHead(len(ids))...)

	for _, id := range ids {
		rsrc = order.AppendUint32(rsrc, id)
		rsrc = order.AppendUint32(rsrc, 0x80000000|uint32(langOff[id])) //nolint:gosec // small
	}

	blobs := make([][]byte, 0, len(entries))

	for _, e := range entries {
		img := testpe.Build(testpe.Options{FileVersion: e.version, Languages: e.languages})
		start := blockOffset(t, img, "VS_VERSION_INFO")
		blobs = append(blobs, img[start:start+int(order.Uint32(img[testDataEntry+4:]))])
	}

	for _, id := range ids {
		var langs []int

		for i, e := range entries {
			if e.id == id {
				langs = append(langs, i)
			}
		}

		rsrc = append(rsrc, dirHead(len(langs))...)
		for _, i := range langs {
			rsrc = order.AppendUint32(rsrc, entries[i].lang)
			rsrc = order.AppendUint32(rsrc, uint32(entryOff+16*i)) //nolint:gosec // small
		}
	}

	pos := dataOff

	for _, b := range blobs {
		rsrc = order.AppendUint32(rsrc, 0x2000+uint32(pos)) //nolint:gosec // small
		rsrc = order.AppendUint32(rsrc, uint32(len(b)))     //nolint:gosec // small
		rsrc = append(rsrc, make([]byte, 8)...)
		pos = align4(pos + len(b))
	}

	for _, b := range blobs {
		rsrc = append(rsrc, b...)
		for len(rsrc)%4 != 0 {
			rsrc = append(rsrc, 0)
		}
	}

	if len(rsrc) > area {
		t.Fatalf("resources of %d bytes do not fit", len(rsrc))
	}

	clear(data[testRsrcRaw : testRsrcRaw+area])
	copy(data[testRsrcRaw:], rsrc)
	order.PutUint32(data[testRsrcSection+8:], area)
	order.PutUint32(data[testRsrcSection+16:], area)
	order.PutUint32(data[testResourceDir+4:], uint32(len(rsrc))) //nolint:gosec // small

	return data
}

// TestVersionResourceSelection reads RT_VERSION ID 1 only, and refuses
// language entries that disagree, since Windows picks one by the user's
// UI language (FILE-14).
func TestVersionResourceSelection(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		entries []resEntry
		version string
		lang    string
		fails   bool
	}{
		"ID 0 and ID 1": {
			entries: []resEntry{{0, 1033, "1.2.3.4", []int{1033}}, {1, 1033, "9.8.7.6", []int{1033}}},
			version: "9.8.7.6", lang: "1033",
		},
		"ID 2 only": {entries: []resEntry{{2, 1033, "1.2.3.4", []int{1033}}}},
		"two languages agree": {
			entries: []resEntry{{1, 1031, "1.2.3.4", []int{1031, 1033}}, {1, 1033, "1.2.3.4", []int{1031, 1033}}},
			version: "1.2.3.4", lang: "1033,1031",
		},
		"two languages disagree": {
			entries: []resEntry{{1, 1031, "1.2.3.4", []int{1031}}, {1, 1033, "9.8.7.6", []int{1033}}},
			fails:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			info, err := Parse(bytes.NewReader(versionResources(t, tc.entries)))

			switch {
			case tc.fails:
				if err == nil {
					t.Errorf("disagreeing languages accepted: %+v", info)
				}
			case err != nil:
				t.Fatal(err)
			case info.FileVersion != tc.version || info.Language() != tc.lang:
				t.Errorf("version %q language %q, want %q %q", info.FileVersion, info.Language(), tc.version, tc.lang)
			}
		})
	}
}

// partialTranslation sets the Translation value length of a two-language
// image to n bytes, inside its Var block.
func partialTranslation(t *testing.T, n uint16) []byte {
	t.Helper()

	data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033, 1031}})
	binary.LittleEndian.PutUint16(data[blockOffset(t, data, "Translation")+2:], n)

	return data
}

// TestPartialTranslation counts a partial last Translation entry, as
// Windows did for value lengths 5 to 7 (FILE-14).
func TestPartialTranslation(t *testing.T) {
	t.Parallel()

	for n, want := range map[uint16]string{4: "1033", 5: "1031,1033", 7: "1031,1033", 8: "1031,1033"} {
		info, err := Parse(bytes.NewReader(partialTranslation(t, n)))
		if err != nil || info.Language() != want {
			t.Errorf("value length %d: %+v %v, want %q", n, info, err, want)
		}
	}
}
