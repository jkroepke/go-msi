package testpe_test

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	gopeinfo "github.com/jkroepke/go-msi/internal/pe"
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

func TestBuildCarriesTheVersionResource(t *testing.T) {
	t.Parallel()

	data := testpe.Build(testpe.Options{FileVersion: "2.0.0.0", Languages: []int{1031}})

	info, err := gopeinfo.Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if info.FileVersion != "2.0.0.0" || len(info.Languages) != 1 || info.Languages[0] != 1031 {
		t.Fatalf("parsed %+v", *info)
	}
}

// TestSysoLinksIntoAGoExecutable builds a trivial program for Windows with
// the object beside it and reads the version back, pinning what the Go
// linker accepts.
func TestSysoLinksIntoAGoExecutable(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("builds a program")
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go on PATH")
	}

	dir := t.TempDir()
	files := map[string][]byte{
		"go.mod":                  []byte("module t\n\ngo 1.27\n"),
		"main.go":                 []byte("package main\n\nfunc main() {}\n"),
		"rsrc_windows_amd64.syso": testpe.Syso(testpe.Options{FileVersion: "3.1.4.1", Languages: []int{1033}}),
	}

	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	exe := filepath.Join(dir, "t.exe")

	cmd := exec.CommandContext(t.Context(), goBin, "build", "-o", exe, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=-mod=mod") //nolint:wsl_v5 // configures cmd

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	linked, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}

	info, err := gopeinfo.Parse(bytes.NewReader(linked))
	if err != nil {
		t.Fatal(err)
	}

	if info.FileVersion != "3.1.4.1" || len(info.Languages) != 1 || info.Languages[0] != 1033 {
		t.Fatalf("linked executable reports %+v", *info)
	}
}
