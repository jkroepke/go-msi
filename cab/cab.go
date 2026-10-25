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
	"strings"
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
	Time       time.Time
	Name       string
	Data       []byte
	Attributes uint16
}

// Options control the writer.
type Options struct {
	Level       int
	Compression Compression
	SetID       uint16
}

// Checksum computes the CFDATA checksum over b with the given seed.
func Checksum(data []byte, seed uint32) uint32 {
	acc := seed
	words := len(data) / 4

	for i := range words {
		acc ^= binary.LittleEndian.Uint32(data[4*i:])
	}

	var tail uint32

	for _, c := range data[4*words:] {
		tail = tail<<8 | uint32(c)
	}

	return acc ^ tail
}

// DOSTime converts t (in UTC) to the cabinet date and time words, clamped to
// the representable range 1980-01-01 … 2107-12-31.
func DOSTime(when time.Time) (uint16, uint16) {
	var date, timeField uint16

	if when.IsZero() {
		return 0x0021, 0
	}

	when = when.UTC()

	if when.Year() < 1980 {
		return 0x0021, 0
	}

	if when.Year() > 2107 {
		return 0xFF9F, 0xBF7D
	}

	date = uint16((when.Year()-1980)<<9 | int(when.Month())<<5 | when.Day())  //nolint:gosec // the year was clamped to 1980…2107
	timeField = uint16(when.Hour()<<11 | when.Minute()<<5 | when.Second()>>1) //nolint:gosec // fits 16 bits by construction

	return date, timeField
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

	deflater, err := flate.NewWriterDict(&buf, level, dict)
	if err != nil {
		return nil, fmt.Errorf("cab: deflate: %w", err)
	}

	if _, err := deflater.Write(chunk); err != nil {
		return nil, fmt.Errorf("cab: deflate: %w", err)
	}

	if err := deflater.Close(); err != nil {
		return nil, fmt.Errorf("cab: deflate: %w", err)
	}

	if buf.Len() > len(chunk)+2+5 {
		stored := make([]byte, 0, len(chunk)+7)
		n := len(chunk)                                                                  // at most 32768
		stored = append(stored, 'C', 'K', 1, byte(n), byte(n>>8), byte(^n), byte(^n>>8)) //nolint:gosec // bounded by the block size
		stored = append(stored, chunk...)

		return stored, nil
	}

	return buf.Bytes(), nil
}

// Write builds a single-folder cabinet.
func Write(files []File, opts Options) ([]byte, error) { //nolint:cyclop,gocognit // one pass over the format; splitting would scatter it
	if len(files) > maxFiles {
		return nil, fmt.Errorf("cab: %d files, at most %d fit in one cabinet", len(files), maxFiles)
	}

	if opts.Compression != None && opts.Compression != MSZIP {
		return nil, fmt.Errorf("cab: unsupported compression %d", opts.Compression)
	}

	level := opts.Level
	if level == 0 {
		level = 9
	}

	if level < 1 || level > 9 {
		return nil, fmt.Errorf("cab: DEFLATE level %d, want 1 to 9", level)
	}

	var folder []byte

	for i, file := range files {
		if file.Name == "" || len(file.Name) > maxNameLen {
			return nil, fmt.Errorf("cab: file %d: name %q must be 1 to %d bytes", i, file.Name, maxNameLen)
		}

		if strings.IndexByte(file.Name, 0) >= 0 {
			return nil, fmt.Errorf("cab: file %d: name %q contains NUL", i, file.Name)
		}

		if int64(len(folder))+int64(len(file.Data)) > maxCabinet {
			return nil, errors.New("cab: folder exceeds 2 GiB")
		}

		folder = append(folder, file.Data...)
	}

	var blocks []block

	for offset := 0; offset < len(folder); offset += blockSize {
		hi := min(offset+blockSize, len(folder))
		chunk := folder[offset:hi]

		var data []byte

		if opts.Compression == MSZIP {
			dict := folder[max(0, offset-blockSize):offset]

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

		cs := Checksum(data, 0) ^ (uint32(len(data)) | uint32(len(chunk))<<16) //nolint:gosec // bounded by the format limits checked earlier
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

	order := binary.LittleEndian
	out := make([]byte, 0, total)

	hdr := make([]byte, headerSize)
	copy(hdr, "MSCF")
	order.PutUint32(hdr[8:], uint32(total))
	order.PutUint32(hdr[16:], uint32(coffFiles))
	hdr[0x18], hdr[0x19] = 3, 1
	order.PutUint16(hdr[0x1A:], 1)
	order.PutUint16(hdr[0x1C:], uint16(len(files))) //nolint:gosec // the file count was checked against maxFiles
	order.PutUint16(hdr[0x20:], opts.SetID)
	out = append(out, hdr...)

	fo := make([]byte, folderSize)
	order.PutUint32(fo, uint32(dataStart))
	order.PutUint16(fo[4:], uint16(len(blocks))) //nolint:gosec // bounded by the format limits checked earlier
	order.PutUint16(fo[6:], uint16(opts.Compression))
	out = append(out, fo...)

	offset := 0

	for _, file := range files {
		entry := make([]byte, 16)
		order.PutUint32(entry, uint32(len(file.Data))) //nolint:gosec // bounded by the format limits checked earlier
		order.PutUint32(entry[4:], uint32(offset))

		date, tm := DOSTime(file.Time)
		order.PutUint16(entry[10:], date)
		order.PutUint16(entry[12:], tm)

		attr := file.Attributes
		if attr == 0 {
			attr = AttrArchive
		}

		attr &^= attrUTF8Name

		for i := range len(file.Name) {
			if file.Name[i] >= 0x80 {
				attr |= attrUTF8Name

				break
			}
		}

		order.PutUint16(entry[14:], attr)
		out = append(out, entry...)
		out = append(out, file.Name...)
		out = append(out, 0)
		offset += len(file.Data)
	}

	for _, blk := range blocks {
		d := make([]byte, 8)
		order.PutUint32(d, blk.checksum)
		order.PutUint16(d[4:], uint16(len(blk.data))) //nolint:gosec // bounded by the format limits checked earlier
		order.PutUint16(d[6:], uint16(blk.uncomp))    //nolint:gosec // bounded by the format limits checked earlier
		out = append(out, d...)
		out = append(out, blk.data...)
	}

	return out, nil
}

// Read extracts the files of a cabinet written by this package or any other
// tool that uses stored or MSZIP folders without spanning. It verifies every
// non-zero block checksum.
func Read(raw []byte) ([]File, error) { //nolint:cyclop,gocognit // one pass over the format; splitting would scatter it
	order := binary.LittleEndian

	if len(raw) < headerSize || string(raw[:4]) != "MSCF" {
		return nil, errors.New("cab: not a cabinet")
	}

	if raw[0x18] != 3 || raw[0x19] != 1 {
		return nil, fmt.Errorf("cab: version %d.%d, want 1.3", raw[0x19], raw[0x18])
	}

	flags := order.Uint16(raw[0x1E:])
	if flags&0x3 != 0 {
		return nil, errors.New("cab: cabinet sets (previous/next cabinet) are not supported")
	}

	if flags&0x4 != 0 {
		return nil, errors.New("cab: reserve areas are not supported")
	}

	numFolders := int(order.Uint16(raw[0x1A:]))
	numFiles := int(order.Uint16(raw[0x1C:]))
	coffFiles := int(order.Uint32(raw[0x10:]))

	if headerSize+folderSize*numFolders > len(raw) || coffFiles > len(raw) {
		return nil, errors.New("cab: truncated header")
	}

	folders := make([][]byte, 0, numFolders)

	for folder := range numFolders {
		fo := raw[headerSize+folderSize*folder:]
		start := int(order.Uint32(fo))
		numData := int(order.Uint16(fo[4:]))
		typ := Compression(order.Uint16(fo[6:]) & 0xF)

		var out []byte

		pos := start

		for blockIdx := range numData {
			if pos+8 > len(raw) {
				return nil, fmt.Errorf("cab: folder %d block %d truncated", folder, blockIdx)
			}

			csum := order.Uint32(raw[pos:])
			compressed := int(order.Uint16(raw[pos+4:]))
			uncompressed := int(order.Uint16(raw[pos+6:]))

			if pos+8+compressed > len(raw) {
				return nil, fmt.Errorf("cab: folder %d block %d truncated", folder, blockIdx)
			}

			payload := raw[pos+8 : pos+8+compressed]

			if csum != 0 {
				if want := Checksum(payload, 0) ^ (uint32(compressed) | uint32(uncompressed)<<16); want != csum { //nolint:gosec // bounded
					return nil, fmt.Errorf("cab: folder %d block %d checksum %#x, want %#x", folder, blockIdx, csum, want)
				}
			}

			switch typ {
			case None:
				if compressed != uncompressed {
					return nil, fmt.Errorf("cab: folder %d block %d: stored block sizes differ", folder, blockIdx)
				}

				out = append(out, payload...)
			case MSZIP:
				if len(payload) < 2 || payload[0] != 'C' || payload[1] != 'K' {
					return nil, fmt.Errorf("cab: folder %d block %d: missing CK signature", folder, blockIdx)
				}

				hist := out
				if len(hist) > blockSize {
					hist = hist[len(hist)-blockSize:]
				}

				r := flate.NewReaderDict(bytes.NewReader(payload[2:]), hist)
				buf := make([]byte, uncompressed)

				if _, err := io.ReadFull(r, buf); err != nil {
					return nil, fmt.Errorf("cab: folder %d block %d: %w", folder, blockIdx, err)
				}

				out = append(out, buf...)
			default:
				return nil, fmt.Errorf("cab: compression type %d is not supported", typ)
			}

			pos += 8 + compressed
		}

		folders = append(folders, out)
	}

	files := make([]File, 0, numFiles)
	pos := coffFiles

	for range numFiles {
		if pos+16 > len(raw) {
			return nil, errors.New("cab: truncated file entry")
		}

		size := int(order.Uint32(raw[pos:]))
		off := int(order.Uint32(raw[pos+4:]))
		folderIdx := int(order.Uint16(raw[pos+8:]))

		end := bytes.IndexByte(raw[pos+16:], 0)
		if end < 0 {
			return nil, errors.New("cab: unterminated file name")
		}

		file := File{
			Name:       string(raw[pos+16 : pos+16+end]),
			Time:       ParseDOSTime(order.Uint16(raw[pos+10:]), order.Uint16(raw[pos+12:])),
			Attributes: order.Uint16(raw[pos+14:]) &^ attrUTF8Name,
		}

		if folderIdx >= len(folders) {
			return nil, fmt.Errorf("cab: file %q in folder %d (spanning is not supported)", file.Name, folderIdx)
		}

		if off+size > len(folders[folderIdx]) {
			return nil, fmt.Errorf("cab: file %q extends past its folder", file.Name)
		}

		file.Data = append([]byte(nil), folders[folderIdx][off:off+size]...)
		files = append(files, file)
		pos += 17 + end
	}

	return files, nil
}
