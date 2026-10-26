package cab_test

import (
	"bytes"
	"io/fs"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/go-msi/pkg/cab"
)

func seedCabinets(f *testing.F) {
	f.Helper()

	when := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)

	for _, comp := range []cab.Compression{cab.None, cab.MSZIP, cab.LZX} {
		raw, err := cab.Write([]cab.File{
			{Name: "a.txt", Data: []byte("hello"), Time: when},
			{Name: "b.bin", Data: bytes.Repeat([]byte("0123456789"), 7000), Time: when},
			{Name: "empty", Time: when},
			{Name: `dir\x.txt`, Data: []byte("in a folder"), Time: when},
		}, cab.Options{Compression: comp})
		if err != nil {
			f.Fatal(err)
		}

		f.Add(raw)
	}
}

// FuzzRead feeds corrupted cabinets to Open. It must not panic, and every
// file the fs.FS view lists must read back with the bytes Read returns.
func FuzzRead(f *testing.F) {
	seedCabinets(f)

	f.Fuzz(func(t *testing.T, raw []byte) {
		fsys, err := cab.Open(raw)
		if err != nil {
			return
		}

		// Open is Read plus the fs.FS checks, so Read succeeds here.
		files, err := cab.Read(raw)
		if err != nil {
			t.Fatalf("Open succeeds, but Read fails: %v", err)
		}

		for _, file := range files {
			// The fs.FS view uses "/" for the cabinet's "\".
			name := path.Clean(strings.ReplaceAll(file.Name, `\`, "/"))

			got, err := fs.ReadFile(fsys, name)
			if err != nil {
				t.Fatalf("Read lists %q but the fs.FS cannot read it: %v", file.Name, err)
			}

			if !bytes.Equal(got, file.Data) {
				t.Fatalf("%q: the fs.FS returns %d bytes that differ from Read's %d", file.Name, len(got), len(file.Data))
			}
		}
	})
}

// FuzzRoundTrip writes fuzzed files and reads them back unchanged.
func FuzzRoundTrip(f *testing.F) {
	f.Add("a.txt", []byte("hello"), "b", make([]byte, 0), true, uint16(0x20))
	f.Add("x", bytes.Repeat([]byte{0}, 40000), "y", []byte("zz"), false, uint16(1))
	f.Add("ü", []byte("a"), "z", []byte("b"), true, uint16(0))
	f.Add("a", []byte("a"), "b", []byte("b"), false, uint16(0xFFFF))

	when := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)

	f.Fuzz(func(t *testing.T, name1 string, data1 []byte, name2 string, data2 []byte, zip bool, attrs uint16) {
		comp := cab.None
		if zip {
			comp = cab.MSZIP
		}

		want := []cab.File{
			{Name: name1, Data: data1, Time: when, Attributes: attrs},
			{Name: name2, Data: data2, Time: when, Attributes: attrs},
		}

		raw, err := cab.Write(want, cab.Options{Compression: comp})
		if err != nil {
			return
		}

		out, err := cab.Read(raw)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}

		if len(out) != len(want) {
			t.Fatalf("read %d files, wrote %d", len(out), len(want))
		}

		for i := range want {
			if out[i].Name != want[i].Name {
				t.Fatalf("file %d: name %q, wrote %q", i, out[i].Name, want[i].Name)
			}

			if !bytes.Equal(out[i].Data, want[i].Data) {
				t.Fatalf("file %d: data differs", i)
			}

			wantAttrs := attrs &^ 0x80 // the UTF-8 name flag is the writer's own
			if attrs == 0 {
				wantAttrs = cab.AttrArchive
			}

			if out[i].Attributes != wantAttrs {
				t.Fatalf("file %d: attributes %#x, wrote %#x", i, out[i].Attributes, attrs)
			}

			if !out[i].Time.Equal(want[i].Time) {
				t.Fatalf("file %d: time %v, wrote %v", i, out[i].Time, want[i].Time)
			}
		}
	})
}
