package rtf_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/internal/rtf"
)

func TestFromText(t *testing.T) {
	t.Parallel()

	got, err := rtf.FromText([]byte("\xEF\xBB\xBFMIT {License}\r\nC:\\path\tend\r\n\fÄ € 😀\n\n"))
	if err != nil {
		t.Fatal(err)
	}

	if !rtf.IsRTF([]byte(got)) || !strings.HasSuffix(got, "\\par\n}\n") {
		t.Fatalf("not a document: %q", got)
	}

	body := got[strings.Index(got, `\fs16 `)+len(`\fs16 `):]
	want := "MIT \\{License\\}\\par\nC:\\\\path\\tab end\\par\n\\u196? \\u8364? \\u-10179?\\u-8704?\\par\n}\n"

	if body != want {
		t.Errorf("body\n%q\nwant\n%q", body, want)
	}

	if _, err := rtf.Check([]byte(got)); err != nil {
		t.Errorf("converted text fails Check: %v", err)
	}
}

func TestFromTextUTF16(t *testing.T) {
	t.Parallel()

	got, err := rtf.FromText([]byte{0xFF, 0xFE, 'H', 0, 0xE4, 0, '\n', 0})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, `H\u228?\par`) {
		t.Errorf("got %q", got)
	}

	if _, err := rtf.FromText([]byte{0xFE, 0xFF, 0, 'H', 0}); err == nil {
		t.Error("odd UTF-16 length accepted")
	}

	if _, err := rtf.FromText([]byte{'a', 0xE4, 'b'}); err == nil {
		t.Error("Latin-1 text accepted as UTF-8")
	}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, doc, want string
	}{
		{"valid", `{\rtf1\ansi{\fonttbl{\f0 Arial;}}\f0 te\'e9 \{x\} \\}` + "\r\n", ""},
		{"bom", "\xEF\xBB\xBF{\\rtf1 x}", ""},
		{"trailing NUL", "{\\rtf1 x}\r\n\x00", ""},
		{"not rtf", `hello`, `does not start`},
		{"non-ASCII", "{\\rtf1 te\xE9}", "byte 0xE9 on line 1"},
		{"control byte", "{\\rtf1\n te\x01}", "byte 0x01 on line 2"},
		{"unclosed", `{\rtf1 {x}`, "1 unclosed"},
		{"extra close", `{\rtf1 x}}`, "text after"},
		{"trailing text", `{\rtf1 x} more`, "text after"},
	} {
		got, err := rtf.Check([]byte(tc.doc))
		if err == nil && strings.ContainsAny(got[strings.LastIndexByte(got, '}'):], "\r\n\x00") {
			t.Errorf("%s: kept bytes after the document: %q", tc.name, got)
		}

		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s: err %v, want %q", tc.name, err, tc.want)
		}
	}
}
