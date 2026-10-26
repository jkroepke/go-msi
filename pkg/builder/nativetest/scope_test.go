package nativetest_test

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/jkroepke/go-msi/internal/exampletest"
	"github.com/jkroepke/go-msi/pkg/builder"
)

// scope is what a snapshot needs to know about the package: the labels for
// machine paths, the input files and the names the package can own.
type scope struct {
	label  func(string) string
	inputs map[string][]byte
	// strings holds every string of the package, lower-cased and separated
	// by NUL.
	strings     string
	productCode string
}

// newScope collects the strings of every table of the package.
func newScope(res *builder.Result, label func(string) string, inputs map[string][]byte) scope {
	var all strings.Builder

	for _, tbl := range res.Database.Tables {
		for _, row := range tbl.Rows {
			for _, cell := range row {
				if text, ok := cell.(string); ok {
					all.WriteString(strings.ToLower(text) + "\x00")
				}
			}
		}
	}

	return scope{label: label, inputs: inputs, strings: all.String(), productCode: res.ProductCode}
}

// owns reports whether a child of a watch can belong to the package: its
// name, with or without its extension, occurs as a whole word in one of the
// package's strings, or it carries the product code. Other programs on the
// machine write next to the package at the same time, such as
// HKCU\Software\Mozilla on a GitHub runner, and stay out of the snapshot.
func (pkg scope) owns(name string) bool {
	if pkg.productCode != "" && strings.Contains(strings.ToUpper(name), strings.ToUpper(pkg.productCode)) {
		return true
	}

	lower := strings.ToLower(name)
	if pkg.hasWord(lower) {
		return true
	}

	stem := strings.TrimSuffix(lower, filepath.Ext(lower))

	return stem != "" && stem != lower && pkg.hasWord(stem)
}

// hasWord reports whether word occurs in the package's strings with no
// letter, digit, '_', '-' or '.' right before or after it, so "github" is
// not found in "github.com" and "go" not in "google".
func (pkg scope) hasWord(word string) bool {
	if word == "" {
		return false
	}

	for rest := pkg.strings; ; {
		i := strings.Index(rest, word)
		if i < 0 {
			return false
		}

		end := i + len(word)
		if (i == 0 || !wordByte(rest[i-1])) && (end == len(rest) || !wordByte(rest[end])) {
			return true
		}

		rest = rest[i+1:]
	}
}

// wordByte reports whether b continues a name. Bytes above ASCII count as
// part of a name.
func wordByte(b byte) bool {
	r := rune(b)

	return r > unicode.MaxASCII || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}

// TestScopeOwns checks the ownership rule of the install snapshots on every
// platform: the top-level entries of the snapshots in testdata/install
// belong to their package, and what other programs write on a runner does
// not.
func TestScopeOwns(t *testing.T) {
	t.Parallel()

	owned := map[string][]string{
		"minimal":          {"Hello", "{ProductCode}", "SourceHash{ProductCode}"},
		"desktop":          {"Example Viewer", "Example Viewer.lnk", "Example", "{ProductCode}"},
		"branded":          {"Example", "ExampleAgent", "{ProductCode}"},
		"windows_exporter": {"windows_exporter", "{ProductCode}", "SourceHash{ProductCode}"},
	}
	foreign := []string{
		"Mozilla", "Google", "GitHub", "Microsoft", "Package Cache", ".NET", ".cache", "MpKsl1a2b3c4d", "Go", "",
	}
	examples := exampletest.Examples()

	for name, names := range owned {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			example := examples[name]
			_, res := exampletest.Build(t, name, example.Vars, example.Inputs)
			pkg := newScope(res, nil, example.Inputs)

			for _, entry := range names {
				entry = strings.ReplaceAll(entry, "{ProductCode}", res.ProductCode)
				if !pkg.owns(entry) {
					t.Errorf("%q is not owned", entry)
				}
			}

			for _, entry := range foreign {
				if pkg.owns(entry) {
					t.Errorf("%q is owned", entry)
				}
			}
		})
	}
}
