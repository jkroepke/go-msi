package pe //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jkroepke/go-msi/internal/testpe"
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
