// Package codepage converts text to and from the Windows code pages a
// Windows Installer database may declare. Unmappable text is an error, never
// a substitution character (docs/spec/MSISPEC.md section 3, "Lossless
// database serialization").
package codepage

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Well-known code pages.
const (
	// ASCII is code page 0: the database holds 7-bit ASCII only.
	ASCII = 0
	// UTF8 is code page 65001.
	UTF8 = 65001
	// Windows1252 is the Western European ANSI page and the default.
	Windows1252 = 1252
)

var ansi = map[int]encoding.Encoding{ //nolint:gochecknoglobals // lookup table
	874:  charmap.Windows874,
	932:  japanese.ShiftJIS,
	936:  simplifiedchinese.GBK,
	949:  korean.EUCKR,
	950:  traditionalchinese.Big5,
	1250: charmap.Windows1250,
	1251: charmap.Windows1251,
	1252: charmap.Windows1252,
	1253: charmap.Windows1253,
	1254: charmap.Windows1254,
	1255: charmap.Windows1255,
	1256: charmap.Windows1256,
	1257: charmap.Windows1257,
	1258: charmap.Windows1258,
}

// IsANSI reports whether cp is an ANSI code page that the Summary
// Information stream may declare (SUM-04: 65001 and 1200 fail with 1620).
func IsANSI(cp int) bool {
	_, ok := ansi[cp]

	return ok
}

// Supported reports whether this package can encode cp.
func Supported(cp int) bool {
	return cp == ASCII || cp == UTF8 || IsANSI(cp)
}

// Encode converts s to the code page.
func Encode(text string, codePage int) ([]byte, error) {
	if !utf8.ValidString(text) {
		return nil, errors.New("codepage: text is not valid UTF-8")
	}

	switch codePage {
	case UTF8:
		return []byte(text), nil
	case ASCII:
		for i := range len(text) {
			if text[i] >= 0x80 {
				return nil, fmt.Errorf("codepage: %q is not ASCII and the database code page is 0", text)
			}
		}

		return []byte(text), nil
	}

	enc, ok := ansi[codePage]
	if !ok {
		return nil, fmt.Errorf("codepage: code page %d is not supported", codePage)
	}

	out, err := enc.NewEncoder().Bytes([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("codepage: %q cannot be represented in code page %d", text, codePage)
	}

	return out, nil
}

// Decode converts bytes in the code page to a string.
func Decode(data []byte, codePage int) (string, error) {
	switch codePage {
	case UTF8:
		if !utf8.Valid(data) {
			return "", errors.New("codepage: invalid UTF-8")
		}

		return string(data), nil
	case ASCII:
		for _, c := range data {
			if c >= 0x80 {
				return "", errors.New("codepage: non-ASCII byte in a code page 0 database")
			}
		}

		return string(data), nil
	}

	enc, ok := ansi[codePage]
	if !ok {
		return "", fmt.Errorf("codepage: code page %d is not supported", codePage)
	}

	out, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return "", fmt.Errorf("codepage: invalid bytes for code page %d", codePage)
	}

	// The x/text decoders turn malformed or unmapped bytes into U+FFFD
	// without an error. No ANSI code page can encode U+FFFD, so one in
	// the result always stands for bytes that were lost. A byte-for-byte
	// round trip would be too strict: Windows code pages map some
	// characters from two byte sequences (932 NEC duplicates, 936 0xA2E3
	// and 0x80) and encode them back to one of them.
	if bytes.ContainsRune(out, utf8.RuneError) {
		return "", fmt.Errorf("codepage: invalid bytes for code page %d", codePage)
	}

	return string(out), nil
}
