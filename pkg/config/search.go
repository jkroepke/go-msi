package config

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SearchFoundProperty returns the public property that AppSearch fills for a
// searched property, before an action copies it over.
func SearchFoundProperty(name string) string {
	const suffix = "_FOUND"
	if len(name)+len(suffix) <= MaxIdentifierLength {
		return name + suffix
	}

	sum := sha256.Sum256([]byte(name))

	return "FOUND_" + strings.ToUpper(hex.EncodeToString(sum[:16]))
}
