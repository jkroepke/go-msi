package pe_test

import (
	"bytes"
	"testing"

	"github.com/jkroepke/go-msi/internal/testpe"
	"github.com/jkroepke/go-msi/pe"
)

// FuzzParse feeds malformed PE files to the parser.
func FuzzParse(f *testing.F) {
	f.Add(testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}}))
	f.Add([]byte("MZ"))

	f.Fuzz(func(t *testing.T, raw []byte) {
		info, err := pe.Parse(bytes.NewReader(raw))
		if err != nil {
			return
		}

		if !pe.IsPE(bytes.NewReader(raw)) {
			t.Fatal("Parse accepts a file IsPE rejects")
		}

		_ = info.Language()
	})
}
