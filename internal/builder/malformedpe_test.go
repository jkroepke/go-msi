package builder_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf16"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// malformedVersionPE returns a PE file whose RT_VERSION data entry claims a
// one-byte resource, so that pe.Parse fails although the file is a PE image.
func malformedVersionPE(t *testing.T) []byte {
	t.Helper()

	exe := testpe.Build(testpe.Options{FileVersion: "1.2.3.4"})

	key := make([]byte, 0, 30)
	for _, r := range utf16.Encode([]rune("VS_VERSION_INFO")) {
		key = binary.LittleEndian.AppendUint16(key, r)
	}

	// VS_VERSIONINFO starts with three WORDs before its key, and the data
	// entry (RVA, Size, CodePage, Reserved) directly precedes it.
	start := bytes.Index(exe, key) - 6
	if start < 16 {
		t.Fatal("no version resource in the test PE")
	}

	binary.LittleEndian.PutUint32(exe[start-12:], 1)

	return exe
}

// TestMalformedPEVersionIsReported checks that a PE file whose version
// resource cannot be read is not packaged as an ordinary unversioned file:
// it fails like an unversioned executable (FILE-01) unless the configuration
// accepts that or sets the version itself (REVIEW1 #6).
func TestMalformedPEVersionIsReported(t *testing.T) {
	t.Parallel()

	const base = `schema: msi-builder/v1
product: { name: Hello, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files: [{ source: hello.exe%s }]
`

	for _, tc := range []struct {
		name, extra string
		fails       bool
	}{
		{"auto version", "", true},
		{"allowed unversioned", ", allowUnversionedExecutable: true", false},
		{"explicit version", ", version: 1.2.3.4", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.Parse(fmt.Appendf(nil, base, tc.extra), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.FS = fstest.MapFS{"hello.exe": {Data: malformedVersionPE(t)}}

			res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
			diags := fmt.Sprint(res.Diagnostics)

			if !strings.Contains(diags, "components[0].files[0].source: hello.exe is a PE file whose version resource cannot be read") {
				t.Errorf("no diagnostic for the malformed version resource: %s", diags)
			}

			if strings.Count(diags, "version resource cannot be read") != 1 || strings.Contains(diags, "without a version resource") {
				t.Errorf("want exactly one diagnostic about the version resource: %s", diags)
			}

			if (err != nil) != tc.fails {
				t.Errorf("Build error %v, want failure %v\n%s", err, tc.fails, diags)
			}
		})
	}
}
