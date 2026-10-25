package cfb_test

import (
	"bytes"
	"testing"

	"github.com/jkroepke/go-msi/cfb"
)

// FuzzRead feeds corrupted compound files to the reader. It must return an
// error or a file, never panic or loop, and the streams it reads must
// survive a write and a second read unchanged.
func FuzzRead(f *testing.F) {
	for _, version := range []int{3, 4} {
		raw, err := cfb.Write([]cfb.Stream{
			{Name: "Small", Data: bytes.Repeat([]byte("x"), 100)},
			{Name: "Large", Data: bytes.Repeat([]byte("y"), 5000)},
			{Name: "Empty"},
		}, cfb.Options{Version: version})
		if err != nil {
			f.Fatal(err)
		}

		f.Add(raw)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		file, err := cfb.Read(raw)
		if err != nil {
			return
		}

		streams := file.Streams()

		for _, stream := range streams {
			if _, err := cfb.EncodeName(stream.Name); err != nil {
				return // the writer is stricter than the reader
			}
		}

		raw2, err := cfb.Write(streams, cfb.Options{Version: file.Version})
		if err != nil {
			if len(streams) == 0 {
				return
			}

			t.Fatalf("write back %d streams: %v", len(streams), err)
		}

		again, err := cfb.Read(raw2)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}

		if len(again.Streams()) != len(streams) {
			t.Fatalf("read back %d streams, wrote %d", len(again.Streams()), len(streams))
		}

		for _, stream := range streams {
			if got, ok := again.Stream(stream.Name); !ok || !bytes.Equal(got, stream.Data) {
				t.Fatalf("stream %q changed in the round trip", stream.Name)
			}
		}
	})
}

// FuzzRoundTrip writes fuzzed stream sets and reads them back unchanged.
func FuzzRoundTrip(f *testing.F) {
	f.Add("A", []byte("data"), "B", make([]byte, 0), uint8(4))
	f.Add("\u0005SummaryInformation", bytes.Repeat([]byte{1}, 4096), "䡀㬿", bytes.Repeat([]byte{2}, 4095), uint8(3))

	f.Fuzz(func(t *testing.T, name1 string, data1 []byte, name2 string, data2 []byte, version uint8) {
		v := 3 + int(version%2)

		streams := []cfb.Stream{{Name: name1, Data: data1}, {Name: name2, Data: data2}}

		raw, err := cfb.Write(streams, cfb.Options{Version: v})
		if err != nil {
			return
		}

		file, err := cfb.Read(raw)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}

		for _, stream := range streams {
			got, ok := file.Stream(stream.Name)
			if !ok {
				t.Fatalf("stream %q is missing", stream.Name)
			}

			if !bytes.Equal(got, stream.Data) {
				t.Fatalf("stream %q: read %d bytes, wrote %d", stream.Name, len(got), len(stream.Data))
			}
		}

		if n := len(file.Streams()); n != len(streams) {
			t.Fatalf("read %d streams, wrote %d", n, len(streams))
		}
	})
}

// TestInvalidUTF8Name is the FuzzRoundTrip finding: a name with invalid
// UTF-8 was stored as U+FFFD and could not be found again.
func TestInvalidUTF8Name(t *testing.T) {
	t.Parallel()

	if _, err := cfb.Write([]cfb.Stream{{Name: "\xff"}}, cfb.Options{}); err == nil {
		t.Fatal("Write accepted a name that is not valid UTF-8")
	}
}
