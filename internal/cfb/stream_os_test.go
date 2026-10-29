package cfb_test

import (
	"os"
	"testing"
)

func mustCreate(t *testing.T, path string) *os.File {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = file.Close() })

	return file
}
