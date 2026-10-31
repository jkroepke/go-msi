package codepage_test

import (
	"testing"

	"github.com/jkroepke/msi-builder/internal/codepage"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()

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

	for _, tc := range cases {
		got, err := codepage.Encode(tc.s, tc.cp)
		if err != nil {
			t.Errorf("Encode(%q, %d): %v", tc.s, tc.cp, err)

			continue
		}

		if string(got) != string(tc.want) {
			t.Errorf("Encode(%q, %d) = % x, want % x", tc.s, tc.cp, got, tc.want)
		}

		back, err := codepage.Decode(got, tc.cp)
		if err != nil || back != tc.s {
			t.Errorf("Decode(%d) = %q, %v", tc.cp, back, err)
		}
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()

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

// TestDecodeMalformed checks that bytes the code page cannot map fail
// instead of turning into U+FFFD, which the x/text decoders do silently.
func TestDecodeMalformed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		data []byte
		cp   int
	}{
		{[]byte{0x81}, 932},           // a lead byte without its trail byte
		{[]byte{'a', 0x81, ' '}, 932}, // a lead byte with an invalid trail byte
		{[]byte{0x81}, 1252},          // unassigned in 1252
		{[]byte{0x98}, 1251},          // unassigned in 1251
		{[]byte{0xFF}, 949},           // no character
		{[]byte{0xA4}, 950},           // a lead byte alone
		{[]byte{0xEF, 0xBF}, 65001},   // a truncated UTF-8 sequence
		{[]byte{0x88, 0x62}, 950},     // read as Ê with U+0304, which 950 cannot write
		{[]byte{0x81, 0x30}, 936},     // a GB18030 four-byte prefix, not GBK
	} {
		if got, err := codepage.Decode(tc.data, tc.cp); err == nil {
			t.Errorf("Decode(% x, %d) = %q, want an error", tc.data, tc.cp, got)
		}
	}

	// Two byte sequences for one character are both read, as Windows
	// reads them: NEC row 13 0x8790 and JIS 0x81E0 are both U+2252.
	// So are IBM 0xFA5C and NEC-selected IBM 0xED40, both U+7E8A.
	for want, seqs := range map[string][][]byte{
		"\u2252": {{0x87, 0x90}, {0x81, 0xE0}},
		"\u7E8A": {{0xFA, 0x5C}, {0xED, 0x40}},
	} {
		for _, data := range seqs {
			if got, err := codepage.Decode(data, 932); err != nil || got != want {
				t.Errorf("Decode(% x, 932) = %q, %v, want %q", data, got, err, want)
			}
		}
	}

	// A U+FFFD that was really written survives.
	if got, err := codepage.Decode([]byte("a�b"), codepage.UTF8); err != nil || got != "a�b" {
		t.Errorf("Decode of an encoded U+FFFD = %q, %v", got, err)
	}
}
