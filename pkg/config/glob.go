package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// globPattern is one include or exclude pattern of a folder source, matched
// with doublestar. A pattern without a slash matches a name in any folder,
// like a .gitignore line; with a slash it is anchored at the source
// folder, and a "**" folder matches any number of folders. A trailing slash
// makes it match folders only. Matching ignores case, as Windows does.
type globPattern struct {
	pattern string
	dirOnly bool
}

// parseGlob checks a pattern and returns it in lower case, anchored.
func parseGlob(pattern string) (globPattern, error) {
	var glob globPattern

	switch {
	case pattern == "":
		return glob, errors.New("empty pattern")
	case strings.Contains(pattern, `\`):
		return glob, errors.New(`use "/" to separate folders; "\" is not allowed`)
	case strings.HasPrefix(pattern, "/"):
		return glob, errors.New("patterns are relative to source; remove the leading /")
	}

	glob.dirOnly = strings.HasSuffix(pattern, "/")
	body := strings.ToLower(strings.TrimSuffix(pattern, "/"))

	for seg := range strings.SplitSeq(body, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return glob, errors.New(`a pattern cannot contain an empty, "." or ".." folder`)
		}
	}

	if !doublestar.ValidatePattern(body) {
		return glob, errors.New("bad pattern syntax")
	}

	glob.pattern = body
	if !strings.Contains(body, "/") {
		glob.pattern = "**/" + body
	}

	return glob, nil
}

// matches reports whether the pattern matches the slash path rel below the
// folder source, a folder when dir is set.
func (g globPattern) matches(rel string, dir bool) bool {
	if g.dirOnly && !dir {
		return false
	}

	return doublestar.MatchUnvalidated(g.pattern, strings.ToLower(rel))
}

// folderFilter decides which entries below a folder source are added.
type folderFilter struct {
	include []globPattern
	exclude []globPattern
}

// newFolderFilter parses the patterns of a folder source.
func newFolderFilter(include, exclude []string) (*folderFilter, error) {
	filter := &folderFilter{}

	for _, list := range []struct {
		into     *[]globPattern
		name     string
		patterns []string
	}{{&filter.include, "include", include}, {&filter.exclude, "exclude", exclude}} {
		for i, pattern := range list.patterns {
			glob, err := parseGlob(pattern)
			if err != nil {
				return nil, fmt.Errorf("%s[%d] %q: %w", list.name, i, pattern, err)
			}

			*list.into = append(*list.into, glob)
		}
	}

	return filter, nil
}

// excluded reports whether an exclude pattern matches rel or one of its
// folders, so that the walk can skip a whole folder.
func (f *folderFilter) excluded(rel string, dir bool) bool {
	return anyMatch(f.exclude, rel, dir)
}

// included reports whether the file rel is added: no exclude pattern
// matches it or a folder of it, and an include pattern does, or there is
// none.
func (f *folderFilter) included(rel string) bool {
	if anyMatch(f.exclude, rel, false) {
		return false
	}

	return len(f.include) == 0 || anyMatch(f.include, rel, false)
}

// anyMatch reports whether a pattern matches rel or one of its folders.
func anyMatch(patterns []globPattern, rel string, dir bool) bool {
	for i := 0; i <= len(rel); i++ {
		if i < len(rel) && rel[i] != '/' {
			continue
		}

		isDir := dir || i < len(rel)
		for _, glob := range patterns {
			if glob.matches(rel[:i], isDir) {
				return true
			}
		}
	}

	return false
}
