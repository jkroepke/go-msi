package cfb_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/memtest"
	"github.com/jkroepke/msi-builder/pkg/cfb"
)

func bytesSource(name string, data []byte) cfb.Source {
	return cfb.Stream{Name: name, Data: data}.Source()
}

// TestWriteToLayout checks that WriteTo reports where every stream landed,
// for streams in the mini stream, in large sectors and empty.
func TestWriteToLayout(t *testing.T) {
	t.Parallel()

	large := make([]byte, 10000)
	for i := range large {
		large[i] = byte(i * 3)
	}

	for _, version := range []int{3, 4} {
		var buf bytes.Buffer

		layout, err := cfb.WriteTo(&buf, []cfb.Source{
			bytesSource("Large", large), bytesSource("Small", []byte("small stream")), bytesSource("Empty", nil),
		}, cfb.Options{Version: version})
		if err != nil {
			t.Fatal(err)
		}

		if layout.Size != int64(buf.Len()) {
			t.Errorf("v%d: layout size %d, wrote %d", version, layout.Size, buf.Len())
		}

		for name, want := range map[string][]byte{"Large": large, "Small": []byte("small stream"), "Empty": nil} {
			off, ok := layout.Offsets[name]
			if !ok {
				t.Errorf("v%d: no offset for %s", version, name)

				continue
			}

			if got := buf.Bytes()[off : off+int64(len(want))]; !bytes.Equal(got, want) {
				t.Errorf("v%d: %s at %d does not hold the stream", version, name, off)
			}
		}

		want, _ := cfb.Write([]cfb.Stream{{Name: "Large", Data: large}, {Name: "Small", Data: []byte("small stream")}, {Name: "Empty"}}, cfb.Options{Version: version})
		if !bytes.Equal(buf.Bytes(), want) {
			t.Errorf("v%d: WriteTo differs from Write", version)
		}
	}
}

// TestOpenReadsLazily checks that Open loads nothing until asked and serves
// the same bytes as Read.
func TestOpenReadsLazily(t *testing.T) {
	t.Parallel()

	large := make([]byte, 3*4096+77)
	for i := range large {
		large[i] = byte(i)
	}

	streams := []cfb.Stream{{Name: "Large", Data: large}, {Name: "Small", Data: []byte("tiny")}, {Name: "Empty"}}

	for _, version := range []int{3, 4} {
		raw, err := cfb.Write(streams, cfb.Options{Version: version})
		if err != nil {
			t.Fatal(err)
		}

		file, err := cfb.Open(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatal(err)
		}

		for _, ent := range file.Entries {
			if ent.Data != nil {
				t.Errorf("v%d: Open loaded %q", version, ent.Name)
			}
		}

		for _, want := range streams {
			reader, err := file.Reader(want.Name)
			if err != nil {
				t.Fatal(err)
			}

			got, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(got, want.Data) || reader.Size() != int64(len(want.Data)) {
				t.Errorf("v%d: Reader(%s): %v, %d bytes", version, want.Name, err, len(got))
			}

			// ReadAt across sector boundaries, and past the end.
			if len(want.Data) > 4100 {
				part := make([]byte, 200)

				if n, err := reader.ReadAt(part, 4000); err != nil || n != 200 || !bytes.Equal(part, want.Data[4000:4200]) {
					t.Errorf("v%d: ReadAt across sectors: %d, %v", version, n, err)
				}

				tail := make([]byte, 100)

				n, err := reader.ReadAt(tail, int64(len(want.Data))-10)
				if n != 10 || !errors.Is(err, io.EOF) {
					t.Errorf("v%d: ReadAt past the end: %d, %v", version, n, err)
				}
			}

			if _, err := reader.Seek(2, io.SeekStart); err != nil {
				t.Fatal(err)
			}

			rest, _ := io.ReadAll(reader)
			if len(want.Data) >= 2 && !bytes.Equal(rest, want.Data[2:]) {
				t.Errorf("v%d: Seek then Read of %s differs", version, want.Name)
			}

			data, ok := file.Stream(want.Name)
			if !ok || !bytes.Equal(data, want.Data) {
				t.Errorf("v%d: Stream(%s) differs", version, want.Name)
			}
		}

		if _, err := file.Reader("Missing"); err == nil {
			t.Error("Reader of a missing stream succeeded")
		}

		if len(file.Streams()) != 3 {
			t.Errorf("v%d: Streams() returned %d", version, len(file.Streams()))
		}
	}
}

// TestSourceErrors checks the size checks on sources.
func TestSourceErrors(t *testing.T) {
	t.Parallel()

	open := func(s string) func() (io.ReadCloser, error) {
		return func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(s)), nil }
	}
	big := strings.Repeat("x", 5000)

	for name, tc := range map[string]struct {
		src  cfb.Source
		want string
	}{
		"small short": {cfb.Source{Name: "S", Size: 10, Open: open("12345")}, "delivered 5 bytes"},
		"small long":  {cfb.Source{Name: "S", Size: 3, Open: open("12345")}, "delivered 4 bytes"},
		"large short": {cfb.Source{Name: "L", Size: 6000, Open: open(big)}, "delivered 5000 bytes"},
		"large long":  {cfb.Source{Name: "L", Size: 4999, Open: open(big)}, "longer than its declared size"},
		"no open":     {cfb.Source{Name: "L", Size: 1}, "no size or source"},
		"open fails": {cfb.Source{Name: "L", Size: 5000, Open: func() (io.ReadCloser, error) {
			return nil, errors.New("boom")
		}}, "boom"},
	} {
		_, err := cfb.WriteTo(io.Discard, []cfb.Source{tc.src}, cfb.Options{})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", name, err, tc.want)
		}
	}
}

// TestBoundedMemory streams a large source through WriteTo and reads it back
// through Open without holding it in memory.
func TestBoundedMemory(t *testing.T) { //nolint:paralleltest // measures the heap of the whole process
	if testing.Short() {
		t.Skip("writes 48 MB")
	}

	const (
		size  = 48 << 20
		limit = 12 << 20
	)

	sink := &discardAt{}

	var (
		layout *cfb.Layout
		err    error
	)

	peak := memtest.PeakHeap(limit, func() {
		layout, err = cfb.WriteTo(sink, []cfb.Source{
			{Name: "Big", Size: size, Open: func() (io.ReadCloser, error) { return memtest.Pattern(size), nil }},
			bytesSource("Small", []byte("s")),
		}, cfb.Options{Version: 3})
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%v", peak)

	if peak.Used() > limit {
		t.Errorf("writer used %d bytes of live heap, want at most %d; %v", peak.Used(), limit, peak)
	}

	if layout.Size != sink.n {
		t.Errorf("layout size %d, wrote %d", layout.Size, sink.n)
	}

	// Reading back needs a file; the FAT of a 96 MB v3 file is small.
	path := t.TempDir() + "/big.cfb"

	out := mustCreate(t, path)

	layout, err = cfb.WriteTo(out, []cfb.Source{
		{Name: "Big", Size: size, Open: func() (io.ReadCloser, error) { return memtest.Pattern(size), nil }},
	}, cfb.Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	want := sha256.New()
	_, _ = io.Copy(want, memtest.Pattern(size))

	peak = memtest.PeakHeap(limit, func() {
		file, oerr := cfb.Open(out, layout.Size)
		if oerr != nil {
			err = oerr

			return
		}

		reader, rerr := file.Reader("Big")
		if rerr != nil {
			err = rerr

			return
		}

		got := sha256.New()
		if _, err = io.Copy(got, reader); err == nil && !bytes.Equal(got.Sum(nil), want.Sum(nil)) {
			err = errors.New("digest differs")
		}
	})

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%v", peak)

	if peak.Used() > limit {
		t.Errorf("reader used %d bytes of live heap, want at most %d; %v", peak.Used(), limit, peak)
	}
}

// discardAt counts bytes.
type discardAt struct{ n int64 }

func (d *discardAt) Write(p []byte) (int, error) {
	d.n += int64(len(p))

	return len(p), nil
}

// TestNestedStream checks that a stream inside a storage is read through its
// own directory entry: Read loads it, Open checks its chain, and it is not a
// root-level stream.
func TestNestedStream(t *testing.T) {
	t.Parallel()

	raw, err := cfb.Write([]cfb.Stream{
		{Name: "A", Data: []byte("root-A")},
		{Name: "B", Data: []byte("root-B")},
		{Name: "C", Data: bytes.Repeat([]byte("root-C!!"), 1024)}, // a large stream, in sectors
	}, cfb.Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(raw)
	if err != nil {
		t.Fatal(err)
	}

	// Turn the tree root's left sibling into a storage holding the right
	// sibling: root.Child = X with X.Left = L and X.Right = R becomes
	// root.Child = X, X.Left = L (storage), L.Child = R.
	root := file.Entries[0]
	top := file.Entries[root.Child]
	left, right := top.Left, top.Right

	if left == 0xFFFFFFFF || right == 0xFFFFFFFF {
		t.Fatalf("unexpected tree: %+v", top)
	}

	dir := int64(binary.LittleEndian.Uint32(raw[0x30:])+1) * 512
	entry := func(sid uint32) []byte { return raw[dir+128*int64(sid):] }

	binary.LittleEndian.PutUint32(entry(root.Child)[0x48:], 0xFFFFFFFF) // X.Right
	entry(left)[0x42] = 1                                               // L is a storage
	binary.LittleEndian.PutUint32(entry(left)[0x4C:], right)            // L.Child = R
	binary.LittleEndian.PutUint64(entry(left)[0x78:], 0)                // storages have no size

	nested := file.Entries[right]
	if nested.Size < 4096 {
		t.Fatalf("nested stream %q has %d bytes; the test wants one in sectors", nested.Name, nested.Size)
	}

	got, err := cfb.Read(raw)
	if err != nil {
		t.Fatalf("Read with a nested stream: %v", err)
	}

	if ent := got.Entries[right]; ent.Parent != int(left) || !bytes.Equal(ent.Data, nested.Data) {
		t.Errorf("nested entry: parent %d, data %q, want parent %d, data %q", ent.Parent, ent.Data, left, nested.Data)
	}

	if _, ok := got.Stream(nested.Name); ok {
		t.Errorf("nested stream %q is listed at the root", nested.Name)
	}

	// The root keeps X; L became a storage and R moved under it.
	if len(got.Streams()) != 1 {
		t.Errorf("root has %d streams, want 1", len(got.Streams()))
	}
}

// TestDeepDirectory reads a file whose directory is a single chain of
// sibling links, the deepest tree a directory of that size can form, and
// checks that the iterative walk reaches every entry.
func TestDeepDirectory(t *testing.T) {
	t.Parallel()

	const count = 5000

	streams := make([]cfb.Stream, 0, count)
	for i := range count {
		streams = append(streams, cfb.Stream{Name: fmt.Sprintf("s%05d", i), Data: []byte{byte(i)}})
	}

	raw, err := cfb.Write(streams, cfb.Options{Version: 3})
	if err != nil {
		t.Fatal(err)
	}

	// Entry i points left to entry i+1 and nowhere else.
	dir := int64(binary.LittleEndian.Uint32(raw[0x30:])+1) * 512

	for sid := 1; sid <= count; sid++ {
		rec := raw[dir+128*int64(sid):]
		left := uint32(sid + 1)

		if sid == count {
			left = 0xFFFFFFFF
		}

		binary.LittleEndian.PutUint32(rec[0x44:], left)
		binary.LittleEndian.PutUint32(rec[0x48:], 0xFFFFFFFF)
	}

	binary.LittleEndian.PutUint32(raw[dir+0x4C:], 1) // root.Child

	file, err := cfb.Read(raw)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(file.Streams()); got != count {
		t.Errorf("read %d streams, want %d", got, count)
	}

	if data, ok := file.Stream("s04999"); !ok || !bytes.Equal(data, []byte{4999 % 256}) {
		t.Errorf("last stream: %v, %v", data, ok)
	}
}
