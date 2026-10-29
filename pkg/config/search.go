package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SearchFoundProperty returns the public property that AppSearch fills for
// the search at index of a searched property, before an action copies it
// over: <NAME>_FOUND for the first, <NAME>_FOUND2 for the second, and so on.
func SearchFoundProperty(name string, index int) string {
	suffix := "_FOUND"
	if index > 0 {
		suffix += strconv.Itoa(index + 1)
	}

	if len(name)+len(suffix) <= MaxIdentifierLength {
		return name + suffix
	}

	sum := sha256.Sum256([]byte(name))

	return strings.TrimPrefix(suffix, "_") + "_" + strings.ToUpper(hex.EncodeToString(sum[:16]))
}

// Search kinds.
const (
	searchKindRegistry  = "registry"
	searchKindDirectory = "directory"
	searchKindFile      = "file"
	searchKindComponent = "component"
)

// Kind returns the one field that is set, or "" when none or several are.
func (s *Search) Kind() string {
	var kinds []string

	if s.Registry != nil {
		kinds = append(kinds, searchKindRegistry)
	}

	if s.Directory != nil {
		kinds = append(kinds, searchKindDirectory)
	}

	if s.File != nil {
		kinds = append(kinds, searchKindFile)
	}

	if s.Component != nil {
		kinds = append(kinds, searchKindComponent)
	}

	if len(kinds) != 1 {
		return ""
	}

	return kinds[0]
}

// maxSearchNesting bounds how many under searches can be stacked.
const maxSearchNesting = 8

// Limits of the Signature and DrLocator columns.
const (
	maxSearchDepth = 32767
	maxSearchSize  = 2147483647
)

// The years a Signature row can hold: MinDate and MaxDate are positive
// 32-bit numbers with the year offset from 1980 in the top seven bits, and
// the sign bit is the highest of them.
const (
	minSearchYear = 1980
	maxSearchYear = 2043
)

// searchFileName rejects the characters a file name cannot hold.
var searchFileName = regexp.MustCompile(`^[^\\/:*?"<>|]+$`)

// IsFullSearchPath reports whether a DrLocator path is a full path: one
// that starts with a drive, a share or a property in brackets.
func IsFullSearchPath(p string) bool {
	return strings.HasPrefix(p, "[") || strings.HasPrefix(p, `\\`) ||
		len(p) >= 3 && p[1] == ':' && p[2] == '\\' && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z')
}

// SearchDate converts a minDate or maxDate (YYYY-MM-DD) to the packed
// MS-DOS date and time of a Signature row (time 00:00:00).
func SearchDate(text string) (int, error) {
	date, err := time.Parse(time.DateOnly, text)
	if err != nil {
		return 0, fmt.Errorf("%q is not a date (YYYY-MM-DD)", text)
	}

	if date.Year() < minSearchYear || date.Year() > maxSearchYear {
		return 0, fmt.Errorf("%s is outside 1980-01-01 to 2043-12-31, the dates a Signature row can hold", text)
	}

	return ((date.Year()-1980)*512 + int(date.Month())*32 + date.Day()) << 16, nil
}

// SearchVersion checks a minVersion or maxVersion: one to four numbers of
// up to 65535.
func SearchVersion(text string) error {
	parts := strings.Split(text, ".")
	if len(parts) > 4 {
		return fmt.Errorf("version %q has more than four fields", text)
	}

	for i, p := range parts {
		num, err := strconv.Atoi(p)
		if err != nil || strings.Trim(p, "0123456789") != "" || num > 0xFFFF {
			return fmt.Errorf("version %q: field %d is not a number up to 65535", text, i+1)
		}
	}

	return nil
}

// SearchMinVersion returns the Signature.MinVersion that finds files of
// at least version text in any language. The engine finds a file whose
// version equals MinVersion only when Signature.Languages lists the file's
// languages (SEARCH-05), so without languages the bound is lowered by one
// in the last field: 1.2.3.4 becomes 1.2.3.3, 1.2 becomes 1.1.65535.65535.
// 0.0.0.0 stays: it cannot go lower, and like every MinVersion it still
// rules out files without a version (SEARCH-05).
func SearchMinVersion(text string, languages []int) string {
	if len(languages) > 0 {
		return text
	}

	parts := strings.Split(text, ".")

	var value uint64

	for i := range 4 {
		field := 0
		if i < len(parts) {
			field, _ = strconv.Atoi(parts[i])
		}

		value = value<<16 | uint64(field)
	}

	if value > 0 {
		value--
	}

	return fmt.Sprintf("%d.%d.%d.%d", value>>48, value>>32&0xFFFF, value>>16&0xFFFF, value&0xFFFF)
}

// propertySearches checks the searches of a property.
func (v *validator) propertySearches(cfg *Config, propPath, name string, searches Searches) {
	searchPath := propPath + ".search"

	v.searchedProperty(cfg, propPath, name)

	if len(searches) == 0 {
		v.errorf(searchPath, "at least one search is required")
	}

	for i, search := range searches {
		itemPath := searchPath
		if len(searches) > 1 {
			itemPath = fmt.Sprintf("%s[%d]", searchPath, i)
		}

		if found := SearchFoundProperty(name, i); cfg.Properties[found] != nil {
			v.errorf(itemPath, "%s holds the search result; it cannot also be defined in properties", found)
		}

		if search == nil {
			v.errorf(itemPath, "a search must not be empty")

			continue
		}

		v.search(cfg, itemPath, search, 0)
	}
}

// searchedProperty checks that a property can be searched.
func (v *validator) searchedProperty(cfg *Config, propPath, name string) {
	searchPath := propPath + ".search"

	if !IsPublicProperty(name) {
		v.errorf(searchPath, "only a public (upper-case) property can be searched, so that a command-line value and "+
			"the result reach the execute sequence")
	}

	if cfg.InstallDir.Remember && name == cfg.InstallDir.ID {
		v.errorf(searchPath, "installDir.remember already searches %s", name)
	}

	if prop := cfg.Properties[name]; prop.Secure != nil && !*prop.Secure {
		v.errorf(propPath+".secure", "a searched property must be secure: after a UI sequence the execute sequence "+
			"does not search again and only receives secure properties")
	}
}

// search checks one search. A nested search (depth > 0) is the folder an
// under search looks in, so it must find a folder.
func (v *validator) search(cfg *Config, searchPath string, search *Search, depth int) {
	switch search.Kind() {
	case searchKindRegistry:
		v.registrySearch(cfg, searchPath+".registry", search.Registry, depth > 0)
	case searchKindDirectory:
		v.directorySearch(cfg, searchPath+".directory", search.Directory, depth)
	case searchKindFile:
		if depth > 0 {
			v.errorf(searchPath+".file", "under must find a folder: a registry search of type directory, "+
				"a directory or a component search")

			return
		}

		v.fileSearch(cfg, searchPath+".file", search.File)
	case searchKindComponent:
		v.componentSearch(searchPath+".component", search.Component)
	default:
		v.errorf(searchPath, "exactly one of registry, directory, file or component is required")
	}
}

// registrySearch checks the registry value a search reads. Under another
// search the value must name a folder.
func (v *validator) registrySearch(cfg *Config, regPath string, reg *RegistrySearch, folder bool) {
	// RegLocator has no HKMU root.
	v.oneOf(regPath+".root", reg.Root, "HKLM", "HKCU", "HKCR", "HKU")
	v.required(regPath+".key", reg.Key)

	if strings.HasPrefix(reg.Key, `\`) || strings.HasSuffix(reg.Key, `\`) {
		v.errorf(regPath+".key", "no leading or trailing backslash")
	}

	if reg.Bitness != "" {
		v.oneOf(regPath+".bitness", reg.Bitness, "32", "64")
	}

	if reg.Bitness == "64" && !cfg.Package.Platform.Is64Bit() {
		v.errorf(regPath+".bitness", "a 32-bit package cannot read the 64-bit registry view")
	}

	switch {
	case reg.Type == "":
	case folder && reg.Type != SearchDirectory:
		v.errorf(regPath+".type", "under must find a folder: leave type out or set it to directory")
	default:
		v.oneOf(regPath+".type", reg.Type, SearchValue, SearchDirectory)
	}
}

// searchFolder checks the folder of a directory or file search: a full
// path, or an under search and a path relative to its folder.
func (v *validator) searchFolder(cfg *Config, field, folder string, under *Search, depth int) {
	switch {
	case under == nil && folder == "":
		v.errorf(field+".path", "required: a full path, or under and a relative path")
	case under == nil && !IsFullSearchPath(folder):
		v.errorf(field+".path", "%q is not a full path (a drive, a share or a property in brackets); "+
			"use under to search below another search's folder", folder)
	case under != nil && IsFullSearchPath(folder):
		v.errorf(field+".path", "with under the path is relative to the folder under finds")
	}

	if strings.Contains(folder, "|") {
		v.errorf(field+".path", "no short|long names; the long name is enough")
	}

	if under != nil {
		if depth+1 >= maxSearchNesting {
			v.errorf(field+".under", "at most %d searches in one chain", maxSearchNesting)

			return
		}

		v.search(cfg, field+".under", under, depth+1)
	}

	v.searchPathProperties(cfg, field+".path", folder)
}

// searchPathProperties warns about folders of this package in a search
// path: AppSearch runs before CostFinalize resolves them.
func (v *validator) searchPathProperties(cfg *Config, field, folder string) {
	dirs := map[string]bool{cfg.InstallDir.ID: true}

	var walk func([]*Directory)

	walk = func(list []*Directory) {
		for _, dir := range list {
			dirs[dir.ID] = true
			walk(dir.Children)
		}
	}
	walk(cfg.Directories)

	for _, match := range bracketProperty.FindAllStringSubmatch(folder, -1) {
		if dirs[match[1]] && !IsStandardFolder(match[1]) {
			v.warnf(field, "[%s] is a folder of this package; AppSearch runs before CostFinalize resolves it, "+
				"so the path is resolved without it", match[1])
		}
	}
}

// bracketProperty matches [NAME] references in a search path.
var bracketProperty = regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_.]*)\]`)

// directorySearch checks a folder search.
func (v *validator) directorySearch(cfg *Config, field string, dir *DirectorySearch, depth int) {
	v.searchFolder(cfg, field, dir.Path, dir.Under, depth)
	v.searchDepth(field, dir.Depth)

	if dir.Depth > 0 && dir.Path == "" {
		v.errorf(field+".depth", "needs a path: depth says how deep below the path's parent its last folder may lie")
	}
}

// searchDepth checks the Depth of a DrLocator row.
func (v *validator) searchDepth(field string, depth int) {
	if depth < 0 || depth > maxSearchDepth {
		v.errorf(field+".depth", "must be 0 to %d", maxSearchDepth)
	}
}

// systemFolders are where Windows keeps its own files.
var systemFolders = regexp.MustCompile(`(?i)^\[(SystemFolder|System64Folder|System16Folder|WindowsFolder)\]|^[a-z]:\\windows(\\|$)`)

// fileSearch checks a file search and its bounds.
func (v *validator) fileSearch(cfg *Config, field string, file *FileSearch) {
	if v.required(field+".name", file.Name) && !searchFileName.MatchString(file.Name) {
		v.errorf(field+".name", "%q is not a file name: no folder and none of \\ / : * ? \" < > |", file.Name)
	}

	v.searchFolder(cfg, field, file.Path, file.Under, 0)
	v.searchDepth(field, file.Depth)
	v.fileSearchBounds(field, file)
}

// fileSearchBounds checks the version, size, date and language bounds of a
// file search.
func (v *validator) fileSearchBounds(field string, file *FileSearch) { //nolint:cyclop // a flat list of independent checks
	if (file.MinVersion != "" || file.MaxVersion != "") && file.Under == nil && systemFolders.MatchString(file.Path) {
		v.warnf(field, "the versions of Windows' own files are unreliable to a search: the engine reports shimmed "+
			"versions on Windows 10 and later (reported), and SEARCH-05 found kernel32.dll with no version bound; "+
			"test VersionNT or WindowsBuild in a condition instead")
	}

	if len(file.Languages) > 0 && file.MinVersion == "" {
		v.warnf(field+".languages", "has no effect without minVersion: the engine compares languages only "+
			"when the file's version equals minVersion")
	}

	for key, text := range map[string]string{"minVersion": file.MinVersion, "maxVersion": file.MaxVersion} {
		if text != "" {
			if err := SearchVersion(text); err != nil {
				v.errorf(field+"."+key, "%v", err)
			}
		}
	}

	for key, size := range map[string]*int64{"minSize": file.MinSize, "maxSize": file.MaxSize} {
		if size != nil && (*size < 0 || *size > maxSearchSize) {
			v.errorf(field+"."+key, "must be 0 to %d", maxSearchSize)
		}
	}

	if file.MinSize != nil && file.MaxSize != nil && *file.MinSize > *file.MaxSize {
		v.errorf(field+".maxSize", "is less than minSize")
	}

	for key, text := range map[string]string{"minDate": file.MinDate, "maxDate": file.MaxDate} {
		if text != "" {
			if _, err := SearchDate(text); err != nil {
				v.errorf(field+"."+key, "%v", err)
			}
		}
	}

	if file.MinDate != "" && file.MaxDate != "" && file.MaxDate < file.MinDate {
		v.errorf(field+".maxDate", "is before minDate")
	}

	for i, lang := range file.Languages {
		if lang < 0 || lang > 0xFFFF {
			v.errorf(fmt.Sprintf("%s.languages[%d]", field, i), "%d is not a language ID", lang)
		}
	}
}

// componentSearch checks a component search.
func (v *validator) componentSearch(field string, comp *ComponentSearch) {
	if v.required(field+".guid", comp.GUID) {
		v.guid(field+".guid", comp.GUID)
	}
}
