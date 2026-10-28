package codepage_test

import (
	"testing"

	"github.com/jkroepke/msi-builder/internal/codepage"
)

// FuzzRoundTrip encodes text and decodes it back to the same string.
func FuzzRoundTrip(f *testing.F) {
	for _, cp := range []int{0, 1252, 1251, 932, 936, 65001} {
		f.Add("Hello ö Ж 日本", cp)
	}

	f.Fuzz(func(t *testing.T, text string, codePage int) {
		raw, err := codepage.Encode(text, codePage)
		if err != nil {
			return
		}

		got, err := codepage.Decode(raw, codePage)
		if err != nil {
			t.Fatalf("decode of encoded %q in %d: %v", text, codePage, err)
		}

		if got != text {
			t.Fatalf("code page %d: %q encodes to %x and decodes to %q", codePage, text, raw, got)
		}
	})
}
