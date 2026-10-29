// Package exampletest builds the configurations in examples/ for tests. Each
// example gets synthetic input files and fixed variables, so the same
// example always gives the same package.
package exampletest

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// BuildTime is the build time of every example package.
var BuildTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) //nolint:gochecknoglobals // a fixed value shared by the tests

// versionVar is the variable the examples take their version from.
const versionVar = "VERSION"

// Example is the variables and input files one example is built with.
type Example struct {
	Vars   map[string]string
	Inputs map[string][]byte
}

// Examples returns every example by directory name. Each call returns new
// maps, so a caller may change them.
func Examples() map[string]Example {
	icon := append([]byte{0, 0, 1, 0, 1, 0}, make([]byte, 64)...)

	return map[string]Example{
		"minimal": {
			Inputs: map[string][]byte{
				"hello.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
			},
		},
		"windows_exporter": {
			Vars: map[string]string{versionVar: "0.32.0.5"},
			Inputs: map[string][]byte{
				"windows_exporter.exe": testpe.Build(testpe.Options{FileVersion: "0.32.0.5", Languages: []int{1033}}),
				"icon.ico":             icon,
			},
		},
		"branded": {
			Vars: map[string]string{versionVar: "1.5.0"},
			Inputs: map[string][]byte{
				"agent.exe":         testpe.Build(testpe.Options{FileVersion: "1.5.0.0", Languages: []int{1033}}),
				"agent-service.exe": testpe.Build(testpe.Options{FileVersion: "1.5.0.0", Languages: []int{1033}}),
				"agent.conf":        []byte("logging {}\n"),
				"icon.ico":          bytes.Clone(icon),
				"LICENSE.rtf":       []byte(`{\rtf1\ansi\deff0{\fonttbl{\f0\fnil Courier New;}}\f0\fs16 Example License\par}`),
				"banner.bmp":        picture(493, 58),
				"dialog.bmp":        picture(493, 312),
			},
		},
		"desktop": {
			Vars: map[string]string{versionVar: "2.0.0"},
			Inputs: map[string][]byte{
				"viewer.exe":  testpe.Build(testpe.Options{FileVersion: "2.0.0.0", Languages: []int{1033}}),
				"LICENSE.txt": []byte("Example license\n"),
			},
		},
	}
}

// Dir returns the examples directory of the repository: the folder next to
// the go.mod above the working directory of the test.
func Dir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "examples"
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "examples")
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "examples"
		}

		dir = parent
	}
}

// Build writes examples/<name>/msi.yaml and the inputs into a temporary
// directory, builds the package and writes it there as <name>.msi.
func Build(tb testing.TB, name string, vars map[string]string, inputs map[string][]byte) (string, *builder.Result) {
	tb.Helper()

	dir := tb.TempDir()

	src, err := os.ReadFile(filepath.Join(Dir(), name, "msi.yaml"))
	if err != nil {
		tb.Fatal(err)
	}

	files := maps.Clone(inputs)
	if files == nil {
		files = make(map[string][]byte, 1)
	}

	files["msi.yaml"] = src

	for n, data := range files {
		if err := os.WriteFile(filepath.Join(dir, n), data, 0o600); err != nil {
			tb.Fatal(err)
		}
	}

	cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{Vars: vars})
	if err != nil {
		tb.Fatal(err)
	}

	res, err := builder.Build(cfg, builder.Options{BuildTime: BuildTime})
	if err != nil {
		tb.Fatalf("%v\n%v", err, res.Diagnostics)
	}

	path := filepath.Join(dir, name+".msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		tb.Fatal(err)
	}

	return path, res
}

// CompareSnapshot compares got with the snapshot file, or rewrites the file
// when MSI_BUILDER_UPDATE_SNAPSHOTS is set. It reports the first line that
// differs and returns whether the snapshot matched.
func CompareSnapshot(tb testing.TB, file, got string) bool {
	tb.Helper()

	if os.Getenv("MSI_BUILDER_UPDATE_SNAPSHOTS") != "" {
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			tb.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(got), 0o600); err != nil {
			tb.Fatal(err)
		}

		return true
	}

	data, err := os.ReadFile(file)
	if err != nil {
		tb.Errorf("%v; run the test with MSI_BUILDER_UPDATE_SNAPSHOTS=1 to create it", err)

		return false
	}

	// A Windows checkout may have turned the line ends into CRLF.
	want := strings.ReplaceAll(string(data), "\r\n", "\n")
	if want == got {
		return true
	}

	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(wantLines), len(gotLines)) {
		var wantLine, gotLine string
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}

		if i < len(gotLines) {
			gotLine = gotLines[i]
		}

		if wantLine != gotLine {
			tb.Errorf("%s differs from line %d on; run the test with MSI_BUILDER_UPDATE_SNAPSHOTS=1 and review the diff\nwant: %q\ngot:  %q",
				file, i+1, wantLine, gotLine)

			break
		}
	}

	return false
}

// picture is a width×height PNG in one colour; the builder re-encodes it as
// BMP.
func picture(width, height int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xFF}), image.Point{}, draw.Src)

	var buf bytes.Buffer

	// Encoding an in-memory image into a buffer cannot fail.
	_ = png.Encode(&buf, img)

	return buf.Bytes()
}
