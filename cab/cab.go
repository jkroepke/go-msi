// Package cab writes and reads Microsoft cabinet files as Windows Installer
// uses them: one folder, stored or MSZIP compression, a checksum on every data
// block, and no spanning across cabinets.
//
// The format is docs/spec/MSISPEC.md section 5. The writer is deterministic:
// identical files and options give identical bytes with one Go version.
package cab

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

// Compression selects the folder's compression type.
type Compression uint16

// Compression types written by this package.
const (
	None  Compression = 0
	MSZIP Compression = 1
)

const (
	headerSize = 36
	folderSize = 8
	blockSize  = 32768
	// maxBlockData is the largest compressed block MSZIP allows.
	maxBlockData = 32780
	maxFiles     = 65535
	maxCabinet   = 0x7FFFFFFF
	maxNameLen   = 255
)

// File attributes.
const (
	AttrReadOnly uint16 = 0x01
	AttrHidden   uint16 = 0x02
	AttrSystem   uint16 = 0x04
	AttrArchive  uint16 = 0x20
	attrUTF8Name uint16 = 0x80
)

// File is one cabinet entry.
type File struct {
	// Name is the entry name, for Windows Installer the File table key.
	Name string
	Data []byte
	// Time is the entry's timestamp, written in UTC fields. The zero time
	// means 1980-01-01T00:00:00Z.
	Time time.Time
	// Attributes defaults to AttrArchive when zero.
	Attributes uint16
}

// Options control the writer.
type Options struct {
	Compression Compression
	// Level is the DEFLATE level for MSZIP, 1 to 9. 0 means 9.
	Level int
	// SetID is written to the header; 0 is fine for embedded cabinets.
	SetID uint16
}

// Checksum computes the CFDATA checksum over b with the given seed.
func Checksum(b []byte, seed uint32) uint32 {
	acc := seed
	n := len(b) / 4

	for i := 0; i < n; i++ {
		acc ^= binary.LittleEndian.Uint32(b[4*i:])
	}

	var tail uint32

	for _, c := range b[4*n:] {
		tail = tail<<8 | uint32(c)
	}

	return acc ^ tail
}

// DOSTime converts t (in UTC) to the cabinet date and time words, clamped to
// the representable range 1980-01-01 … 2107-12-31.
func DOSTime(t time.Time) (date, tm uint16) {
	if t.IsZero() {
		return 0x0021, 0
	}

	t = t.UTC()

	if t.Year() < 1980 {
		return 0x0021, 0
	}

	if t.Year() > 2107 {
		return 0xFF9F, 0xBF7D
	}

	date = uint16((t.Year()-1980)<<9 | int(t.Month())<<5 | t.Day())
	tm = uint16(t.Hour()<<11 | t.Minute()<<5 | t.Second()>>1)

	return date, tm
}

// ParseDOSTime converts cabinet date and time words to a UTC time.
func ParseDOSTime(date, tm uint16) time.Time {
	return time.Date(1980+int(date>>9), time.Month(date>>5&0xF), int(date&0x1F),
		int(tm>>11), int(tm>>5&0x3F), int(tm&0x1F)*2, 0, time.UTC)
}

type block struct {
	data     []byte
	uncomp   int
	checksum uint32
}

// compressBlock produces one MSZIP block for chunk with dict as the DEFLATE
// history, falling back to a stored block when compression does not pay.
func compressBlock(chunk, dict []byte, level int) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("CK")

	w, err := flate.NewWriterDict(&buf, level, dict)
	if err != nil {
		return nil, err
	}

	if _, err := w.Write(chunk); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	if buf.Len() > len(chunk)+2+5 {
		stored := make([]byte, 0, len(chunk)+7)
		stored = append(stored, 'C', 'K', 1, byte(len(chunk)), byte(len(chunk)>>8), byte(^len(chunk)), byte(^len(chunk)>>8))
		stored = append(stored, chunk...)

		return stored, nil
	}

	return buf.Bytes(), nil
}

// Write builds a single-folder cabinet.
func Write(files []File, o Options) ([]byte, error) {
	if len(files) > maxFiles {
		return nil, fmt.Errorf("cab: %d files, at most %d fit in one cabinet", len(files), maxFiles)
	}

	if o.Compression != None && o.Compression != MSZIP {
		return nil, fmt.Errorf("cab: unsupported compression %d", o.Compression)
	}

	level := o.Level
	if level == 0 {
		level = 9
	}

	if level < 1 || level > 9 {
		return nil, fmt.Errorf("cab: DEFLATE level %d, want 1 to 9", level)
	}

	var folder []byte

	for i, f := range files {
		if f.Name == "" || len(f.Name) > maxNameLen {
			return nil, fmt.Errorf("cab: file %d: name %q must be 1 to %d bytes", i, f.Name, maxNameLen)
		}

		if bytes.IndexByte([]byte(f.Name), 0) >= 0 {
			return nil, fmt.Errorf("cab: file %d: name %q contains NUL", i, f.Name)
		}

		if int64(len(folder))+int64(len(f.Data)) > maxCabinet {
			return nil, errors.New("cab: folder exceeds 2 GiB")
		}

		folder = append(folder, f.Data...)
	}

	var blocks []block

	for lo := 0; lo < len(folder); lo += blockSize {
		hi := min(lo+blockSize, len(folder))
		chunk := folder[lo:hi]

		var data []byte

		if o.Compression == MSZIP {
			dict := folder[max(0, lo-blockSize):lo]

			var err error

			data, err = compressBlock(chunk, dict, level)
			if err != nil {
				return nil, err
			}

			if len(data) > maxBlockData {
				return nil, fmt.Errorf("cab: compressed block of %d bytes exceeds %d", len(data), maxBlockData)
			}
		} else {
			data = chunk
		}

		cs := Checksum(data, 0) ^ (uint32(len(data)) | uint32(len(chunk))<<16)
		blocks = append(blocks, block{data: data, uncomp: len(chunk), checksum: cs})
	}

	coffFiles := headerSize + folderSize
	dataStart := coffFiles

	for _, f := range files {
		dataStart += 17 + len(f.Name)
	}

	total := dataStart
	for _, b := range blocks {
		total += 8 + len(b.data)
	}

	if total > maxCabinet {
		return nil, errors.New("cab: cabinet exceeds 2 GiB")
	}

	le := binary.LittleEndian
	out := make([]byte, 0, total)

	h := make([]byte, headerSize)
	copy(h, "MSCF")
	le.PutUint32(h[8:], uint32(total))
	le.PutUint32(h[16:], uint32(coffFiles))
	h[0x18], h[0x19] = 3, 1
	le.PutUint16(h[0x1A:], 1)
	le.PutUint16(h[0x1C:], uint16(len(files)))
	le.PutUint16(h[0x20:], o.SetID)
	out = append(out, h...)

	fo := make([]byte, folderSize)
	le.PutUint32(fo, uint32(dataStart))
	le.PutUint16(fo[4:], uint16(len(blocks)))
	le.PutUint16(fo[6:], uint16(o.Compression))
	out = append(out, fo...)

	offset := 0

	for _, f := range files {
		e := make([]byte, 16)
		le.PutUint32(e, uint32(len(f.Data)))
		le.PutUint32(e[4:], uint32(offset))

		date, tm := DOSTime(f.Time)
		le.PutUint16(e[10:], date)
		le.PutUint16(e[12:], tm)

		attr := f.Attributes
		if attr == 0 {
			attr = AttrArchive
		}

		attr &^= attrUTF8Name

		for i := 0; i < len(f.Name); i++ {
			if f.Name[i] >= 0x80 {
				attr |= attrUTF8Name

				break
			}
		}

		le.PutUint16(e[14:], attr)
		out = append(out, e...)
		out = append(out, f.Name...)
		out = append(out, 0)
		offset += len(f.Data)
	}

	for _, b := range blocks {
		d := make([]byte, 8)
		le.PutUint32(d, b.checksum)
		le.PutUint16(d[4:], uint16(len(b.data)))
		le.PutUint16(d[6:], uint16(b.uncomp))
		out = append(out, d...)
		out = append(out, b.data...)
	}

	return out, nil
}

// Read extracts the files of a cabinet written by this package or any other
// tool that uses stored or MSZIP folders without spanning. It verifies every
// non-zero block checksum.
func Read(c []byte) ([]File, error) {
	le := binary.LittleEndian

	if len(c) < headerSize || string(c[:4]) != "MSCF" {
		return nil, errors.New("cab: not a cabinet")
	}

	if c[0x18] != 3 || c[0x19] != 1 {
		return nil, fmt.Errorf("cab: version %d.%d, want 1.3", c[0x19], c[0x18])
	}

	flags := le.Uint16(c[0x1E:])
	if flags&0x3 != 0 {
		return nil, errors.New("cab: cabinet sets (previous/next cabinet) are not supported")
	}

	if flags&0x4 != 0 {
		return nil, errors.New("cab: reserve areas are not supported")
	}

	numFolders := int(le.Uint16(c[0x1A:]))
	numFiles := int(le.Uint16(c[0x1C:]))
	coffFiles := int(le.Uint32(c[0x10:]))

	if headerSize+folderSize*numFolders > len(c) || coffFiles > len(c) {
		return nil, errors.New("cab: truncated header")
	}

	folders := make([][]byte, 0, numFolders)

	for fi := 0; fi < numFolders; fi++ {
		fo := c[headerSize+folderSize*fi:]
		start := int(le.Uint32(fo))
		numData := int(le.Uint16(fo[4:]))
		typ := Compression(le.Uint16(fo[6:]) & 0xF)

		var out []byte

		p := start

		for k := 0; k < numData; k++ {
			if p+8 > len(c) {
				return nil, fmt.Errorf("cab: folder %d block %d truncated", fi, k)
			}

			csum := le.Uint32(c[p:])
			cb := int(le.Uint16(c[p+4:]))
			un := int(le.Uint16(c[p+6:]))

			if p+8+cb > len(c) {
				return nil, fmt.Errorf("cab: folder %d block %d truncated", fi, k)
			}

			ab := c[p+8 : p+8+cb]

			if csum != 0 {
				if want := Checksum(ab, 0) ^ (uint32(cb) | uint32(un)<<16); want != csum {
					return nil, fmt.Errorf("cab: folder %d block %d checksum %#x, want %#x", fi, k, csum, want)
				}
			}

			switch typ {
			case None:
				if cb != un {
					return nil, fmt.Errorf("cab: folder %d block %d: stored block sizes differ", fi, k)
				}

				out = append(out, ab...)
			case MSZIP:
				if len(ab) < 2 || ab[0] != 'C' || ab[1] != 'K' {
					return nil, fmt.Errorf("cab: folder %d block %d: missing CK signature", fi, k)
				}

				hist := out
				if len(hist) > blockSize {
					hist = hist[len(hist)-blockSize:]
				}

				r := flate.NewReaderDict(bytes.NewReader(ab[2:]), hist)
				buf := make([]byte, un)

				if _, err := io.ReadFull(r, buf); err != nil {
					return nil, fmt.Errorf("cab: folder %d block %d: %w", fi, k, err)
				}

				out = append(out, buf...)
			default:
				return nil, fmt.Errorf("cab: compression type %d is not supported", typ)
			}

			p += 8 + cb
		}

		folders = append(folders, out)
	}

	files := make([]File, 0, numFiles)
	p := coffFiles

	for i := 0; i < numFiles; i++ {
		if p+16 > len(c) {
			return nil, errors.New("cab: truncated file entry")
		}

		size := int(le.Uint32(c[p:]))
		off := int(le.Uint32(c[p+4:]))
		fi := int(le.Uint16(c[p+8:]))

		end := bytes.IndexByte(c[p+16:], 0)
		if end < 0 {
			return nil, errors.New("cab: unterminated file name")
		}

		f := File{
			Name:       string(c[p+16 : p+16+end]),
			Time:       ParseDOSTime(le.Uint16(c[p+10:]), le.Uint16(c[p+12:])),
			Attributes: le.Uint16(c[p+14:]) &^ attrUTF8Name,
		}

		if fi >= len(folders) {
			return nil, fmt.Errorf("cab: file %q in folder %d (spanning is not supported)", f.Name, fi)
		}

		if off+size > len(folders[fi]) {
			return nil, fmt.Errorf("cab: file %q extends past its folder", f.Name)
		}

		f.Data = append([]byte(nil), folders[fi][off:off+size]...)
		files = append(files, f)
		p += 17 + end
	}

	return files, nil
}
