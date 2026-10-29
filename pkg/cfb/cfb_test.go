package cfb //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

// Test vector A.2 of docs/spec/MSISPEC.md section 2.
func vectorStreams() []Stream {
	small := bytes.Repeat([]byte("0123456789"), 10)

	large := make([]byte, 5000)
	for i := range large {
		large[i] = byte(i)
	}

	return []Stream{{Name: "Large", Data: large}, {Name: "Small", Data: small}}
}

func TestVectorA2(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		sha     string
		version int
		size    int
	}{
		{version: 3, size: 7680, sha: "37d8668bf78bc6da2c9b3c6fa3da9c0ffb6824baee21bf13ff9b02dd74e08aed"},
		{version: 4, size: 28672, sha: "acb3c3db476f1d8aebfac49e89b86895756faa24288228a61f617d3c4d54956a"},
	} {
		out, err := Write(vectorStreams(), Options{Version: tc.version})
		if err != nil {
			t.Fatal(err)
		}

		if len(out) != tc.size {
			t.Errorf("v%d: %d bytes, want %d", tc.version, len(out), tc.size)
		}

		sum := sha256.Sum256(out)
		if got := hex.EncodeToString(sum[:]); got != tc.sha {
			t.Errorf("v%d: SHA-256 %s, want %s", tc.version, got, tc.sha)
		}

		file, err := Read(out)
		if err != nil {
			t.Fatal(err)
		}

		// Small is the black top node, Large its red left child.
		if file.Entries[0].Child != 2 || file.Entries[2].Name != "Small" || file.Entries[2].Colour != 1 {
			t.Errorf("v%d: top node is not black Small", tc.version)
		}

		if file.Entries[2].Left != 1 || file.Entries[1].Name != "Large" || file.Entries[1].Colour != 0 {
			t.Errorf("v%d: left child is not red Large", tc.version)
		}

		if file.CLSID != CLSIDMSI {
			t.Errorf("v%d: root CLSID %x", tc.version, file.CLSID)
		}
	}
}

func TestDefaultVersionIs4(t *testing.T) {
	t.Parallel()

	out, err := Write(nil, Options{})
	if err != nil {
		t.Fatal(err)
	}

	file, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}

	if file.Version != 4 || file.SectorSize != 4096 || len(out) != 3*4096 {
		t.Errorf("version %d, sector size %d, %d bytes", file.Version, file.SectorSize, len(out))
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1)) //nolint:gosec // reproducible test data

	for _, version := range []int{3, 4} {
		for _, count := range []int{0, 1, 2, 7, 40, 300} {
			var streams []Stream

			for i := range count {
				size := 0

				switch i % 5 {
				case 1:
					size = rng.Intn(64)
				case 2:
					size = rng.Intn(4096)
				case 3:
					size = 4096 + rng.Intn(20000)
				case 4:
					size = 100000 + rng.Intn(50000)
				}

				data := make([]byte, size)
				rng.Read(data)
				streams = append(streams, Stream{Name: fmt.Sprintf("s%03d䡀x", i), Data: data})
			}

			out, err := Write(streams, Options{Version: version})
			if err != nil {
				t.Fatalf("v%d/%d: %v", version, count, err)
			}

			if len(out)%(512<<(3*(version-3))) != 0 {
				t.Errorf("v%d/%d: %d bytes is not a whole number of sectors", version, count, len(out))
			}

			file, err := Read(out)
			if err != nil {
				t.Fatalf("v%d/%d: %v", version, count, err)
			}

			if len(file.Streams()) != count {
				t.Fatalf("v%d/%d: read %d streams", version, count, len(file.Streams()))
			}

			for _, s := range streams {
				got, ok := file.Stream(s.Name)
				if !ok || !bytes.Equal(got, s.Data) {
					t.Errorf("v%d/%d: stream %q differs", version, count, s.Name)
				}
			}

			again, err := Write(streams, Options{Version: version})
			if err != nil || !bytes.Equal(again, out) {
				t.Errorf("v%d/%d: not deterministic", version, count)
			}
		}
	}
}

func TestManyFATSectors(t *testing.T) {
	t.Parallel()

	// More than 109 FAT sectors in v3 needs DIFAT sectors: 109 × 128 × 512
	// bytes is about 7 MB.
	data := make([]byte, 8<<20)
	for i := range data {
		data[i] = byte(i * 7)
	}

	out, err := Write([]Stream{{Name: "big", Data: data}, {Name: "tiny", Data: []byte("x")}}, Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	file, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := file.Stream("big")
	if !bytes.Equal(got, data) {
		t.Error("big stream differs")
	}

	got, _ = file.Stream("tiny")
	if string(got) != "x" {
		t.Error("tiny stream differs")
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"a", "B", -1},
		{"b", "A", 1},
		{"A", "a", 0},
		{"ab", "c", 1},
		{"c", "ab", -1},
		{"䡀x", "䡀y", -1},
		{"\u0005SummaryInformation", "䡀Property", 1},
	} {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNameErrors(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "a/b", "a\\b", "a:b", "a!b", "a\x00b", "0123456789012345678901234567890x", "ä"} {
		if _, err := Write([]Stream{{Name: name}}, Options{}); err == nil {
			t.Errorf("name %q accepted", name)
		}
	}

	if _, err := Write([]Stream{{Name: "Same"}, {Name: "same"}}, Options{}); err == nil {
		t.Error("case-insensitive duplicate accepted")
	}

	if _, err := Write([]Stream{{Name: "é"}}, Options{}); err == nil {
		t.Error("cased non-ASCII name accepted")
	}

	if _, err := Write([]Stream{{Name: "䡀㠀\u0005"}}, Options{}); err != nil {
		t.Errorf("MSI name rejected: %v", err)
	}
}

func TestReadRejectsGarbage(t *testing.T) {
	t.Parallel()

	out, _ := Write(vectorStreams(), Options{Version: 3})

	bad := append([]byte(nil), out...)

	bad[0] = 0
	if _, err := Read(bad); err == nil {
		t.Error("bad signature accepted")
	}

	bad = append([]byte(nil), out...)

	bad[0x1E] = 10
	if _, err := Read(bad); err == nil {
		t.Error("bad sector shift accepted")
	}

	// A sibling loop: Large's left points at Small.
	bad = append([]byte(nil), out...)
	f, _ := Read(out)
	dirOff := (int(readU32(out, 0x30)) + 1) * 512
	copy(bad[dirOff+128*int(f.Entries[2].Left)+0x44:], []byte{2, 0, 0, 0})

	if _, err := Read(bad); err == nil {
		t.Error("sibling loop accepted")
	}
}

func readU32(b []byte, off int) uint32 {
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}

func TestEmptyStreamAnyStartSector(t *testing.T) {
	t.Parallel()

	out, _ := Write([]Stream{{Name: "empty"}, {Name: "x", Data: []byte("x")}}, Options{Version: 3})

	f, _ := Read(out)
	dirOff := (int(readU32(out, 0x30)) + 1) * 512

	var sid int

	for _, e := range f.Entries {
		if e.Name == "empty" {
			sid = e.SID

			if e.Start != endOfChain {
				t.Errorf("empty stream starts at %#x, want ENDOFCHAIN", e.Start)
			}
		}
	}

	for _, start := range []uint32{0, freeSect, 12345} {
		bad := append([]byte(nil), out...)
		binary.LittleEndian.PutUint32(bad[dirOff+128*sid+0x74:], start)

		reread, err := Read(bad)
		if err != nil {
			t.Errorf("start %#x: %v", start, err)

			continue
		}

		if d, ok := reread.Stream("empty"); !ok || len(d) != 0 {
			t.Errorf("start %#x: empty stream not empty", start)
		}
	}
}

// Packages from other build tools can end with the last byte of their last
// stream instead of a whole sector (MSISPEC section 2). The data before the cut stays readable;
// a cut into the stream data is an error.
func TestShortLastSector(t *testing.T) {
	t.Parallel()

	for _, version := range []int{3, 4} {
		out, err := Write(vectorStreams(), Options{Version: version})
		if err != nil {
			t.Fatal(err)
		}

		// Large (5000 bytes) is the last stream, so the file ends in its tail.
		sectorSize := 512 << (3 * (version - 3))

		short := out[:len(out)-sectorSize+5000%sectorSize]

		file, err := Read(short)
		if err != nil {
			t.Fatalf("v%d: %v", version, err)
		}

		if data, ok := file.Stream("Large"); !ok || !bytes.Equal(data, vectorStreams()[0].Data) {
			t.Errorf("v%d: Large differs after the cut", version)
		}

		if _, err := Read(short[:len(short)-1]); err == nil {
			t.Errorf("v%d: a cut into the stream data was accepted", version)
		}

		// The lazy path reports the cut too instead of returning zeros.
		cut := short[:len(short)-1]

		lazy, err := Open(bytes.NewReader(cut), int64(len(cut)))
		if err != nil {
			t.Fatalf("v%d: %v", version, err)
		}

		stream, err := lazy.Reader("Large")
		if err != nil {
			t.Fatalf("v%d: %v", version, err)
		}

		if _, err := io.ReadAll(stream); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("v%d: lazy read of a cut stream: %v, want io.ErrUnexpectedEOF", version, err)
		}
	}
}
