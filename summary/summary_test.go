package summary

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func sample() []Property {
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	return []Property{
		{PIDWordCount, 2},
		{PIDCodepage, 1252},
		{PIDTitle, "Installation Database"},
		{PIDSubject, "Hello"},
		{PIDAuthor, "Example"},
		{PIDKeywords, "Installer"},
		{PIDComments, "This installer database contains the logic and data required to install Hello."},
		{PIDTemplate, "x64;1033"},
		{PIDRevision, "{12345678-1234-1234-1234-123456789ABC}"},
		{PIDCreated, when},
		{PIDLastSaved, when},
		{PIDPageCount, 500},
		{PIDAppName, "go-msi"},
		{PIDSecurity, 2},
	}
}

func TestRoundTrip(t *testing.T) {
	out, err := Marshal(sample(), 1252)
	if err != nil {
		t.Fatal(err)
	}

	le := binary.LittleEndian

	if le.Uint16(out) != 0xFFFE || le.Uint32(out[4:]) != 0x0002000A || le.Uint32(out[0x2C:]) != 0x30 {
		t.Errorf("bad header %x", out[:0x30])
	}

	if int(le.Uint32(out[0x30:])) != len(out)-0x30 {
		t.Errorf("section size %d, stream %d", le.Uint32(out[0x30:]), len(out))
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

	if got := Get(props, PIDCreated).(time.Time); !got.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("created %v", got)
	}

	again, _ := Marshal(sample(), 1252)
	if !bytes.Equal(again, out) {
		t.Error("not deterministic")
	}
}

func TestErrors(t *testing.T) {
	if _, err := Marshal([]Property{{PIDTitle, "x"}}, 1252); err == nil {
		t.Error("missing code page accepted")
	}

	if _, err := Marshal([]Property{{PIDCodepage, 1252}}, 65001); err == nil {
		t.Error("UTF-8 summary code page accepted")
	}

	if _, err := Marshal([]Property{{PIDCodepage, 1252}, {PIDCodepage, 1252}}, 1252); err == nil {
		t.Error("duplicate PID accepted")
	}

	if _, err := Marshal([]Property{{PIDCodepage, 1252}, {PIDTitle, "日本"}}, 1252); err == nil {
		t.Error("unmappable text accepted")
	}

	if _, err := Marshal([]Property{{PIDCodepage, 1252}, {PIDTitle, 5}}, 1252); err == nil {
		t.Error("wrong type accepted")
	}

	if _, err := Marshal([]Property{{PIDCodepage, 1252}, {10, 1}}, 1252); err == nil {
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
