//go:build windows

package pe //nolint:testpackage // temporary probe

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// TestProbeRawSizes is a temporary probe: it reports what MsiGetFileVersionW
// returns for resource sections whose SizeOfRawData ends at each offset.
func TestProbeRawSizes(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	dir := t.TempDir()

	for _, raw := range []uint32{1, 2, 4, 8, 0x10, 0x20, 0x40, 0x58, 0x60, 0x80, 0x90, 0xA0, 0xB4, 0xC0, 0xD4, 0xE0, 0xF0, 0xF8, 0xFC, 0x100, 0x180, 0x1FF, 0x200} {
		for _, langs := range [][]int{{1033}, {1033, 1031}} {
			data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: langs})
			binary.LittleEndian.PutUint32(data[testRsrcSection+16:], raw)

			path := filepath.Join(dir, fmt.Sprintf("p%x-%d.exe", raw, len(langs)))
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}

			ver, lang, ok := msiFileVersion(t, path)
			fmt.Fprintf(&out, "raw %#x langs %v: ok=%v %q %q\n", raw, langs, ok, ver, lang)
		}
	}

	t.Errorf("probe:\n%s", out.String())
}
