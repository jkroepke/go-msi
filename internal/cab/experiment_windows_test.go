//go:build windows && (amd64 || arm64)

package cab_test

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/cab"
)

// TestExperimentSharedOffsets is a temporary experiment: it points one CFFILE
// record at the bytes of an earlier one and logs how much of the cabinet
// cabinet.dll reads to extract everything.
func TestExperimentSharedOffsets(t *testing.T) { //nolint:paralleltest // shared FDI state
	rng := rand.New(rand.NewSource(7))
	random := func(n int) []byte {
		b := make([]byte, n)
		rng.Read(b)

		return b
	}

	when := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	big := random(8 << 20)
	shared := random(1 << 20)
	tail := random(1 << 20)

	layouts := map[string]struct {
		files       []cab.File
		copyOf, dup int
	}{
		"adjacent": {[]cab.File{{Name: "a.bin", Data: big}, {Name: "b.bin", Data: shared}, {Name: "c.bin", Data: random(1 << 20)}, {Name: "d.bin", Data: tail}}, 1, 2},
		"far":      {[]cab.File{{Name: "b.bin", Data: shared}, {Name: "a.bin", Data: big}, {Name: "c.bin", Data: random(1 << 20)}, {Name: "d.bin", Data: tail}}, 0, 2},
		"first":    {[]cab.File{{Name: "b.bin", Data: shared}, {Name: "c.bin", Data: random(1 << 20)}, {Name: "a.bin", Data: big}, {Name: "d.bin", Data: tail}}, 0, 1},
		"last":     {[]cab.File{{Name: "a.bin", Data: big}, {Name: "d.bin", Data: tail}, {Name: "b.bin", Data: shared}, {Name: "c.bin", Data: random(1 << 20)}}, 2, 3},
	}

	for name, layout := range layouts {
		for _, opts := range []cab.Options{{Compression: cab.None}, {Compression: cab.MSZIP}, {Compression: cab.LZX}, {Compression: cab.LZX, Window: 18}} {
			for i := range layout.files {
				layout.files[i].Time = when
			}

			raw, err := cab.Write(layout.files, opts)
			if err != nil {
				t.Fatal(err)
			}

			base, err := fdiExtract(t, raw)
			if err != nil {
				t.Fatalf("%s: baseline: %v", name, err)
			}

			baseRead := lastRead

			if len(base) != len(layout.files) {
				t.Fatalf("baseline extracted %d", len(base))
			}

			// Patch the dup record to point at the copyOf record.
			pos := 36 + 8
			recs := make([]int, len(layout.files))

			for i := range layout.files {
				recs[i] = pos
				pos += 16 + len(layout.files[i].Name) + 1
			}

			patched := bytes.Clone(raw)
			copy(patched[recs[layout.dup]:recs[layout.dup]+8], raw[recs[layout.copyOf]:recs[layout.copyOf]+8])

			got, err := fdiExtract(t, patched)
			if err != nil {
				t.Errorf("%s comp %d window %d: FDI failed: %v", name, opts.Compression, opts.Window, err)

				continue
			}

			ok := len(got) == len(layout.files)
			for i := range got {
				want := layout.files[i].Data
				if i == layout.dup {
					want = layout.files[layout.copyOf].Data
				}

				if got[i].Name != layout.files[i].Name || !bytes.Equal(got[i].Data, want) {
					ok = false
				}
			}

			t.Logf("EXPERIMENT %-8s comp %d window %2d: cabinet %d bytes, baseline read %d, shared read %d (extra %d), shared offset %d, correct %v",
				name, opts.Compression, opts.Window, len(raw), baseRead, lastRead, lastRead-baseRead,
				binary.LittleEndian.Uint32(raw[recs[layout.copyOf]+4:]), ok)
		}
	}

	t.Error("EXPERIMENT REPORT (temporary failure to show the log)")
}
