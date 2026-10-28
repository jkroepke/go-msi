package pe_test

import (
	"bytes"
	"testing"

	"github.com/jkroepke/msi-builder/internal/pe"
	"github.com/jkroepke/msi-builder/internal/testpe"
)

// FuzzParse feeds malformed PE files to the parser.
func FuzzParse(f *testing.F) {
	f.Add(testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}}))
	f.Add([]byte("MZ"))

	f.Fuzz(func(_ *testing.T, raw []byte) {
		_ = pe.IsPE(bytes.NewReader(raw))

		info, err := pe.Parse(bytes.NewReader(raw))
		if err != nil {
			return
		}

		_ = info.Language()
	})
}
