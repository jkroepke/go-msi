// Package cab writes and reads Microsoft cabinet files as Windows Installer
// uses them: one folder, stored or MSZIP compression, a checksum on every data
// block, and no spanning across cabinets.
//
// The format is docs/spec/MSISPEC.md section 5. The writer is deterministic:
// identical files and options give identical bytes with one Go version. It
// streams: WriteTo reads every file once through its Open function and keeps
// two blocks and the DEFLATE state in memory, however large the files are.
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

	"github.com/jkroepke/go-msi/internal/spool"
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
	// maxFolder is the largest folder whose block count fits cCFData (u16):
	// 65535 blocks of 32768 bytes, the split threshold of section 5.
	maxFolder  = maxFiles * blockSize
	maxNameLen = 255
	// fileEntrySize is a CFFILE record without its name and NUL.
	fileEntrySize = 16
	// cbCabinetOffset is where the header holds the cabinet size.
	cbCabinetOffset = 8
)

// File attributes.
const (
	AttrReadOnly uint16 = 0x01
	AttrHidden   uint16 = 0x02
	AttrSystem   uint16 = 0x04
	AttrArchive  uint16 = 0x20
	attrUTF8Name uint16 = 0x80
)

// File is one cabinet entry held in memory.
type File struct {
	Time       time.Time
	Name       string
	Data       []byte
	Attributes uint16
}

// Entry is one cabinet entry whose bytes WriteTo reads through Open while it
// writes, so the file never has to be in memory. Open is called once; the
// reader must deliver exactly Size bytes.
type Entry struct {
	Time       time.Time
	Open       func() (io.ReadCloser, error)
	Name       string
	Size       int64
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

// blockChecksum is the stored csum of one CFDATA record.
func blockChecksum(data []byte, uncompressed int) uint32 {
	return Checksum(data, 0) ^ (uint32(len(data)) | uint32(uncompressed)<<16) //nolint:gosec // bounded by the block size
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

// entry converts an in-memory file into a streaming entry.
func (f File) entry() Entry {
	return Entry{
		Name:       f.Name,
		Size:       int64(len(f.Data)),
		Time:       f.Time,
		Attributes: f.Attributes,
		Open:       func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(f.Data)), nil },
	}
}

// Write builds a single-folder cabinet in memory.
func Write(files []File, opts Options) ([]byte, error) {
	entries := make([]Entry, 0, len(files))
	for _, f := range files {
		entries = append(entries, f.entry())
	}

	var buf spool.Buffer

	if _, err := WriteTo(&buf, entries, opts); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// checkOptions validates the options and returns the DEFLATE level.
func checkOptions(opts Options) (int, error) {
	if opts.Compression != None && opts.Compression != MSZIP {
		return 0, fmt.Errorf("cab: unsupported compression %d", opts.Compression)
	}

	level := opts.Level
	if level == 0 {
		level = 9
	}

	if level < 1 || level > 9 {
		return 0, fmt.Errorf("cab: DEFLATE level %d, want 1 to 9", level)
	}

	return level, nil
}

// checkEntries validates the entries and returns the folder size.
func checkEntries(entries []Entry) (int64, error) {
	if len(entries) > maxFiles {
		return 0, fmt.Errorf("cab: %d files, at most %d fit in one cabinet", len(entries), maxFiles)
	}

	var folder int64

	for i, ent := range entries {
		if ent.Name == "" || len(ent.Name) > maxNameLen {
			return 0, fmt.Errorf("cab: file %d: name %q must be 1 to %d bytes", i, ent.Name, maxNameLen)
		}

		if strings.IndexByte(ent.Name, 0) >= 0 {
			return 0, fmt.Errorf("cab: file %d: name %q contains NUL", i, ent.Name)
		}

		if ent.Size < 0 {
			return 0, fmt.Errorf("cab: file %d: negative size", i)
		}

		if ent.Open == nil {
			return 0, fmt.Errorf("cab: file %d: no Open function", i)
		}

		if folder+ent.Size > maxFolder {
			return 0, fmt.Errorf("cab: folder exceeds %d bytes (65535 blocks); split the files across cabinets", maxFolder)
		}

		folder += ent.Size
	}

	return folder, nil
}

// fileEntry encodes one CFFILE record.
func fileEntry(ent Entry, offset int64) []byte {
	order := binary.LittleEndian
	rec := make([]byte, fileEntrySize, fileEntrySize+len(ent.Name)+1)
	order.PutUint32(rec, uint32(ent.Size))   //nolint:gosec // bounded by the folder limit checked earlier
	order.PutUint32(rec[4:], uint32(offset)) //nolint:gosec // bounded by the folder limit checked earlier

	date, tm := DOSTime(ent.Time)
	order.PutUint16(rec[10:], date)
	order.PutUint16(rec[12:], tm)

	attr := ent.Attributes
	if attr == 0 {
		attr = AttrArchive
	}

	attr &^= attrUTF8Name

	for i := range len(ent.Name) {
		if ent.Name[i] >= 0x80 {
			attr |= attrUTF8Name

			break
		}
	}

	order.PutUint16(rec[14:], attr)
	rec = append(rec, ent.Name...)

	return append(rec, 0)
}

// WriteTo writes a single-folder cabinet of entries to w. It opens and reads
// each entry once, in order, holding two blocks and the DEFLATE state in
// memory. The cabinet size in the header is known only at the end, so w must
// seek. It returns the number of bytes written.
func WriteTo(dst io.WriteSeeker, entries []Entry, opts Options) (int64, error) {
	level, err := checkOptions(opts)
	if err != nil {
		return 0, err
	}

	folderBytes, err := checkEntries(entries)
	if err != nil {
		return 0, err
	}

	out := &countingWriter{w: dst}

	if err := writeHeader(out, entries, folderBytes, opts); err != nil {
		return 0, err
	}

	src := &concat{entries: entries}
	defer src.Close()

	if err := writeBlocks(out, src, folderBytes, opts.Compression, level); err != nil {
		return 0, err
	}

	if out.n > maxCabinet {
		return 0, errors.New("cab: cabinet exceeds 2 GiB")
	}

	var cbCabinet [4]byte

	binary.LittleEndian.PutUint32(cbCabinet[:], uint32(out.n)) //nolint:gosec // checked against maxCabinet

	if _, err := dst.Seek(cbCabinetOffset-out.n, io.SeekCurrent); err != nil {
		return 0, fmt.Errorf("cab: seek: %w", err)
	}

	if _, err := dst.Write(cbCabinet[:]); err != nil {
		return 0, fmt.Errorf("cab: write: %w", err)
	}

	if _, err := dst.Seek(out.n-cbCabinetOffset-4, io.SeekCurrent); err != nil {
		return 0, fmt.Errorf("cab: seek: %w", err)
	}

	return out.n, nil
}

// writeHeader writes the CFHEADER, the CFFOLDER and the CFFILE records with
// a zero cabinet size, which WriteTo patches at the end.
func writeHeader(out *countingWriter, entries []Entry, folderBytes int64, opts Options) error {
	order := binary.LittleEndian
	coffFiles := headerSize + folderSize
	dataStart := coffFiles

	for _, ent := range entries {
		dataStart += fileEntrySize + len(ent.Name) + 1
	}

	numBlocks := int((folderBytes + blockSize - 1) / blockSize)

	hdr := make([]byte, headerSize+folderSize)
	copy(hdr, "MSCF")
	order.PutUint32(hdr[16:], uint32(coffFiles))
	hdr[0x18], hdr[0x19] = 3, 1
	order.PutUint16(hdr[0x1A:], 1)
	order.PutUint16(hdr[0x1C:], uint16(len(entries))) //nolint:gosec // the file count was checked against maxFiles
	order.PutUint16(hdr[0x20:], opts.SetID)
	order.PutUint32(hdr[headerSize:], uint32(dataStart))
	order.PutUint16(hdr[headerSize+4:], uint16(numBlocks)) //nolint:gosec // at most 65535 blocks by the folder limit
	order.PutUint16(hdr[headerSize+6:], uint16(opts.Compression))

	if _, err := out.Write(hdr); err != nil {
		return err
	}

	var offset int64

	for _, ent := range entries {
		if _, err := out.Write(fileEntry(ent, offset)); err != nil {
			return err
		}

		offset += ent.Size
	}

	return nil
}

// encodeBlock returns the CFDATA payload for chunk: the bytes themselves or
// an MSZIP block with prev as the DEFLATE history.
func encodeBlock(chunk, prev []byte, comp Compression, level int) ([]byte, error) {
	if comp != MSZIP {
		return chunk, nil
	}

	data, err := compressBlock(chunk, prev, level)
	if err != nil {
		return nil, err
	}

	if len(data) > maxBlockData {
		return nil, fmt.Errorf("cab: compressed block of %d bytes exceeds %d", len(data), maxBlockData)
	}

	return data, nil
}

// writeBlocks compresses the folder bytes from src into CFDATA records.
func writeBlocks(out *countingWriter, src io.Reader, folderBytes int64, comp Compression, level int) error {
	order := binary.LittleEndian
	chunk, prev := make([]byte, blockSize), make([]byte, 0, blockSize)
	rec := make([]byte, 8)

	for remaining := folderBytes; remaining > 0; {
		want := int(min(remaining, blockSize))

		if _, err := io.ReadFull(src, chunk[:want]); err != nil {
			return fmt.Errorf("cab: read: %w", err)
		}

		data, err := encodeBlock(chunk[:want], prev, comp, level)
		if err != nil {
			return err
		}

		order.PutUint32(rec, blockChecksum(data, want))
		order.PutUint16(rec[4:], uint16(len(data))) //nolint:gosec // bounded by maxBlockData
		order.PutUint16(rec[6:], uint16(want))

		if _, err := out.Write(rec); err != nil {
			return err
		}

		if _, err := out.Write(data); err != nil {
			return err
		}

		remaining -= int64(want)
		prev, chunk = chunk[:want], prev[:blockSize]
	}

	// The last entry has delivered its size; make sure it ends there, which
	// also closes it.
	var probe [1]byte

	if extra, err := src.Read(probe[:]); extra > 0 || (err != nil && !errors.Is(err, io.EOF)) {
		if err == nil {
			err = errors.New("more bytes than the entries declared")
		}

		return fmt.Errorf("cab: read: %w", err)
	}

	return nil
}

// countingWriter counts the bytes written and wraps write errors.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	written, err := c.w.Write(p)
	c.n += int64(written)

	if err != nil {
		return written, fmt.Errorf("cab: write: %w", err)
	}

	return written, nil
}

// concat reads the entries back to back, checking that each delivers its
// declared size.
type concat struct {
	cur     io.ReadCloser
	entries []Entry
	left    int64
	index   int
}

func (c *concat) Read(buf []byte) (int, error) {
	for {
		if c.cur == nil {
			if c.index >= len(c.entries) {
				return 0, io.EOF
			}

			if err := c.open(); err != nil {
				return 0, err
			}
		}

		if c.left == 0 {
			if err := c.finish(); err != nil {
				return 0, err
			}

			continue
		}

		read, err := c.readCurrent(buf)
		if err != nil || read > 0 || c.left == 0 {
			return read, err
		}
	}
}

// Close closes the entry that is open, if any; finish has closed the others.
func (c *concat) Close() error {
	if c.cur == nil {
		return nil
	}

	err := c.cur.Close()
	c.cur = nil

	if err != nil {
		return fmt.Errorf("%s: %w", c.entries[c.index].Name, err)
	}

	return nil
}

// readCurrent reads from the current entry, which must have bytes left.
func (c *concat) readCurrent(buf []byte) (int, error) {
	read, err := c.cur.Read(buf[:min(int64(len(buf)), c.left)])
	c.left -= int64(read)

	switch {
	case errors.Is(err, io.EOF) && c.left > 0:
		return read, fmt.Errorf("%s: %d bytes short of its declared size", c.entries[c.index].Name, c.left)
	case errors.Is(err, io.EOF):
		return read, nil
	case err != nil:
		return read, fmt.Errorf("%s: %w", c.entries[c.index].Name, err)
	}

	return read, nil
}

// open starts the current entry.
func (c *concat) open() error {
	ent := c.entries[c.index]

	reader, err := ent.Open()
	if err != nil {
		return fmt.Errorf("%s: %w", ent.Name, err)
	}

	c.cur, c.left = reader, ent.Size

	return nil
}

// finish checks that the current entry has no bytes beyond its size and
// closes it.
func (c *concat) finish() error {
	name := c.entries[c.index].Name

	var probe [1]byte

	extra, err := c.cur.Read(probe[:])
	if extra > 0 {
		_ = c.Close()

		return fmt.Errorf("%s: longer than its declared size", name)
	}

	if err != nil && !errors.Is(err, io.EOF) {
		_ = c.Close()

		return fmt.Errorf("%s: %w", name, err)
	}

	if err := c.Close(); err != nil {
		return err
	}

	c.index++

	return nil
}

// readAt fills p from r at off; a reader that reports io.EOF together with
// the last bytes counts as a full read.
func readAt(r io.ReaderAt, p []byte, off int64) error {
	read, err := r.ReadAt(p, off)
	if err != nil && (!errors.Is(err, io.EOF) || read != len(p)) {
		return err //nolint:wrapcheck // callers add the record context
	}

	return nil
}

// header is a parsed CFHEADER with its folders and file records.
type header struct {
	folders []folderInfo
	files   []fileRecord
	size    int64
}

type folderInfo struct {
	start  int64
	blocks int
	typ    Compression
}

// fileRecord is a parsed CFFILE.
type fileRecord struct {
	file   File
	size   int64
	offset int64
	folder int
}

// parseHeader reads the header, folders and file records of a cabinet.
func parseHeader(src io.ReaderAt, size int64) (*header, error) { //nolint:cyclop // one pass over the format
	order := binary.LittleEndian
	raw := make([]byte, headerSize)

	if size < headerSize {
		return nil, errors.New("cab: not a cabinet")
	}

	if err := readAt(src, raw, 0); err != nil {
		return nil, fmt.Errorf("cab: header: %w", err)
	}

	if string(raw[:4]) != "MSCF" {
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
	coffFiles := int64(order.Uint32(raw[0x10:]))

	if int64(headerSize+folderSize*numFolders) > size || coffFiles > size {
		return nil, errors.New("cab: truncated header")
	}

	hdr := &header{size: size, folders: make([]folderInfo, 0, numFolders)}
	folderRaw := make([]byte, folderSize*numFolders)

	if err := readAt(src, folderRaw, headerSize); err != nil {
		return nil, fmt.Errorf("cab: folders: %w", err)
	}

	dataStart := size

	for i := range numFolders {
		fo := folderRaw[folderSize*i:]
		info := folderInfo{
			start:  int64(order.Uint32(fo)),
			blocks: int(order.Uint16(fo[4:])),
			typ:    Compression(order.Uint16(fo[6:]) & 0xF),
		}
		hdr.folders = append(hdr.folders, info)
		dataStart = min(dataStart, info.start)
	}

	if dataStart < coffFiles {
		return nil, errors.New("cab: folder data starts before the file records")
	}

	// The records cannot be longer than numFiles maximal entries, whatever
	// the folder offsets claim.
	filesLen := min(dataStart-coffFiles, int64(numFiles)*(fileEntrySize+maxNameLen+1))
	filesRaw := make([]byte, filesLen)

	if err := readAt(src, filesRaw, coffFiles); err != nil {
		return nil, fmt.Errorf("cab: file records: %w", err)
	}

	pos := 0

	for range numFiles {
		if pos+fileEntrySize > len(filesRaw) {
			return nil, errors.New("cab: truncated file entry")
		}

		end := bytes.IndexByte(filesRaw[pos+fileEntrySize:], 0)
		if end < 0 {
			return nil, errors.New("cab: unterminated file name")
		}

		rec := fileRecord{
			file: File{
				Name:       string(filesRaw[pos+fileEntrySize : pos+fileEntrySize+end]),
				Time:       ParseDOSTime(order.Uint16(filesRaw[pos+10:]), order.Uint16(filesRaw[pos+12:])),
				Attributes: order.Uint16(filesRaw[pos+14:]) &^ attrUTF8Name,
			},
			size:   int64(order.Uint32(filesRaw[pos:])),
			offset: int64(order.Uint32(filesRaw[pos+4:])),
			folder: int(order.Uint16(filesRaw[pos+8:])),
		}

		if rec.folder >= numFolders {
			return nil, fmt.Errorf("cab: file %q in folder %d (spanning is not supported)", rec.file.Name, rec.folder)
		}

		hdr.files = append(hdr.files, rec)
		pos += fileEntrySize + end + 1
	}

	return hdr, nil
}

// blockReader decodes the CFDATA records of one folder in order. Each call to
// next returns the uncompressed bytes of one block, valid until the next call.
type blockReader struct {
	r       io.ReaderAt
	payload []byte
	out     []byte
	hist    []byte
	pos     int64
	size    int64
	folder  int
	index   int
	blocks  int
	typ     Compression
}

func newBlockReader(r io.ReaderAt, size int64, folder int, info folderInfo) *blockReader {
	return &blockReader{
		r: r, size: size, folder: folder, pos: info.start, blocks: info.blocks, typ: info.typ,
		payload: make([]byte, maxBlockData), out: make([]byte, blockSize), hist: make([]byte, 0, blockSize),
	}
}

// errorf names the current block in an error.
func (b *blockReader) errorf(format string, args ...any) error {
	return fmt.Errorf("cab: folder %d block %d: %s", b.folder, b.index, fmt.Sprintf(format, args...))
}

// record reads and checks the next CFDATA record, returning its payload and
// uncompressed size.
func (b *blockReader) record() ([]byte, int, error) {
	order := binary.LittleEndian

	var rec [8]byte

	if b.pos+8 > b.size {
		return nil, 0, b.errorf("truncated")
	}

	if err := readAt(b.r, rec[:], b.pos); err != nil {
		return nil, 0, b.errorf("%v", err)
	}

	csum := order.Uint32(rec[:])
	compressed := int(order.Uint16(rec[4:]))
	uncompressed := int(order.Uint16(rec[6:]))

	if b.pos+8+int64(compressed) > b.size {
		return nil, 0, b.errorf("truncated")
	}

	if compressed > len(b.payload) {
		return nil, 0, b.errorf("%d payload bytes exceed %d", compressed, maxBlockData)
	}

	if uncompressed > blockSize {
		return nil, 0, b.errorf("%d uncompressed bytes exceed %d", uncompressed, blockSize)
	}

	payload := b.payload[:compressed]

	if err := readAt(b.r, payload, b.pos+8); err != nil {
		return nil, 0, b.errorf("%v", err)
	}

	if csum != 0 {
		if want := blockChecksum(payload, uncompressed); want != csum {
			return nil, 0, b.errorf("checksum %#x, want %#x", csum, want)
		}
	}

	return payload, uncompressed, nil
}

// next decodes the next block, or returns io.EOF after the last one.
func (b *blockReader) next() ([]byte, error) {
	if b.index >= b.blocks {
		return nil, io.EOF
	}

	payload, uncompressed, err := b.record()
	if err != nil {
		return nil, err
	}

	out := b.out[:uncompressed]

	if err := b.decode(payload, out); err != nil {
		return nil, err
	}

	// The DEFLATE history is the last 32 KiB of the folder, which spans more
	// than one block when blocks are short.
	if keep := blockSize - len(out); keep <= 0 {
		b.hist = append(b.hist[:0], out...)
	} else {
		if len(b.hist) > keep {
			b.hist = b.hist[:copy(b.hist, b.hist[len(b.hist)-keep:])]
		}

		b.hist = append(b.hist, out...)
	}

	b.pos += 8 + int64(len(payload))
	b.index++

	return out, nil
}

// decode fills out with the uncompressed bytes of payload.
func (b *blockReader) decode(payload, out []byte) error {
	switch b.typ {
	case None:
		if len(payload) != len(out) {
			return b.errorf("stored block sizes differ")
		}

		copy(out, payload)
	case MSZIP:
		if len(payload) < 2 || payload[0] != 'C' || payload[1] != 'K' {
			return b.errorf("missing CK signature")
		}

		inflater := flate.NewReaderDict(bytes.NewReader(payload[2:]), b.hist)

		if _, err := io.ReadFull(inflater, out); err != nil {
			return b.errorf("%v", err)
		}
	default:
		return fmt.Errorf("cab: compression type %d is not supported", b.typ)
	}

	return nil
}

// folderReader serves the uncompressed bytes of a folder as a stream.
type folderReader struct {
	blocks *blockReader
	buf    []byte
}

func (f *folderReader) Read(buf []byte) (int, error) {
	for len(f.buf) == 0 {
		out, err := f.blocks.next()
		if err != nil {
			return 0, err
		}

		f.buf = out
	}

	copied := copy(buf, f.buf)
	f.buf = f.buf[copied:]

	return copied, nil
}

// Read extracts the files of a cabinet written by this package or any other
// tool that uses stored or MSZIP folders without spanning. It verifies every
// non-zero block checksum. Files may share or overlap folder bytes.
func Read(raw []byte) ([]File, error) {
	hdr, err := parseHeader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}

	folders := make([][]byte, 0, len(hdr.folders))

	for i, info := range hdr.folders {
		blocks := newBlockReader(bytes.NewReader(raw), int64(len(raw)), i, info)

		var out []byte

		for {
			block, err := blocks.next()
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				return nil, err
			}

			out = append(out, block...)
		}

		folders = append(folders, out)
	}

	files := make([]File, 0, len(hdr.files))

	for _, rec := range hdr.files {
		folder := folders[rec.folder]
		if rec.offset+rec.size > int64(len(folder)) {
			return nil, fmt.Errorf("cab: file %q extends past its folder", rec.file.Name)
		}

		file := rec.file
		file.Data = append([]byte(nil), folder[rec.offset:rec.offset+rec.size]...)
		files = append(files, file)
	}

	return files, nil
}

// Walk reads the cabinet in r in one pass and calls fn for every file in
// folder order with a reader over its bytes, holding one block in memory at
// a time. The File passed to fn has no Data. Walk serves cabinets whose files
// lie back to back in their folders, which is how this package writes them;
// files that overlap or repeat need Read.
func Walk(src io.ReaderAt, size int64, visit func(File, io.Reader) error) error {
	hdr, err := parseHeader(src, size)
	if err != nil {
		return err
	}

	for folderIdx, info := range hdr.folders {
		stream := &folderReader{blocks: newBlockReader(src, size, folderIdx, info)}

		var pos int64

		for _, rec := range hdr.files {
			if rec.folder != folderIdx {
				continue
			}

			if rec.offset < pos {
				return fmt.Errorf("cab: file %q overlaps the file before it; use Read", rec.file.Name)
			}

			if _, err := io.CopyN(io.Discard, stream, rec.offset-pos); err != nil {
				return fmt.Errorf("cab: file %q: %w", rec.file.Name, err)
			}

			limited := io.LimitReader(stream, rec.size)

			if err := visit(rec.file, limited); err != nil {
				return err
			}

			if _, err := io.Copy(io.Discard, limited); err != nil {
				return fmt.Errorf("cab: file %q: %w", rec.file.Name, err)
			}

			if limited.(*io.LimitedReader).N > 0 { //nolint:forcetypeassert // io.LimitReader returns *io.LimitedReader
				return fmt.Errorf("cab: file %q extends past its folder", rec.file.Name)
			}

			pos = rec.offset + rec.size
		}
	}

	return nil
}
