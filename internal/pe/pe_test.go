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

	if info.Language() != "1033,1031" {
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
		"version data size wraps":    {testDataEntry + 4, 0xFFFFFFF8},
		"version data size huge":     {testDataEntry + 4, 0x7FFFFFFF},
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
// language while its value length still claims two. The second language
// lies outside the block and must not be reported.
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

	if info.Language() != "1033" {
		t.Errorf("language %q, want 1033", info.Language())
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
		"value runs past the block": {2, 0xFFFF},
		"text type":                 {4, 1},
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
