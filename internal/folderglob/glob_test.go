package folderglob_test

import (
	"testing"

	"github.com/jkroepke/msi-builder/internal/folderglob"
)

func TestFilter(t *testing.T) {
	t.Parallel()

	filter, err := folderglob.New([]string{"*.exe", "docs/**/*.md"}, []string{"logs/", "docs/old/"})
	if err != nil {
		t.Fatal(err)
	}

	for rel, want := range map[string]bool{
		"app.exe":            true,  // a pattern without / matches in any folder
		"bin/Tool.EXE":       true,  // case is ignored
		"docs/guide.md":      true,  // ** matches no folder too
		"docs/api/x.md":      true,  // and several
		"guide.md":           false, // a pattern with / is anchored
		"logs/app.exe":       false, // exclude on a folder covers its files
		"docs/old/a.md":      false, // also below the anchor
		"readme.txt":         false, // no include pattern matches
		"logs.exe":           true,  // a trailing / matches folders only
		"bin/logs/today.exe": false, // a folder name excluded at any depth
	} {
		if got := filter.Included(rel); got != want {
			t.Errorf("Included(%q) = %t, want %t", rel, got, want)
		}
	}

	if !filter.Excluded("logs", true) || filter.Excluded("logs", false) {
		t.Error("logs/ excludes the folder logs only")
	}

	all, err := folderglob.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !all.Included("any/file.txt") {
		t.Error("without include every file is added")
	}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	for pattern, ok := range map[string]bool{
		"*.pdb": true, "lib/**/*.dll": true, "logs/": true,
		"": false, `logs\*`: false, "/conf": false, "../x": false, "a//b": false, "[a": false,
	} {
		if err := folderglob.Check(pattern); (err == nil) != ok {
			t.Errorf("Check(%q) = %v, want ok %t", pattern, err, ok)
		}
	}
}
