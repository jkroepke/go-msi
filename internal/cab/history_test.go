package cab //nolint:testpackage // the test builds records with the unexported checksum

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"io"
	"math/rand/v2"
	"testing"
)

// TestShortBlockHistory reads a cabinet whose MSZIP blocks are shorter than
// 32 KiB, so the DEFLATE history of a block reaches back over several
// earlier blocks. Our writer always fills its blocks, other writers do not.
func TestShortBlockHistory(t *testing.T) {
	t.Parallel()

	page := make([]byte, 3000)
	rnd := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // test data

	for i := range page {
		page[i] = byte(rnd.Uint32()) //nolint:gosec // the low byte is wanted
	}

	data := bytes.Repeat(page, 12)[:34000]
	sizes := []int{1000, 1000, 1000, 1000, 30000}

	var blocks bytes.Buffer

	var hist []byte

	pos := 0

	for _, size := range sizes {
		chunk := data[pos : pos+size]
		pos += size

		var payload bytes.Buffer

		payload.WriteString("CK")

		comp, _ := flate.NewWriterDict(&payload, flate.BestCompression, hist)
		_, _ = comp.Write(chunk)
		_ = comp.Close()

		hist = append(hist, chunk...)
		if len(hist) > blockSize {
			hist = hist[len(hist)-blockSize:]
		}

		rec := make([]byte, 8)
		binary.LittleEndian.PutUint32(rec, blockChecksum(payload.Bytes(), size))
		binary.LittleEndian.PutUint16(rec[4:], uint16(payload.Len())) //nolint:gosec // small
		binary.LittleEndian.PutUint16(rec[6:], uint16(size))          //nolint:gosec // small
		blocks.Write(rec)
		blocks.Write(payload.Bytes())
	}

	name := "page.bin"
	dataStart := headerSize + folderSize + fileEntrySize + len(name) + 1
	raw := make([]byte, dataStart, dataStart+blocks.Len())
	copy(raw, "MSCF")
	binary.LittleEndian.PutUint32(raw[8:], uint32(dataStart+blocks.Len())) //nolint:gosec // small
	binary.LittleEndian.PutUint32(raw[16:], headerSize+folderSize)
	raw[0x18], raw[0x19] = 3, 1
	binary.LittleEndian.PutUint16(raw[0x1A:], 1)
	binary.LittleEndian.PutUint16(raw[0x1C:], 1)
	binary.LittleEndian.PutUint32(raw[headerSize:], uint32(dataStart))    //nolint:gosec // small
	binary.LittleEndian.PutUint16(raw[headerSize+4:], uint16(len(sizes))) //nolint:gosec // small
	binary.LittleEndian.PutUint16(raw[headerSize+6:], uint16(MSZIP))
	binary.LittleEndian.PutUint32(raw[headerSize+folderSize:], uint32(len(data))) //nolint:gosec // small
	copy(raw[headerSize+folderSize+fileEntrySize:], name)
	raw = raw[:dataStart+blocks.Len()]
	copy(raw[dataStart:], blocks.Bytes())

	files, err := Read(raw)
	if err != nil || len(files) != 1 || !bytes.Equal(files[0].Data, data) {
		t.Errorf("Read: %v, %d files", err, len(files))
	}

	err = Walk(bytes.NewReader(raw), int64(len(raw)), func(_ File, r io.Reader) error {
		got, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		if !bytes.Equal(got, data) {
			t.Errorf("Walk gave %d bytes that differ from the data", len(got))
		}

		return nil
	})
	if err != nil {
		t.Errorf("Walk: %v", err)
	}
}
