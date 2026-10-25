package codepage_test

import (
	"testing"

	"github.com/jkroepke/go-msi/codepage"
)

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		cp   int
		s    string
		want []byte
	}{
		{codepage.ASCII, "Hello", []byte("Hello")},
		{codepage.UTF8, "Grüße €", []byte("Grüße €")},
		{codepage.Windows1252, "Grüße €", []byte{'G', 'r', 0xFC, 0xDF, 'e', ' ', 0x80}},
		{932, "日本", []byte{0x93, 0xFA, 0x96, 0x7B}},
	}

	for _, c := range cases {
		got, err := codepage.Encode(c.s, c.cp)
		if err != nil {
			t.Errorf("Encode(%q, %d): %v", c.s, c.cp, err)

			continue
		}

		if string(got) != string(c.want) {
			t.Errorf("Encode(%q, %d) = % x, want % x", c.s, c.cp, got, c.want)
		}

		back, err := codepage.Decode(got, c.cp)
		if err != nil || back != c.s {
			t.Errorf("Decode(%d) = %q, %v", c.cp, back, err)
		}
	}
}

func TestErrors(t *testing.T) {
	if _, err := codepage.Encode("日本", codepage.Windows1252); err == nil {
		t.Error("expected an error for characters outside the code page")
	}

	if _, err := codepage.Encode("é", codepage.ASCII); err == nil {
		t.Error("expected an error for non-ASCII in code page 0")
	}

	if _, err := codepage.Encode("x", 12345); err == nil {
		t.Error("expected an error for an unknown code page")
	}

	if codepage.Supported(12345) || !codepage.Supported(1252) || !codepage.IsANSI(1252) || codepage.IsANSI(codepage.UTF8) {
		t.Error("Supported/IsANSI")
	}
}
