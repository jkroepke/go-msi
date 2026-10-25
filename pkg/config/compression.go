package config

import (
	"strconv"
	"strings"
)

// Compressions returns the values of package.compression: "lzx" is LZX with
// a 2 MB window, "lzx:N" a window of 2^N bytes.
func Compressions() []string {
	return []string{"lzx", "lzx:15", "lzx:16", "lzx:17", "lzx:18", "lzx:19", "lzx:20", "lzx:21", "mszip", None}
}

// LZXWindow returns the window exponent of an LZX compression value and
// whether the value selects LZX.
func LZXWindow(compression string) (int, bool) {
	if compression == "lzx" {
		return 21, true
	}

	n, ok := strings.CutPrefix(compression, "lzx:")
	if !ok {
		return 0, false
	}

	window, err := strconv.Atoi(n)
	if err != nil {
		return 0, false
	}

	return window, true
}
