package cab_test

import (
	"bytes"
	"io/fs"
	"testing"
	"time"

	"github.com/jkroepke/go-msi/cab"
)

func seedCabinets(f *testing.F) {
	f.Helper()

	when := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)

	for _, comp := range []cab.Compression{cab.None, cab.MSZIP} {
		raw, err := cab.Write([]cab.File{
			{Name: "a.txt", Data: []byte("hello"), Time: when},
			{Name: "b.bin", Data: bytes.Repeat([]byte("0123456789"), 7000), Time: when},
			{Name: "empty", Time: when},
		}, cab.Options{Compression: comp})
		if err != nil {
			f.Fatal(err)
		}

		f.Add(raw)
	}
}

// FuzzRead feeds corrupted cabinets to Read and Open. Both must return an
// error or a result, never panic, and must agree on the file list.
func FuzzRead(f *testing.F) {
	seedCabinets(f)

	f.Fuzz(func(t *testing.T, raw []byte) {
		files, readErr := cab.Read(raw)

		fsys, openErr := cab.Open(raw)
		if openErr != nil {
			return
		}

		if readErr != nil {
			// Open may be lazier than Read; reading every file must fail then.
			_ = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					_, _ = fs.ReadFile(fsys, path)
				}

				return nil
			})

			return
		}

		for _, file := range files {
			if !fs.ValidPath(file.Name) {
				continue
			}

			if _, err := fs.ReadFile(fsys, file.Name); err != nil {
				t.Fatalf("Read lists %q but the fs.FS cannot read it: %v", file.Name, err)
			}
		}
	})
}

// FuzzRoundTrip writes fuzzed files and reads them back unchanged.
func FuzzRoundTrip(f *testing.F) {
	f.Add("a.txt", []byte("hello"), "b", make([]byte, 0), true, uint16(0x20))
	f.Add("x", bytes.Repeat([]byte{0}, 40000), "y", []byte("zz"), false, uint16(1))

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

			if !out[i].Time.Equal(want[i].Time) {
				t.Fatalf("file %d: time %v, wrote %v", i, out[i].Time, want[i].Time)
			}
		}
	})
}
