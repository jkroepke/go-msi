package cab_test

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jkroepke/msi-builder/pkg/cab"
)

func TestFS(t *testing.T) {
	t.Parallel()

	stamp := time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)

	raw, err := cab.Write([]cab.File{
		{Name: "readme.txt", Data: []byte("hello"), Time: stamp, Attributes: cab.AttrArchive | cab.AttrReadOnly},
		{Name: `bin\tool.exe`, Data: make([]byte, 5000), Time: stamp},
		{Name: `bin\sub\empty.dat`, Time: stamp},
	}, cab.Options{Compression: cab.MSZIP})
	if err != nil {
		t.Fatal(err)
	}

	fsys, err := cab.Open(raw)
	if err != nil {
		t.Fatal(err)
	}

	if err := fstest.TestFS(fsys, "readme.txt", "bin/tool.exe", "bin/sub/empty.dat"); err != nil {
		t.Fatal(err)
	}

	data, err := fs.ReadFile(fsys, "readme.txt")
	if err != nil || string(data) != "hello" {
		t.Errorf("readme.txt = %q, %v", data, err)
	}

	info, err := fs.Stat(fsys, "readme.txt")
	if err != nil || info.Mode() != 0o444 || !info.ModTime().Equal(stamp) || info.Size() != 5 {
		t.Errorf("stat: %v %v", info, err)
	}

	entries, err := fs.ReadDir(fsys, "bin")
	if err != nil || len(entries) != 2 || entries[0].Name() != "sub" || !entries[0].IsDir() || entries[1].Name() != "tool.exe" {
		t.Errorf("bin: %v %v", entries, err)
	}

	if _, err := fsys.Open("missing"); err == nil {
		t.Error("missing file opened")
	}

	// Round trip through Files: the names get their backslashes back.
	files, err := cab.Files(fsys)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 3 || files[0].Name != `bin\sub\empty.dat` || files[2].Name != "readme.txt" || !files[2].Time.Equal(stamp) {
		t.Errorf("files: %+v", files)
	}

	again, err := cab.Write(files, cab.Options{Compression: cab.MSZIP})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := cab.Open(again); err != nil {
		t.Fatal(err)
	}
}

func TestFilesFromMapFS(t *testing.T) {
	t.Parallel()

	src := fstest.MapFS{
		"a.txt":     {Data: []byte("a")},
		"dir/b.txt": {Data: []byte("b")},
	}

	files, err := cab.Files(src)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 || files[0].Name != "a.txt" || files[1].Name != `dir\b.txt` {
		t.Errorf("%+v", files)
	}

	if _, err := cab.NewFS([]cab.File{{Name: "x"}, {Name: "x"}}); err == nil {
		t.Error("duplicate accepted")
	}

	if _, err := cab.NewFS([]cab.File{{Name: `..\x`}}); err == nil {
		t.Error("escaping path accepted")
	}
}
