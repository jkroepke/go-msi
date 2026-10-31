package msidb //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"encoding/binary"
	"io"
	"maps"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/jkroepke/msi-builder/internal/cfb"
)

// storageContents reads every stream of a storage, by name.
func storageContents(t *testing.T, storage cfb.Storage) map[string]string {
	t.Helper()

	out := make(map[string]string, len(storage.Streams))

	for _, src := range storage.Streams {
		reader, err := src.Open()
		if err != nil {
			t.Fatalf("storage %s stream %q: %v", storage.Name, src.Name, err)
		}

		data, err := io.ReadAll(reader)
		_ = reader.Close()

		if err != nil {
			t.Fatal(err)
		}

		if int64(len(data)) != src.Size {
			t.Errorf("storage %s stream %q: %d bytes, size says %d", storage.Name, src.Name, len(data), src.Size)
		}

		out[src.Name] = string(data)
	}

	return out
}

// TestStoragesRoundTrip checks that Unmarshal and Decode keep an embedded
// language transform, so that marshaling the result writes it back instead
// of silently dropping it.
func TestStoragesRoundTrip(t *testing.T) {
	t.Parallel()

	base := propertyDB([]any{"ProductName", "Example"}, []any{"ProductLanguage", "1033"})
	variant := propertyDB([]any{"ProductName", "Beispiel"}, []any{"ProductLanguage", "1031"})

	storage, err := TransformStorage("1031", base, variant, TransformOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// A large stream as well, so that one is read from sectors and not
	// only from the mini stream.
	big := bytes.Repeat([]byte("transform-"), 1000)
	storage.Streams = append(storage.Streams, cfb.Stream{Name: "Big", Data: big}.Source())

	database := propertyDB([]any{"ProductName", "Example"}, []any{"ProductLanguage", "1033"})
	database.Storages = []cfb.Storage{storage}

	first, err := database.Marshal(Options{})
	if err != nil {
		t.Fatal(err)
	}

	back, err := Unmarshal(first)
	if err != nil {
		t.Fatal(err)
	}

	if len(back.Storages) != 1 || back.Storages[0].Name != "1031" || back.Storages[0].CLSID != cfb.CLSIDTransform {
		t.Fatalf("storages after Unmarshal: %+v", back.Storages)
	}

	want := storageContents(t, storage)
	if got := storageContents(t, back.Storages[0]); !maps.Equal(got, want) {
		t.Errorf("Unmarshal storage streams %q, want %q", keysOf(got), keysOf(want))
	}

	second, err := back.Marshal(Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(first, second) {
		t.Error("Unmarshal then Marshal changed the package")
	}

	file, err := cfb.Open(bytes.NewReader(second), int64(len(second)))
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(decoded.Storages) != 1 {
		t.Fatalf("Decode returned %d storages, want 1", len(decoded.Storages))
	}

	if got := storageContents(t, decoded.Storages[0]); !maps.Equal(got, want) {
		t.Errorf("Decode storage streams %q, want %q", keysOf(got), keysOf(want))
	}
}

// TestNestedStorageRefused checks that a storage inside a storage, which
// Database.Storages cannot hold, fails decoding instead of being dropped.
func TestNestedStorageRefused(t *testing.T) {
	t.Parallel()

	database := propertyDB([]any{"ProductName", "Example"})
	database.Storages = []cfb.Storage{{Name: "1031", CLSID: cfb.CLSIDTransform, Streams: []cfb.Source{
		cfb.Stream{Name: "A", Data: []byte("a")}.Source(),
		cfb.Stream{Name: "B", Data: []byte("b")}.Source(),
		cfb.Stream{Name: "C", Data: []byte("c")}.Source(),
	}}}

	var buf bytes.Buffer
	if _, err := database.MarshalTo(&buf, Options{CFBVersion: 3}); err != nil {
		t.Fatal(err)
	}

	data := buf.Bytes()

	file, err := cfb.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	var storage *cfb.Entry

	for _, ent := range file.Entries {
		if ent.Type == 1 && ent.Name == "1031" {
			storage = ent
		}
	}

	if storage == nil {
		t.Fatal("no storage 1031")
	}

	// Turn a sibling of the storage's tree root into an empty storage.
	top := file.Entries[storage.Child]

	child := top.Left
	if child == 0xFFFFFFFF {
		child = top.Right
	}

	if child == 0xFFFFFFFF {
		t.Fatalf("unexpected tree: %+v", top)
	}

	dir := int64(binary.LittleEndian.Uint32(data[0x30:])+1) * 512
	entry := data[dir+128*int64(child):]
	entry[0x42] = 1                                         // a storage
	binary.LittleEndian.PutUint32(entry[0x4C:], 0xFFFFFFFF) // with no children
	binary.LittleEndian.PutUint32(entry[0x74:], 0)          // no start sector
	binary.LittleEndian.PutUint64(entry[0x78:], 0)          // and no size

	if _, err := Unmarshal(data); err == nil || !strings.Contains(err.Error(), "nested storages") {
		t.Errorf("Unmarshal with a nested storage: %v, want a refusal", err)
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}

// TestHugeLongStringLength checks that a long pool entry whose length does
// not fit the remaining _StringData fails before it becomes an int: on a
// 32-bit build 0x80000000 turned negative and a later string panicked.
func TestHugeLongStringLength(t *testing.T) {
	t.Parallel()

	raw, err := propertyDB([]any{"ProductName", "Example"}).Marshal(Options{CFBVersion: 3})
	if err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(raw)
	if err != nil {
		t.Fatal(err)
	}

	poolName, _ := EncodeStreamName(TableStringPool, true)
	streams := file.Streams()

	for i, stream := range streams {
		if stream.Name != poolName {
			continue
		}

		// The code page header, a long entry of 0x80000000 bytes, then
		// the original entries.
		patched := make([]byte, 0, len(stream.Data)+8)
		patched = append(patched, stream.Data[:4]...)
		patched = append(patched, 0, 0, 1, 0, 0, 0, 0, 0x80)
		patched = append(patched, stream.Data[4:]...)
		streams[i].Data = patched
	}

	bad, err := cfb.Write(streams, cfb.Options{Version: 3, CLSID: cfb.CLSIDMSI})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Unmarshal(bad); err == nil || !strings.Contains(err.Error(), "overruns _StringData") {
		t.Errorf("Unmarshal with a 2 GiB long string: %v, want an overrun error", err)
	}
}

// TestConstructedTypesChecked checks that a column type built directly,
// not by ParseType, is held to the same limits: a width of 256 would be
// stored as s0, and other values would spill into the type flags.
func TestConstructedTypesChecked(t *testing.T) {
	t.Parallel()

	for name, typ := range map[string]Type{
		"string width 256":    {Kind: KindString, Width: 256},
		"negative width":      {Kind: KindString, Width: -1},
		"int16 width 4":       {Kind: KindInt16, Width: 4},
		"localizable integer": {Kind: KindInt32, Width: 4, Localizable: true},
		"binary with width":   {Kind: KindBinary, Width: 2},
		"unknown kind":        {Kind: Kind(7)},
	} {
		tbl := &Table{Name: "T", Columns: []Column{Col("K", "s72", true), {Name: "C", Type: typ}}}
		if _, err := (&Database{Codepage: 1252, Tables: []*Table{tbl}}).Marshal(Options{}); err == nil {
			t.Errorf("%s: Marshal accepted %+v", name, typ)
		}
	}

	// Types as ParseType returns them pass, keys included.
	tbl := &Table{Name: "T", Columns: []Column{{Name: "K", Type: Type{Kind: KindString, Width: 255, Key: true}}, {Name: "C", Type: Type{Kind: KindInt32, Width: 4, Nullable: true}}}}
	if _, err := (&Database{Codepage: 1252, Tables: []*Table{tbl}}).Marshal(Options{}); err != nil {
		t.Errorf("valid constructed types: %v", err)
	}
}

// TestStreamNameCollision checks that two stored names that decode alike,
// the encoded and the literal form of one name, fail instead of one payload
// silently replacing the other.
func TestStreamNameCollision(t *testing.T) {
	t.Parallel()

	raw, err := propertyDB([]any{"ProductName", "Example"}).Marshal(Options{CFBVersion: 3})
	if err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(raw)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := EncodeStreamName("cab", false)
	if err != nil || encoded == "cab" {
		t.Fatalf("encoded name %q, %v", encoded, err)
	}

	streams := append(file.Streams(), cfb.Stream{Name: encoded, Data: []byte("first")}, cfb.Stream{Name: "cab", Data: []byte("second")})

	bad, err := cfb.Write(streams, cfb.Options{Version: 3, CLSID: cfb.CLSIDMSI})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Unmarshal(bad); err == nil || !strings.Contains(err.Error(), "both decode to") {
		t.Errorf("Unmarshal with colliding names: %v, want a refusal", err)
	}
}

// TestLoneSurrogateNameRefused checks that a stored name holding a lone
// surrogate, which reads as U+FFFD, fails instead of being written back
// under another name.
func TestLoneSurrogateNameRefused(t *testing.T) {
	t.Parallel()

	for _, inStorage := range []bool{false, true} {
		database := propertyDB([]any{"ProductName", "Example"})
		extra := cfb.Stream{Name: "Zq", Data: []byte("x")}

		if inStorage {
			database.Storages = []cfb.Storage{{Name: "1031", CLSID: cfb.CLSIDTransform, Streams: []cfb.Source{extra.Source()}}}
		} else {
			database.Streams = []cfb.Stream{extra}
		}

		var buf bytes.Buffer
		if _, err := database.MarshalTo(&buf, Options{CFBVersion: 3}); err != nil {
			t.Fatal(err)
		}

		data := buf.Bytes()
		dir := int(binary.LittleEndian.Uint32(data[0x30:])+1) * 512

		encoded := "Zq"
		if !inStorage {
			encoded, _ = EncodeStreamName("Zq", false)
		}

		patched := false

		for off := dir; off+128 <= len(data); off += 128 {
			name := data[off : off+2*len([]rune(encoded))]
			if string(name) == string(utf16le(utf16.Encode([]rune(encoded)))) {
				binary.LittleEndian.PutUint16(data[off:], 0xD800) // a lone high surrogate

				patched = true
			}
		}

		if !patched {
			t.Fatal("entry not found")
		}

		if _, err := Unmarshal(data); err == nil || !strings.Contains(err.Error(), "round-trip") {
			t.Errorf("in storage %v: Unmarshal with a lone surrogate name: %v, want a refusal", inStorage, err)
		}
	}
}

// TestOrphanTableStreamRefused checks that a table stream _Tables does not
// list fails instead of being dropped by the next Marshal.
func TestOrphanTableStreamRefused(t *testing.T) {
	t.Parallel()

	raw, err := propertyDB([]any{"ProductName", "Example"}).Marshal(Options{CFBVersion: 3})
	if err != nil {
		t.Fatal(err)
	}

	file, err := cfb.Read(raw)
	if err != nil {
		t.Fatal(err)
	}

	orphan, err := EncodeStreamName("UnknownTable", true)
	if err != nil {
		t.Fatal(err)
	}

	bad, err := cfb.Write(append(file.Streams(), cfb.Stream{Name: orphan, Data: []byte{1, 0}}), cfb.Options{Version: 3, CLSID: cfb.CLSIDMSI})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Unmarshal(bad); err == nil || !strings.Contains(err.Error(), "UnknownTable is not in _Tables") {
		t.Errorf("Unmarshal with an orphan table stream: %v, want a refusal", err)
	}
}
