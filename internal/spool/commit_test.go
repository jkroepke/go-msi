package spool_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/spool"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		return "missing"
	}

	return string(data)
}

// TestCommit checks that Commit replaces every destination and leaves no
// backup behind.
func TestCommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.new"), "new a")
	writeFile(t, filepath.Join(dir, "b.new"), "new b")
	writeFile(t, filepath.Join(dir, "a"), "old a")

	err := spool.Commit([]spool.Move{
		{From: filepath.Join(dir, "a.new"), To: filepath.Join(dir, "a")},
		{From: filepath.Join(dir, "b.new"), To: filepath.Join(dir, "b")},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, filepath.Join(dir, "a")) + "|" + readFile(t, filepath.Join(dir, "b")); got != "new a|new b" {
		t.Errorf("contents %q", got)
	}

	if left, _ := os.ReadDir(dir); len(left) != 2 {
		t.Errorf("the folder holds %d entries, want 2", len(left))
	}
}

// TestCommitRollback checks that a failed move undoes the moves before it:
// the old files are back and the new ones at their sources.
func TestCommitRollback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.new"), "new a")
	writeFile(t, filepath.Join(dir, "b.new"), "new b")
	writeFile(t, filepath.Join(dir, "a"), "old a")

	err := spool.Commit([]spool.Move{
		{From: filepath.Join(dir, "a.new"), To: filepath.Join(dir, "a")},
		{From: filepath.Join(dir, "b.new"), To: filepath.Join(dir, "b")},
		{From: filepath.Join(dir, "missing"), To: filepath.Join(dir, "c")},
	})
	if err == nil {
		t.Fatal("the move of a missing file succeeded")
	}

	for path, want := range map[string]string{
		"a": "old a", "b": "missing", "c": "missing", "a.new": "new a", "b.new": "new b",
	} {
		if got := readFile(t, filepath.Join(dir, path)); got != want {
			t.Errorf("%s holds %q, want %q", path, got, want)
		}
	}

	if left, _ := os.ReadDir(dir); len(left) != 3 {
		t.Errorf("the folder holds %d entries, want 3", len(left))
	}
}
