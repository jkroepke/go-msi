package builder_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/memtest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/cab"
	"github.com/jkroepke/msi-builder/pkg/cfb"
	"github.com/jkroepke/msi-builder/pkg/config"
	"github.com/jkroepke/msi-builder/pkg/msidb"
)

// TestOutputFile checks that a build written to a path gives the same bytes
// as one kept in memory, and that a failed build leaves no file behind.
func TestOutputFile(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	inMemory := mustBuild(t, example(t, "minimal", nil, map[string][]byte{"hello.exe": exe}))

	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	out := filepath.Join(cfg.BaseDir, "out", "hello.msi")

	if err := os.Mkdir(filepath.Dir(out), 0o750); err != nil {
		t.Fatal(err)
	}

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, Output: out})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	if res.Package != nil {
		t.Error("a build to a file also returned the bytes")
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, inMemory.Package) || res.Size != int64(len(data)) || res.PackageCode != inMemory.PackageCode {
		t.Errorf("file build differs from the in-memory build: %d vs %d bytes, codes %s / %s", len(data), len(inMemory.Package), res.PackageCode, inMemory.PackageCode)
	}

	left, _ := os.ReadDir(filepath.Dir(out))
	if len(left) != 1 {
		t.Errorf("the output directory holds %d entries, want only the package", len(left))
	}

	// A build that fails while writing leaves no file behind and keeps an
	// existing package: the cabinet name passes validation and is refused
	// by the stream writer.
	bad := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	bad.Package.Cabinet = "Ä.cab"
	badOut := filepath.Join(bad.BaseDir, "bad.msi")

	if _, err := builder.Build(bad, builder.Options{BuildTime: buildTime, Output: badOut}); err == nil || !strings.Contains(err.Error(), "cased") {
		t.Fatalf("the cased cabinet name gave %v", err)
	}

	if _, err := os.Stat(badOut); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the failed build left %s behind: %v", badOut, err)
	}

	if _, err := builder.Build(bad, builder.Options{BuildTime: buildTime, Output: out}); err == nil {
		t.Fatal("the cased cabinet name was accepted")
	}

	if kept, err := os.ReadFile(out); err != nil || !bytes.Equal(kept, data) {
		t.Errorf("the failed build replaced the existing package: %v", err)
	}

	left, _ = os.ReadDir(bad.BaseDir)
	for _, ent := range left {
		if strings.HasPrefix(ent.Name(), ".msi-builder-") {
			t.Errorf("the failed build left the spool %s behind", ent.Name())
		}
	}

	if _, err := builder.Build(bad, builder.Options{BuildTime: buildTime, Output: filepath.Join(bad.BaseDir, "missing", "x.msi")}); err == nil {
		t.Error("an output in a missing directory was accepted")
	}

	// A symlink at the output is written through, and the target keeps
	// its mode.
	targetDir := filepath.Join(cfg.BaseDir, "target")
	if err := os.Mkdir(targetDir, 0o750); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(targetDir, "real.msi")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(cfg.BaseDir, "link.msi")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, Output: link}); err != nil {
		t.Fatalf("build through a symlink: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink was replaced: %v, %v", info, err)
	}

	if through, err := os.ReadFile(target); err != nil || !bytes.Equal(through, data) {
		t.Errorf("the symlink target does not hold the package: %v", err)
	}

	// Windows reports only the read-only attribute as permission bits.
	if runtime.GOOS == "windows" {
		return
	}

	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the target's mode changed: %v, %v", info.Mode(), err)
	}
}

// TestLargePayloadBoundedMemory builds a package with a payload far larger
// than the memory the build may use.
func TestLargePayloadBoundedMemory(t *testing.T) { //nolint:paralleltest // measures the heap of the whole process
	if testing.Short() {
		t.Skip("writes 32 MB")
	}

	const (
		size  = 32 << 20
		limit = 16 << 20
	)

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})

	big, err := os.Create(filepath.Join(cfg.BaseDir, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(big, memtest.Pattern(size)); err != nil {
		t.Fatal(err)
	}

	if err := big.Close(); err != nil {
		t.Fatal(err)
	}

	cfg.Components[0].Files = append(cfg.Components[0].Files, &config.File{Source: "big.bin"})

	// The default LZX takes too long for 32 MB under the race detector; CI
	// runs this test without -race as well.
	if raceEnabled {
		cfg.Package.Compression = "mszip"
	}

	out := filepath.Join(cfg.BaseDir, "big.msi")

	var res *builder.Result

	peak := memtest.PeakHeap(limit, func() {
		res, err = builder.Build(cfg, builder.Options{BuildTime: buildTime, Output: out})
	})

	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	t.Logf("package of %d bytes; live heap before %d, peak %d, allocated peak %d", res.Size, peak.Before, peak.Max, peak.Allocated)

	if peak.Used() > limit {
		t.Errorf("the build used %d bytes of live heap, want at most %d", peak.Used(), limit)
	}

	// The package holds the file: extract it through the lazy readers and
	// compare digests.
	pkg, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}

	defer pkg.Close()

	file, err := cfb.Open(pkg, res.Size)
	if err != nil {
		t.Fatal(err)
	}

	database, err := msidb.Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	if len(database.Sources) != 1 || database.Sources[0].Name != cfg.Package.Cabinet {
		t.Fatalf("sources %+v", database.Sources)
	}

	want := sha256.New()
	_, _ = io.Copy(want, memtest.Pattern(size))

	encodedCab, err := msidb.EncodeStreamName(cfg.Package.Cabinet, false)
	if err != nil {
		t.Fatal(err)
	}

	cabinet, err := file.Reader(encodedCab)
	if err != nil {
		t.Fatal(err)
	}

	var walked bool

	if err := cab.Walk(cabinet, cabinet.Size(), func(entry cab.File, data io.Reader) error {
		if entry.Name != "big.bin" {
			return nil // hello.exe, checked by Build's own verification
		}

		walked = true
		got := sha256.New()

		if _, err := io.Copy(got, data); err != nil {
			return err
		}

		if !bytes.Equal(got.Sum(nil), want.Sum(nil)) {
			t.Errorf("file %q: digest differs from the source", entry.Name)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if !walked {
		t.Error("the cabinet has no big.bin")
	}

	var found bool

	for _, row := range rows(t, database, "File") {
		if row[2] == "big.bin" && row[3] == size {
			found = true
		}
	}

	if !found {
		t.Errorf("File table has no big.bin of %d bytes: %v", size, rows(t, database, "File"))
	}
}
