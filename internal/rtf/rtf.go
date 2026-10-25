// Package rtf prepares the license text of the setup wizard. The
// ScrollableText control shows rich text (RTF) and stores it raw in the
// Control table, so a plain-text license is converted here and an RTF file
// is checked before it is embedded.
package rtf

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// header starts every converted document: ANSI with one fallback character
// per \u escape and Courier New at 8 points, a fixed-width font that keeps
// the layout of hand-wrapped license files.
const header = `{\rtf1\ansi\ansicpg1252\deff0\uc1{\fonttbl{\f0\fmodern\fcharset0 Courier New;}}` + "\n" + `\viewkind4\pard\f0\fs16 `

// IsRTF reports whether data is a rich text document.
func IsRTF(data []byte) bool {
	return bytes.HasPrefix(skipBOM(data), []byte(`{\rtf`))
}

// FromText converts a plain-text document to RTF. The text is UTF-8 or
// UTF-16 with a byte order mark. Every non-ASCII character is written as a
// \u escape, so the result is plain ASCII and fits any package code page.
func FromText(data []byte) (string, error) {
	text, err := decodeText(data)
	if err != nil {
		return "", err
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimRight(text, "\n")

	var out strings.Builder

	out.WriteString(header)

	for _, char := range text {
		writeChar(&out, char)
	}

	out.WriteString("\\par\n}\n")

	return out.String(), nil
}

// writeChar writes one character of plain text as RTF.
func writeChar(out *strings.Builder, char rune) {
	switch {
	case char == '\n':
		out.WriteString("\\par\n")
	case char == '\t':
		out.WriteString("\\tab ")
	case strings.ContainsRune(`\{}`, char):
		out.WriteByte('\\')
		out.WriteRune(char)
	case char < 0x20 || char == 0x7F:
		// Control characters have no RTF form; a form feed between
		// license sections is dropped.
	case char < 0x80:
		out.WriteRune(char)
	default:
		for _, unit := range utf16.Encode([]rune{char}) {
			// \u takes a signed 16-bit number; ? is the fallback.
			out.WriteString(`\u` + strconv.Itoa(int(int16(unit))) + "?") //nolint:gosec // a UTF-16 unit, reinterpreted as RTF wants
		}
	}
}

func decodeText(data []byte) (string, error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return decodeUTF16(data[2:], false)
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return decodeUTF16(data[2:], true)
	}

	data = skipBOM(data)
	if !utf8.Valid(data) {
		return "", errors.New("the license is not UTF-8 text; save it as UTF-8 or as RTF")
	}

	return string(data), nil
}

func decodeUTF16(data []byte, bigEndian bool) (string, error) {
	if len(data)%2 != 0 {
		return "", errors.New("the license is UTF-16 with an odd number of bytes")
	}

	units := make([]uint16, len(data)/2)
	for i := range units {
		if bigEndian {
			units[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
		} else {
			units[i] = uint16(data[2*i+1])<<8 | uint16(data[2*i])
		}
	}

	return string(utf16.Decode(units)), nil
}

func skipBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

// Check validates an RTF document for the Control table and returns it as
// a string. The document must be 7-bit ASCII: the Control table stores
// text in the package code page, and an RTF writer marks other characters
// with \'xx or \u escapes. Braces must balance, so that the control does
// not swallow or cut off the text.
func Check(data []byte) (string, error) {
	data = skipBOM(data)

	if !IsRTF(data) {
		return "", errors.New(`the license does not start with {\rtf`)
	}

	end, err := documentEnd(data)
	if err != nil {
		return "", err
	}

	// Writers often end the file with a line break or a NUL after the
	// document; only the document is stored.
	data = data[:end]

	for i, char := range data {
		if char >= 0x80 || (char < ' ' && char != '\r' && char != '\n' && char != '\t') {
			line := 1 + bytes.Count(data[:i], []byte("\n"))

			return "", fmt.Errorf("the license has the byte 0x%02X on line %d; RTF writes such characters as \\'xx or \\u escapes", char, line)
		}
	}

	return string(data), nil
}

// documentEnd checks that the groups of an RTF document nest and that only
// white space and NULs follow the document's closing brace, and returns the
// length of the document.
func documentEnd(data []byte) (int, error) {
	depth := 0
	end := -1

	for i := 0; i < len(data); i++ {
		char := data[i]

		if end >= 0 {
			if char > ' ' {
				return 0, errors.New("the license has text after the closing brace of the document")
			}

			continue
		}

		switch char {
		case '\\':
			i++ // the escaped character, also \{ \} and \\
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
	}

	if depth != 0 {
		return 0, fmt.Errorf("the license has %d unclosed braces", depth)
	}

	return end, nil
}
