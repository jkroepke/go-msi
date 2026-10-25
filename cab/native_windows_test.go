//go:build windows

package cab_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/jkroepke/go-msi/cab"
)

// These tests check the cabinets this package writes against Windows itself:
// cabinet.dll's FDI (the extractor Windows Installer uses, loaded through
// syscall without cgo) and expand.exe extract them, and makecab.exe builds a
// cabinet that this package reads. They run only without -short.

var (
	cabinetDLL  = syscall.NewLazyDLL("cabinet.dll")  //nolint:gochecknoglobals // one DLL table for the file
	pFDICreate  = cabinetDLL.NewProc("FDICreate")    //nolint:gochecknoglobals // one DLL table for the file
	pFDICopy    = cabinetDLL.NewProc("FDICopy")      //nolint:gochecknoglobals // one DLL table for the file
	pFDIDestroy = cabinetDLL.NewProc("FDIDestroy")   //nolint:gochecknoglobals // one DLL table for the file
	pFDIIsCab   = cabinetDLL.NewProc("FDIIsCabinet") //nolint:gochecknoglobals // one DLL table for the file
	kernel32    = syscall.NewLazyDLL("kernel32.dll") //nolint:gochecknoglobals // one DLL table for the file
	pLocalAlloc = kernel32.NewProc("LocalAlloc")     //nolint:gochecknoglobals // one DLL table for the file
	pLocalFree  = kernel32.NewProc("LocalFree")      //nolint:gochecknoglobals // one DLL table for the file
)

// FDI notification types.
const (
	fdintCabinetInfo   = 0
	fdintPartialFile   = 1
	fdintCopyFile      = 2
	fdintCloseFileInfo = 3
	fdintNextCabinet   = 4
)

// fdiErrors names the FDIERROR codes.
var fdiErrors = []string{ //nolint:gochecknoglobals // a constant table
	"FDIERROR_NONE", "FDIERROR_CABINET_NOT_FOUND", "FDIERROR_NOT_A_CABINET", "FDIERROR_UNKNOWN_CABINET_VERSION",
	"FDIERROR_CORRUPT_CABINET", "FDIERROR_ALLOC_FAIL", "FDIERROR_BAD_COMPR_TYPE", "FDIERROR_MDI_FAIL",
	"FDIERROR_TARGET_FILE", "FDIERROR_RESERVE_MISMATCH", "FDIERROR_WRONG_CABINET", "FDIERROR_USER_ABORT",
}

// extracted is one file as FDI reported and wrote it.
type extracted struct {
	Name    string
	Data    []byte
	Date    uint16
	Time    uint16
	Attribs uint16
	Size    int32
}

// memFile is an in-memory file behind an FDI handle.
type memFile struct {
	data []byte
	pos  int
	out  *extracted
}

// fdiState is the single in-flight extraction. FDI calls back on the calling
// thread, so one package-level state is enough for sequential tests.
type fdiState struct {
	cabinet []byte
	handles map[uintptr]*memFile
	next    uintptr
	files   []extracted
	erf     *[3]int32 // erfOper, erfType, fError
}

var state *fdiState //nolint:gochecknoglobals // FDI callbacks have no user pointer

// fdiNotification mirrors FDINOTIFICATION on 64-bit Windows.
type fdiNotification struct {
	Cb       int32
	_        int32
	Psz1     *byte
	Psz2     *byte
	Psz3     *byte
	Pv       uintptr
	Hf       uintptr
	Date     uint16
	Time     uint16
	Attribs  uint16
	SetID    uint16
	ICabinet uint16
	IFolder  uint16
	Fdie     int32
}

// fdiCabinetInfo mirrors FDICABINETINFO.
type fdiCabinetInfo struct {
	Size     int32
	Folders  uint16
	Files    uint16
	SetID    uint16
	ICabinet uint16
	Reserve  int32
	Prev     int32
	Next     int32
}

func cstring(ptr *byte) string {
	if ptr == nil {
		return ""
	}

	var n int
	for *(*byte)(unsafe.Add(unsafe.Pointer(ptr), n)) != 0 {
		n++
	}

	return string(unsafe.Slice(ptr, n))
}

// The callbacks. Win64 has one calling convention, so cdecl FDI callbacks are
// plain syscall.NewCallback functions.
// syscall.NewCallback must run once per function, so the callbacks are globals.
var (
	cbAlloc = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(n uintptr) uintptr {
			p, _, _ := pLocalAlloc.Call(0x40 /* LMEM_ZEROINIT */, n)

			return p
		})
	cbFree = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(p uintptr) uintptr {
			_, _, _ = pLocalFree.Call(p)

			return 0
		})
	cbOpen = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(_, _, _ uintptr) uintptr {
			// Only the cabinet is ever opened by name; extracted files are created
			// in the COPY_FILE notification.
			return state.open(&memFile{data: state.cabinet})
		})
	cbRead = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(handle uintptr, buf *byte, n uintptr) uintptr {
			file := state.handles[handle]
			if file == nil {
				return ^uintptr(0)
			}

			got := copy(unsafe.Slice(buf, n), file.data[file.pos:])
			file.pos += got

			return uintptr(got)
		})
	cbWrite = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(handle uintptr, buf *byte, n uintptr) uintptr {
			file := state.handles[handle]
			if file == nil {
				return ^uintptr(0)
			}

			file.data = append(file.data, unsafe.Slice(buf, n)...)

			return n
		})
	cbClose = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(handle uintptr) uintptr {
			file := state.handles[handle]
			if file == nil {
				return ^uintptr(0)
			}

			if file.out != nil {
				file.out.Data = file.data
				state.files = append(state.files, *file.out)
			}

			delete(state.handles, handle)

			return 0
		})
	cbSeek = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(handle, dist, whence uintptr) uintptr {
			file := state.handles[handle]
			if file == nil {
				return ^uintptr(0)
			}

			delta := int(int32(uint32(dist))) // long is 32 bits on Windows

			switch whence {
			case 0:
				file.pos = delta
			case 1:
				file.pos += delta
			case 2:
				file.pos = len(file.data) + delta
			}

			if file.pos < 0 || file.pos > len(file.data) {
				return ^uintptr(0)
			}

			return uintptr(file.pos)
		})
	cbNotify = syscall.NewCallback( //nolint:gochecknoglobals // see above
		func(typ uintptr, note *fdiNotification) uintptr {
			switch typ {
			case fdintCabinetInfo, fdintPartialFile:
				return 0
			case fdintCopyFile:
				out := &extracted{Name: cstring(note.Psz1), Size: note.Cb, Date: note.Date, Time: note.Time, Attribs: note.Attribs}

				return state.open(&memFile{out: out})
			case fdintCloseFileInfo:
				if file := state.handles[note.Hf]; file != nil && file.out != nil {
					file.out.Data = file.data
					state.files = append(state.files, *file.out)
					file.out = nil
				}

				delete(state.handles, note.Hf)

				return 1
			case fdintNextCabinet:
				return ^uintptr(0)
			default:
				return 0
			}
		})
)

func (s *fdiState) open(f *memFile) uintptr {
	h := s.next
	s.next++
	s.handles[h] = f

	return h
}

func (s *fdiState) errorText() string {
	oper := int(s.erf[0])
	name := fmt.Sprintf("FDIERROR %d", oper)

	if oper >= 0 && oper < len(fdiErrors) {
		name = fdiErrors[oper]
	}

	return fmt.Sprintf("%s (erfType %d)", name, s.erf[1])
}

// fdiExtract extracts every file of raw with cabinet.dll and returns them in
// cabinet order.
func fdiExtract(t *testing.T, raw []byte) ([]extracted, error) {
	t.Helper()

	state = &fdiState{cabinet: raw, handles: make(map[uintptr]*memFile), next: 1, erf: new([3]int32)}
	defer func() { state = nil }()

	hfdi, _, _ := pFDICreate.Call(cbAlloc, cbFree, cbOpen, cbRead, cbWrite, cbClose, cbSeek,
		^uintptr(0) /* cpuUNKNOWN */, uintptr(unsafe.Pointer(state.erf)))
	if hfdi == 0 {
		return nil, fmt.Errorf("fdi: FDICreate failed: %s", state.errorText())
	}

	defer func() { _, _, _ = pFDIDestroy.Call(hfdi) }()

	// FDIIsCabinet checks the header the way FDICopy will.
	var info fdiCabinetInfo

	hf := state.open(&memFile{data: raw})

	isCab, _, _ := pFDIIsCab.Call(hfdi, hf, uintptr(unsafe.Pointer(&info)))
	delete(state.handles, hf)

	if isCab == 0 {
		return nil, fmt.Errorf("fdi: FDIIsCabinet rejected the cabinet: %s", state.errorText())
	}

	if int(info.Size) != len(raw) {
		return nil, fmt.Errorf("fdi: FDIIsCabinet reports cbCabinet %d, the cabinet has %d bytes", info.Size, len(raw))
	}

	name, _ := syscall.BytePtrFromString("test.cab")
	path, _ := syscall.BytePtrFromString("")

	copied, _, _ := pFDICopy.Call(hfdi, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(path)), 0, cbNotify, 0, 0)
	if copied == 0 {
		return nil, fmt.Errorf("fdi: FDICopy failed: %s", state.errorText())
	}

	if len(state.handles) != 0 {
		return nil, fmt.Errorf("%d handles left open", len(state.handles))
	}

	return state.files, nil
}

// expandExtract extracts raw with expand.exe into a temporary directory.
func expandExtract(t *testing.T, raw []byte) (map[string][]byte, error) {
	t.Helper()

	dir := t.TempDir()
	src := filepath.Join(dir, "in.cab")
	out := filepath.Join(dir, "out")

	if err := os.WriteFile(src, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(out, 0o700); err != nil {
		t.Fatal(err)
	}

	cmd := exec.CommandContext(t.Context(), filepath.Join(os.Getenv("SystemRoot"), "System32", "expand.exe"), "-F:*", src, out) //nolint:gosec // fixed Windows tool
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("expand.exe: %w\n%s", err, output)
	}

	files := make(map[string][]byte)

	err := filepath.WalkDir(out, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, _ := filepath.Rel(out, path)

		data, err := os.ReadFile(path) //nolint:gosec // a file expand.exe just wrote
		files[filepath.ToSlash(rel)] = data

		return err
	})

	return files, err
}

type nativeCase struct {
	name  string
	files []cab.File
	opts  cab.Options
}

func nativeCases() []nativeCase {
	when := time.Date(2024, 6, 15, 13, 45, 30, 0, time.UTC)
	rng := rand.New(rand.NewSource(11)) //nolint:gosec // reproducible test data
	random := make([]byte, 100000)
	rng.Read(random)

	// Compressible data with matches that reach back across block boundaries.
	text := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. 0123456789\r\n"), 2000)

	many := make([]cab.File, 0, 2000)
	for i := range 2000 {
		many = append(many, cab.File{Name: fmt.Sprintf("fil_%04d", i), Data: fmt.Appendf(nil, "file %d\r\n", i), Time: when})
	}

	cases := []nativeCase{
		{name: "stored-vector", opts: cab.Options{Compression: cab.None, SetID: 0x1234}, files: []cab.File{
			{Name: "fil_a", Data: []byte("Hello, cabinet!\r\n"), Time: when, Attributes: 0x20},
			{Name: "fil_b", Data: []byte("MSI\r\n"), Time: when, Attributes: 0x21},
		}},
		{name: "mszip-small", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "fil_a", Data: []byte("Hello, cabinet!\r\n"), Time: when},
			{Name: "fil_b", Data: []byte("MSI\r\n"), Time: when, Attributes: cab.AttrReadOnly | cab.AttrHidden | cab.AttrSystem},
		}},
		{name: "empty-file-only", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "empty", Time: when},
		}},
		{name: "empty-files-stored", opts: cab.Options{Compression: cab.None}, files: []cab.File{
			{Name: "empty1", Time: when}, {Name: "empty2", Time: when},
		}},
		{name: "empty-between-files", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "first", Data: text[:1000], Time: when},
			{Name: "empty", Time: when},
			{Name: "last", Data: text[1000:3000], Time: when},
		}},
		{name: "empty-at-end", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "first", Data: text[:40000], Time: when},
			{Name: "empty", Time: when},
		}},
		{name: "exactly-one-block", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "block", Data: text[:32768], Time: when},
		}},
		{name: "one-block-plus-one", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "block", Data: text[:32769], Time: when},
		}},
		{name: "file-starts-at-block-boundary", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "fil_a", Data: text[:32768], Time: when},
			{Name: "fil_b", Data: text[32768:65536], Time: when},
			{Name: "fil_c", Data: text[65536:65537], Time: when},
		}},
		{name: "file-crosses-blocks", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "fil_a", Data: text[:20000], Time: when},
			{Name: "fil_b", Data: text[20000:70000], Time: when},
			{Name: "fil_c", Data: text[70000:], Time: when},
		}},
		{name: "stored-multi-block", opts: cab.Options{Compression: cab.None}, files: []cab.File{
			{Name: "fil_a", Data: text[:70000], Time: when},
			{Name: "fil_b", Data: random[:3], Time: when},
		}},
		{name: "random-stored-fallback", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "random", Data: random, Time: when},
		}},
		{name: "mixed-random-and-text", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "r1", Data: random[:33000], Time: when},
			{Name: "t1", Data: text[:50000], Time: when},
			{Name: "r2", Data: random[33000:], Time: when},
		}},
		{name: "many-files", opts: cab.Options{Compression: cab.MSZIP}, files: many},
		{name: "dates", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "zero", Data: []byte("x")},
			{Name: "early", Data: []byte("x"), Time: time.Date(1979, 1, 1, 0, 0, 0, 0, time.UTC)},
			{Name: "late", Data: []byte("x"), Time: time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC)},
			{Name: "odd", Data: []byte("x"), Time: time.Date(2026, 10, 25, 23, 59, 59, 0, time.UTC)},
		}},
		{name: "utf8-name", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: "grüße.txt", Data: []byte("x"), Time: when},
		}},
		{name: "long-name", opts: cab.Options{Compression: cab.MSZIP}, files: []cab.File{
			{Name: strings.Repeat("n", 255), Data: []byte("x"), Time: when},
		}},
	}

	// Checksum tails: compressed sizes of 1, 2 and 3 bytes modulo 4.
	for tail := range 4 {
		cases = append(cases, nativeCase{
			name: fmt.Sprintf("checksum-tail-%d", tail),
			opts: cab.Options{Compression: cab.None},
			files: []cab.File{
				{Name: "fil_a", Data: random[:32768], Time: when},
				{Name: "fil_b", Data: random[32768 : 32768+100+tail], Time: when},
			},
		})
	}

	return cases
}

func TestNativeFDIExtracts(t *testing.T) { //nolint:paralleltest // the FDI callbacks share one package state
	if testing.Short() {
		t.Skip("native tests run without -short")
	}

	for _, tc := range nativeCases() { //nolint:paralleltest // shared FDI state
		t.Run(tc.name, func(t *testing.T) {
			raw, err := cab.Write(tc.files, tc.opts)
			if err != nil {
				t.Fatal(err)
			}

			got, err := fdiExtract(t, raw)
			if err != nil {
				t.Fatalf("cabinet.dll: %v", err)
			}

			if len(got) != len(tc.files) {
				t.Fatalf("FDI extracted %d files, want %d", len(got), len(tc.files))
			}

			for i, want := range tc.files {
				got := got[i]

				if got.Name != want.Name {
					t.Errorf("file %d: FDI reports name %q, want %q", i, got.Name, want.Name)
				}

				if !bytes.Equal(got.Data, want.Data) {
					t.Errorf("file %d (%s): FDI wrote %d bytes, want %d; data differs at %d", i, want.Name, len(got.Data), len(want.Data), firstDiff(got.Data, want.Data))
				}

				if int(got.Size) != len(want.Data) {
					t.Errorf("file %d (%s): FDI reports size %d, want %d", i, want.Name, got.Size, len(want.Data))
				}

				date, tm := cab.DOSTime(want.Time)
				if got.Date != date || got.Time != tm {
					t.Errorf("file %d (%s): FDI reports date %#x time %#x, want %#x %#x", i, want.Name, got.Date, got.Time, date, tm)
				}

				wantAttr := want.Attributes
				if wantAttr == 0 {
					wantAttr = cab.AttrArchive
				}

				for _, c := range []byte(want.Name) {
					if c >= 0x80 {
						wantAttr |= 0x80
					}
				}

				if got.Attribs != wantAttr {
					t.Errorf("file %d (%s): FDI reports attributes %#x, want %#x", i, want.Name, got.Attribs, wantAttr)
				}
			}
		})
	}
}

func TestNativeExpandExtracts(t *testing.T) { //nolint:paralleltest // the FDI callbacks share one package state
	if testing.Short() {
		t.Skip("native tests run without -short")
	}

	for _, tc := range nativeCases() { //nolint:paralleltest // shared FDI state
		t.Run(tc.name, func(t *testing.T) {
			raw, err := cab.Write(tc.files, tc.opts)
			if err != nil {
				t.Fatal(err)
			}

			got, err := expandExtract(t, raw)
			if err != nil {
				t.Fatal(err)
			}

			if len(got) != len(tc.files) {
				t.Errorf("expand.exe extracted %d files, want %d", len(got), len(tc.files))
			}

			for _, want := range tc.files {
				data, ok := got[want.Name]
				if !ok && len(tc.files) == 1 && len(got) == 1 {
					// expand.exe spells a UTF-8 flagged name through the ANSI code
					// page; only the data is compared for it.
					for _, data = range got {
						ok = true
					}
				}

				if !ok {
					t.Errorf("expand.exe did not extract %q (got %v)", want.Name, keys(got))

					continue
				}

				if !bytes.Equal(data, want.Data) {
					t.Errorf("%s: expand.exe wrote %d bytes, want %d; data differs at %d", want.Name, len(data), len(want.Data), firstDiff(data, want.Data))
				}
			}
		})
	}
}

// TestNativeReadMakecab reads a cabinet built by makecab.exe, Microsoft's own
// compressor, through cab.Read: cross-block MSZIP history, checksums and the
// DOS times are then checked against a native writer.
func TestNativeReadMakecab(t *testing.T) { //nolint:paralleltest // the FDI callbacks share one package state
	if testing.Short() {
		t.Skip("native tests run without -short")
	}

	rng := rand.New(rand.NewSource(5)) //nolint:gosec // reproducible test data
	random := make([]byte, 40000)
	rng.Read(random)

	text := bytes.Repeat([]byte("MSZIP history crosses CFDATA blocks when makecab compresses it.\r\n"), 1500)

	for _, comp := range []string{"MSZIP", "NONE"} { //nolint:paralleltest // shared FDI state
		t.Run(comp, func(t *testing.T) {
			dir := t.TempDir()
			inputs := map[string][]byte{"text.txt": text, "random.bin": random, "small.txt": []byte("small\r\n"), "empty.txt": nil}

			var ddf strings.Builder

			ddf.WriteString(".OPTION EXPLICIT\r\n.Set CabinetNameTemplate=out.cab\r\n.Set DiskDirectoryTemplate=" + dir + "\r\n")
			ddf.WriteString(".Set Cabinet=on\r\n.Set Compress=on\r\n.Set CompressionType=" + comp + "\r\n")
			ddf.WriteString(".Set MaxDiskSize=CDROM\r\n.Set RptFileName=nul\r\n.Set InfFileName=nul\r\n.Set UniqueFiles=off\r\n")

			for _, name := range []string{"text.txt", "empty.txt", "random.bin", "small.txt"} {
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, inputs[name], 0o600); err != nil {
					t.Fatal(err)
				}

				ddf.WriteString(`"` + path + `" ` + name + "\r\n")
			}

			ddfPath := filepath.Join(dir, "cab.ddf")
			if err := os.WriteFile(ddfPath, []byte(ddf.String()), 0o600); err != nil {
				t.Fatal(err)
			}

			cmd := exec.CommandContext(t.Context(), filepath.Join(os.Getenv("SystemRoot"), "System32", "makecab.exe"), "/F", ddfPath) //nolint:gosec // fixed Windows tool
			cmd.Dir = dir

			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("makecab.exe: %v\n%s", err, output)
			}

			raw, err := os.ReadFile(filepath.Join(dir, "out.cab"))
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("makecab %s: %d bytes, folders %d, files %d, blocks %d, typeCompress %#x", comp, len(raw),
				int(raw[0x1A])|int(raw[0x1B])<<8, int(raw[0x1C])|int(raw[0x1D])<<8, int(raw[36+4])|int(raw[36+5])<<8, int(raw[36+6])|int(raw[36+7])<<8)

			files, err := cab.Read(raw)
			if err != nil {
				t.Fatalf("cab.Read of a makecab cabinet: %v", err)
			}

			if len(files) != len(inputs) {
				t.Fatalf("read %d files, want %d", len(files), len(inputs))
			}

			for _, f := range files {
				want, ok := inputs[f.Name]
				if !ok {
					t.Errorf("unexpected file %q", f.Name)

					continue
				}

				if !bytes.Equal(f.Data, want) {
					t.Errorf("%s: read %d bytes, want %d; data differs at %d", f.Name, len(f.Data), len(want), firstDiff(f.Data, want))
				}

				st, _ := os.Stat(filepath.Join(dir, f.Name))
				local := st.ModTime().Local()
				date, tm := cab.DOSTime(time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), 0, time.UTC))

				if gotDate, gotTime := cab.DOSTime(f.Time); gotDate != date || gotTime != tm {
					t.Errorf("%s: read date %#x time %#x, makecab should have written the local time %#x %#x", f.Name, gotDate, gotTime, date, tm)
				}
			}
		})
	}
}

// TestNativeFDIRejectsCorruption checks that the checksum we write is the one
// cabinet.dll verifies: a flipped payload byte must be refused.
func TestNativeFDIRejectsCorruption(t *testing.T) { //nolint:paralleltest // the FDI callbacks share one package state
	if testing.Short() {
		t.Skip("native tests run without -short")
	}

	text := bytes.Repeat([]byte("checksums are verified by FDI\r\n"), 3000)

	raw, err := cab.Write([]cab.File{{Name: "fil_a", Data: text}}, cab.Options{Compression: cab.MSZIP})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fdiExtract(t, raw); err != nil {
		t.Fatalf("intact cabinet: %v", err)
	}

	bad := bytes.Clone(raw)
	bad[len(bad)-1] ^= 0x40

	_, err = fdiExtract(t, bad)
	if err == nil {
		t.Fatal("FDI extracted a cabinet with a flipped payload byte")
	}

	t.Logf("corrupt last block: %v", err)
}

func firstDiff(a, b []byte) int {
	limit := min(len(a), len(b))
	for i := range limit {
		if a[i] != b[i] {
			return i
		}
	}

	return limit
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
