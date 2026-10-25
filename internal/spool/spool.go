// Package spool holds bytes that a writer produces sequentially and a reader
// later needs at random offsets: an in-memory Buffer for small outputs and
// tests, with the same shape as an *os.File for large ones.
package spool

import (
	"errors"
	"fmt"
	"io"
)

// Target is what a package or cabinet is written to: sequential writes while
// it is produced, then random access to patch and read it back. *os.File and
// *Buffer satisfy it.
type Target interface {
	io.Writer
	io.WriterAt
	io.ReaderAt
	io.Seeker
}

// Buffer is an in-memory Target.
type Buffer struct {
	data []byte
	pos  int64
}

var errOffset = errors.New("spool: negative offset")

// Write appends at the current position, growing the buffer as needed.
func (b *Buffer) Write(data []byte) (int, error) {
	n, err := b.WriteAt(data, b.pos)
	b.pos += int64(n)

	return n, err
}

// WriteAt writes data at off, growing the buffer as needed.
func (b *Buffer) WriteAt(data []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errOffset
	}

	if end := off + int64(len(data)); end > int64(len(b.data)) {
		if end > int64(cap(b.data)) {
			grown := make([]byte, end, max(end, 2*int64(cap(b.data))))
			copy(grown, b.data)
			b.data = grown
		} else {
			b.data = b.data[:end]
		}
	}

	return copy(b.data[off:], data), nil
}

// ReadAt reads from off, returning io.EOF at the end of the data.
func (b *Buffer) ReadAt(data []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errOffset
	}

	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}

	n := copy(data, b.data[off:])
	if n < len(data) {
		return n, io.EOF
	}

	return n, nil
}

// Seek moves the write position.
func (b *Buffer) Seek(offset int64, whence int) (int64, error) {
	var pos int64

	switch whence {
	case io.SeekStart:
		pos = offset
	case io.SeekCurrent:
		pos = b.pos + offset
	case io.SeekEnd:
		pos = int64(len(b.data)) + offset
	default:
		return 0, fmt.Errorf("spool: whence %d", whence)
	}

	if pos < 0 {
		return 0, errOffset
	}

	b.pos = pos

	return pos, nil
}

// Bytes returns the written bytes without copying.
func (b *Buffer) Bytes() []byte { return b.data }

// Len returns the number of bytes written.
func (b *Buffer) Len() int64 { return int64(len(b.data)) }

// Size returns the length of t: the current end of a Buffer, the file size
// of an *os.File.
func Size(target io.Seeker) (int64, error) {
	cur, err := target.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("spool: %w", err)
	}

	end, err := target.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("spool: %w", err)
	}

	if _, err := target.Seek(cur, io.SeekStart); err != nil {
		return 0, fmt.Errorf("spool: %w", err)
	}

	return end, nil
}
