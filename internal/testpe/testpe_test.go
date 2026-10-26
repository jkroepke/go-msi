package testpe_test

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"testing"

	"github.com/jkroepke/go-msi/internal/testpe"
)

func TestSysoIsACOFFObjectWithOneRelocatedResourceSection(t *testing.T) {
	t.Parallel()

	data := testpe.Syso(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}})

	obj, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("debug/pe rejects the object: %v", err)
	}

	if obj.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || obj.OptionalHeader != nil || len(obj.Sections) != 1 {
		t.Fatalf("machine %#x, optional header %v, %d sections", obj.Machine, obj.OptionalHeader != nil, len(obj.Sections))
	}

	sec := obj.Sections[0]
	if sec.Name != ".rsrc" || sec.NumberOfRelocations != 1 || sec.Characteristics != 0x40000040 {
		t.Fatalf("section %q, %d relocations, characteristics %#x", sec.Name, sec.NumberOfRelocations, sec.Characteristics)
	}

	if len(obj.Symbols) != 1 || obj.Symbols[0].Name != ".rsrc" || obj.Symbols[0].SectionNumber != 1 {
		t.Fatalf("symbols %+v", obj.Symbols)
	}

	raw, err := sec.Data()
	if err != nil {
		t.Fatal(err)
	}

	// The relocated data entry points at the version data inside the section;
	// VS_FIXEDFILEINFO follows the 6-byte header and the padded key.
	reloc := data[sec.PointerToRelocations:]
	off := binary.LittleEndian.Uint32(reloc)
	typ := binary.LittleEndian.Uint16(reloc[8:])
	dataOff := binary.LittleEndian.Uint32(raw[off:])
	size := binary.LittleEndian.Uint32(raw[off+4:])

	if typ != 3 || int(dataOff)+int(size) != len(raw) || binary.LittleEndian.Uint32(raw[dataOff+40:]) != 0xFEEF04BD {
		t.Fatalf("relocation type %d at %d, data %d+%d of %d", typ, off, dataOff, size, len(raw))
	}
}

func TestBuildStillCarriesTheVersionResource(t *testing.T) {
	t.Parallel()

	img, err := pe.NewFile(bytes.NewReader(testpe.Build(testpe.Options{FileVersion: "2.0.0.0"})))
	if err != nil {
		t.Fatal(err)
	}

	if img.Section(".rsrc") == nil {
		t.Fatal("no .rsrc section")
	}
}
