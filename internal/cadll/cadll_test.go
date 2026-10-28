package cadll_test

import (
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/cadll"
)

// TestBinaries checks every committed DLL against SHA256SUMS and inspects
// its PE headers: the machine of its platform, the exported entry point and
// no import beyond kernel32, advapi32 and msi.
func TestBinaries(t *testing.T) {
	t.Parallel()

	sums := map[string]string{}

	for line := range strings.Lines(readFile(t, "SHA256SUMS")) {
		if sum, name, ok := strings.Cut(strings.TrimSpace(line), "  "); ok {
			sums[name] = sum
		}
	}

	for _, tc := range []struct {
		platform, file string
		machine        uint16
	}{
		{"Intel", "builderrun-x86.dll", pe.IMAGE_FILE_MACHINE_I386},
		{"x64", "builderrun-x64.dll", pe.IMAGE_FILE_MACHINE_AMD64},
		{"Arm64", "builderrun-arm64.dll", pe.IMAGE_FILE_MACHINE_ARM64},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			t.Parallel()

			data, err := cadll.Binary(tc.platform)
			if err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(sha256Sum(data)); got != sums[tc.file] {
				t.Errorf("%s: sha256 %s, SHA256SUMS has %s; rerun build.sh and commit both", tc.file, got, sums[tc.file])
			}

			img, err := pe.NewFile(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}

			if img.Machine != tc.machine {
				t.Errorf("machine %#x, want %#x", img.Machine, tc.machine)
			}

			if img.Characteristics&pe.IMAGE_FILE_DLL == 0 {
				t.Error("not a DLL")
			}

			symbols, err := img.ImportedSymbols()
			if err != nil {
				t.Fatal(err)
			}

			var libs []string

			for _, symbol := range symbols {
				if _, lib, ok := strings.Cut(symbol, ":"); ok && !slices.Contains(libs, lib) {
					libs = append(libs, lib)
				}
			}

			slices.Sort(libs)

			if want := []string{"advapi32.dll", "kernel32.dll", "msi.dll"}; !slices.Equal(libs, want) {
				t.Errorf("imports %v, want %v", libs, want)
			}

			if !bytes.Contains(data, []byte(cadll.Entry+"\x00")) {
				t.Errorf("export %s not found", cadll.Entry)
			}
		})
	}

	if _, err := cadll.Binary("Intel64"); err == nil {
		t.Error("Intel64 has no DLL and should fail")
	}
}

func TestData(t *testing.T) {
	t.Parallel()

	if got, want := cadll.Data(`C:\App\`, `"C:\App\a.exe" x`), `C:\App\|"C:\App\a.exe" x`; got != want {
		t.Errorf("Data = %q, want %q", got, want)
	}

	if got, want := cadll.Data("", "cmd"), "|cmd"; got != want {
		t.Errorf("Data = %q, want %q", got, want)
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)

	return sum[:]
}
