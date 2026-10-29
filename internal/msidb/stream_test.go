package msidb //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/jkroepke/msi-builder/internal/cfb"
)

// TestSourcesAndDecode checks that MarshalTo copies a source stream through,
// that Unmarshal loads it and that Decode leaves it in the file.
func TestSourcesAndDecode(t *testing.T) {
	t.Parallel()

	cabinet := bytes.Repeat([]byte("MSCF-block-"), 1000)
	opened := 0

	database := sampleDB()
	database.Streams = nil
	database.Sources = []cfb.Source{{
		Name: "cab1.cab", Size: int64(len(cabinet)),
		Open: func() (io.ReadCloser, error) {
			opened++

			return io.NopCloser(bytes.NewReader(cabinet)), nil
		},
	}}

	var buf bytes.Buffer

	layout, err := database.MarshalTo(&buf, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if opened != 1 {
		t.Errorf("the source was opened %d times", opened)
	}

	encoded, _ := EncodeStreamName("cab1.cab", false)
	if off := layout.Offsets[encoded]; !bytes.Equal(buf.Bytes()[off:off+int64(len(cabinet))], cabinet) {
		t.Error("the layout offset does not point at the cabinet")
	}

	back, err := Unmarshal(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if data, ok := back.Stream("cab1.cab"); !ok || !bytes.Equal(data, cabinet) {
		t.Error("Unmarshal did not load the cabinet")
	}

	file, err := cfb.Open(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	lazy, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := lazy.Stream("cab1.cab"); ok || len(lazy.Streams) != 0 {
		t.Errorf("Decode loaded extra streams: %v", lazy.Streams)
	}

	if len(lazy.Sources) != 1 || lazy.Sources[0].Name != "cab1.cab" || lazy.Sources[0].Size != int64(len(cabinet)) || lazy.Sources[0].Open != nil {
		t.Errorf("Decode sources: %+v", lazy.Sources)
	}

	// Tables, their binary streams and the summary are loaded either way.
	if bin := lazy.Table("Binary"); bin == nil || !bytes.Equal(fmt.Append(nil, bin.Rows[0][1]), fmt.Append(nil, []byte("MZ-not-really"))) {
		t.Errorf("Decode Binary table: %v", lazy.Table("Binary"))
	}

	if len(lazy.Summary) == 0 || len(lazy.Problems) != 0 || len(lazy.Tables) != len(back.Tables) {
		t.Errorf("Decode: %d summary properties, problems %v, %d tables", len(lazy.Summary), lazy.Problems, len(lazy.Tables))
	}

	rd, err := file.Reader(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := io.ReadAll(rd); !bytes.Equal(got, cabinet) {
		t.Error("the cabinet read through the file differs")
	}

	database.Streams = []cfb.Stream{{Name: "cab1.cab", Data: cabinet}}
	if _, err := database.MarshalTo(io.Discard, Options{}); err == nil {
		t.Error("a stream written twice was accepted")
	}
}
