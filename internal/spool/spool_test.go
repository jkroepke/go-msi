package spool_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/jkroepke/msi-builder/internal/spool"
)

func TestBuffer(t *testing.T) {
	t.Parallel()

	var buf spool.Buffer

	if _, err := buf.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}

	if _, err := buf.WriteAt([]byte("W"), 6); err != nil {
		t.Fatal(err)
	}

	if _, err := buf.WriteAt([]byte("!"), 13); err != nil {
		t.Fatal(err)
	}

	want := []byte("hello World\x00\x00!")
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("got %q, want %q", buf.Bytes(), want)
	}

	size, err := spool.Size(&buf)
	if err != nil || size != int64(len(want)) || buf.Len() != size {
		t.Errorf("size %d, %v", size, err)
	}

	got, err := io.ReadAll(io.NewSectionReader(&buf, 6, 5))
	if err != nil || string(got) != "World" {
		t.Errorf("section read %q, %v", got, err)
	}

	if n, err := buf.ReadAt(make([]byte, 4), 12); n != 2 || !errors.Is(err, io.EOF) {
		t.Errorf("read past the end: %d, %v", n, err)
	}

	if _, err := buf.Seek(5, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	if _, err := buf.Write([]byte("-")); err != nil {
		t.Fatal(err)
	}

	if string(buf.Bytes()[:7]) != "hello-W" {
		t.Errorf("seek and write: %q", buf.Bytes())
	}

	if _, err := buf.Seek(-100, io.SeekEnd); err == nil {
		t.Error("negative position accepted")
	}

	if _, err := buf.ReadAt(nil, -1); err == nil {
		t.Error("negative offset accepted")
	}
}
