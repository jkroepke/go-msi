// Package codepage converts text to and from the Windows code pages a
// Windows Installer database may declare. Unmappable text is an error, never
// a substitution character (docs/spec/MSISPEC.md section 3, "Lossless
// database serialization").
package codepage

import (
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

var ansi = map[int]encoding.Encoding{
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
func Encode(s string, cp int) ([]byte, error) {
	if !utf8.ValidString(s) {
		return nil, errors.New("codepage: text is not valid UTF-8")
	}

	switch cp {
	case UTF8:
		return []byte(s), nil
	case ASCII:
		for i := 0; i < len(s); i++ {
			if s[i] >= 0x80 {
				return nil, fmt.Errorf("codepage: %q is not ASCII and the database code page is 0", s)
			}
		}

		return []byte(s), nil
	}

	enc, ok := ansi[cp]
	if !ok {
		return nil, fmt.Errorf("codepage: code page %d is not supported", cp)
	}

	out, err := enc.NewEncoder().Bytes([]byte(s))
	if err != nil {
		return nil, fmt.Errorf("codepage: %q cannot be represented in code page %d", s, cp)
	}

	return out, nil
}

// Decode converts bytes in the code page to a string.
func Decode(b []byte, cp int) (string, error) {
	switch cp {
	case UTF8:
		if !utf8.Valid(b) {
			return "", errors.New("codepage: invalid UTF-8")
		}

		return string(b), nil
	case ASCII:
		for _, c := range b {
			if c >= 0x80 {
				return "", errors.New("codepage: non-ASCII byte in a code page 0 database")
			}
		}

		return string(b), nil
	}

	enc, ok := ansi[cp]
	if !ok {
		return "", fmt.Errorf("codepage: code page %d is not supported", cp)
	}

	out, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return "", fmt.Errorf("codepage: invalid bytes for code page %d", cp)
	}

	return string(out), nil
}
