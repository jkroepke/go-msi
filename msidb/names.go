package msidb

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Stream name encoding of section 3: table streams carry a U+4840 prefix,
// pairs of alphabet characters pack into U+3800–U+47FF and single ones into
// U+4800–U+483F; every other character is copied.
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz._"

const (
	tablePrefix = 0x4840
	pairBase    = 0x3800
	singleBase  = 0x4800
	maxEncoded  = 31
)

func alphaIndex(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'A' && r <= 'Z':
		return int(r-'A') + 10
	case r >= 'a' && r <= 'z':
		return int(r-'a') + 36
	case r == '.':
		return 62
	case r == '_':
		return 63
	}

	return -1
}

// EncodeStreamName encodes a decoded stream or table name. It fails when the
// result exceeds 31 UTF-16 units, which is the engine's limit (DB-11).
func EncodeStreamName(name string, table bool) (string, error) {
	if name == "" {
		return "", errors.New("msidb: empty stream name")
	}

	var out []uint16

	if table {
		out = append(out, tablePrefix)
	}

	rs := []rune(name)

	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r >= pairBase && r <= tablePrefix {
			return "", fmt.Errorf("msidb: stream name %q contains the reserved character %U", name, r)
		}

		a := alphaIndex(r)
		if a < 0 {
			out = append(out, utf16.Encode([]rune{r})...)

			continue
		}

		if i+1 < len(rs) {
			if b := alphaIndex(rs[i+1]); b >= 0 {
				out = append(out, uint16(pairBase+a+(b<<6))) //nolint:gosec // bounded by the format limits checked earlier
				i++

				continue
			}
		}

		out = append(out, uint16(singleBase+a)) //nolint:gosec // bounded by the format limits checked earlier
	}

	if len(out) > maxEncoded {
		return "", fmt.Errorf("msidb: stream name %q encodes to %d UTF-16 units, the limit is %d", name, len(out), maxEncoded)
	}

	return string(utf16.Decode(out)), nil
}

// DecodeStreamName decodes an encoded directory entry name.
func DecodeStreamName(encoded string) (name string, table bool) {
	u := utf16.Encode([]rune(encoded))
	if len(u) > 0 && u[0] == tablePrefix {
		table = true
		u = u[1:]
	}

	var rs []rune

	for _, c := range u {
		switch {
		case c >= pairBase && c < singleBase:
			v := c - pairBase
			rs = append(rs, rune(alphabet[v&0x3F]), rune(alphabet[(v>>6)&0x3F]))
		case c >= singleBase && c < tablePrefix:
			rs = append(rs, rune(alphabet[c-singleBase]))
		default:
			rs = append(rs, rune(c))
		}
	}

	// Surrogate pairs copied through come back as two runes; re-join them.
	return string(utf16.Decode(utf16.Encode(rs))), table
}

// BinaryStreamName is the decoded name of the stream holding a binary cell:
// the table name and the row's key values joined with ".", integer keys in
// decimal.
func BinaryStreamName(table string, keys []any) string {
	parts := []string{table}

	for _, k := range keys {
		switch v := k.(type) {
		case string:
			parts = append(parts, v)
		case int:
			parts = append(parts, strconv.Itoa(v))
		default:
			parts = append(parts, fmt.Sprint(v))
		}
	}

	return strings.Join(parts, ".")
}
