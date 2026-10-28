package pe //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

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
		"resource directory wraps":   {testResourceDir + 4, 0xFFFFF000},
		"resource directory huge":    {testResourceDir + 4, 0x7FFFFFFF},
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
