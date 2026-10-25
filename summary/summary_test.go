package summary //nolint:testpackage // the tests reach unexported encoders

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func sample() []Property {
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	return []Property{
		{ID: PIDWordCount, Value: 2},
		{ID: PIDCodepage, Value: 1252},
		{ID: PIDTitle, Value: "Installation Database"},
		{ID: PIDSubject, Value: "Hello"},
		{ID: PIDAuthor, Value: "Example"},
		{ID: PIDKeywords, Value: "Installer"},
		{ID: PIDComments, Value: "This installer database contains the logic and data required to install Hello."},
		{ID: PIDTemplate, Value: "x64;1033"},
		{ID: PIDRevision, Value: "{12345678-1234-1234-1234-123456789ABC}"},
		{ID: PIDCreated, Value: when},
		{ID: PIDLastSaved, Value: when},
		{ID: PIDPageCount, Value: 500},
		{ID: PIDAppName, Value: "go-msi"},
		{ID: PIDSecurity, Value: 2},
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	out, err := Marshal(sample(), 1252)
	if err != nil {
		t.Fatal(err)
	}

	order := binary.LittleEndian

	if order.Uint16(out) != 0xFFFE || order.Uint32(out[4:]) != 0x0002000A || order.Uint32(out[0x2C:]) != 0x30 {
		t.Errorf("bad header %x", out[:0x30])
	}

	if int(order.Uint32(out[0x30:])) != len(out)-0x30 {
		t.Errorf("section size %d, stream %d", order.Uint32(out[0x30:]), len(out))
	}

	// FILETIME of 2026-01-01T00:00:00Z.
	if FileTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) != 0x01DC7AB192810000 {
		t.Errorf("FileTime = %#x", FileTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	}

	props, err := Unmarshal(out)
	if err != nil {
		t.Fatal(err)
	}

	if len(props) != 14 {
		t.Fatalf("%d properties", len(props))
	}

	// Ascending PID order.
	for i := 1; i < len(props); i++ {
		if props[i-1].ID >= props[i].ID {
			t.Errorf("PIDs out of order at %d", i)
		}
	}

	if Get(props, PIDCodepage) != 1252 || Get(props, PIDTemplate) != "x64;1033" || Get(props, PIDPageCount) != 500 {
		t.Errorf("values: %+v", props)
	}

	if got, ok := Get(props, PIDCreated).(time.Time); !ok || !got.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("created %v", got)
	}

	again, _ := Marshal(sample(), 1252)
	if !bytes.Equal(again, out) {
		t.Error("not deterministic")
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()

	if _, err := Marshal([]Property{{ID: PIDTitle, Value: "x"}}, 1252); err == nil {
		t.Error("missing code page accepted")
	}

	if _, err := Marshal([]Property{{ID: PIDCodepage, Value: 1252}}, 65001); err == nil {
		t.Error("UTF-8 summary code page accepted")
	}

	if _, err := Marshal([]Property{{ID: PIDCodepage, Value: 1252}, {ID: PIDCodepage, Value: 1252}}, 1252); err == nil {
		t.Error("duplicate PID accepted")
	}

	if _, err := Marshal([]Property{{ID: PIDCodepage, Value: 1252}, {ID: PIDTitle, Value: "日本"}}, 1252); err == nil {
		t.Error("unmappable text accepted")
	}

	if _, err := Marshal([]Property{{ID: PIDCodepage, Value: 1252}, {ID: PIDTitle, Value: 5}}, 1252); err == nil {
		t.Error("wrong type accepted")
	}

	if _, err := Marshal([]Property{{ID: PIDCodepage, Value: 1252}, {ID: 10, Value: 1}}, 1252); err == nil {
		t.Error("PID 10 accepted")
	}

	out, _ := Marshal(sample(), 1252)
	// Corrupt the string count of PID 2 into 0xFFFFFFFF.
	idx := bytes.Index(out, []byte("Installation Database")) - 4
	copy(out[idx:], []byte{0xFF, 0xFF, 0xFF, 0xFF})

	if _, err := Unmarshal(out); err == nil {
		t.Error("huge string count accepted")
	}
}
