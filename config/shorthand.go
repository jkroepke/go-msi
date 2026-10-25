package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Defaults of the shorthands in docs/design/yaml-schema.md section 3.3.
const (
	DefaultInstallDirID = "INSTALLDIR"
	DefaultFeatureID    = "MainFeature"

	maxGeneratedIDLength = 60
	hashSuffixLength     = 9 // "_" and 8 hex digits
)

// expandShorthands rewrites the short forms of a file into the explicit
// model that the builder reads: the install dir and folder paths become
// directory nodes, components defined inside features move to components,
// missing component ids are generated, a package without features gets one,
// and conditional service arguments become setProperty actions. Every step
// leaves an already expanded config unchanged, so ApplyDefaults stays
// idempotent.
func expandShorthands(cfg *Config) {
	hoistFeatureComponents(cfg)

	tree := newDirTree(cfg)
	tree.installDir()

	for _, comp := range cfg.Components {
		comp.Directory = tree.resolve(comp.Directory)
	}

	componentIDs(cfg)
	referenceFeatureComponents(cfg)
	defaultFeature(cfg)
	serviceArguments(cfg)
}

// hoistFeatureComponents appends the components defined inside features to
// cfg.Components. The feature entries are turned into references later, once
// every component has an id.
func hoistFeatureComponents(cfg *Config) {
	var walk func([]*Feature, string)

	walk = func(features []*Feature, prefix string) {
		for i, feature := range features {
			featurePath := fmt.Sprintf("%s[%d]", prefix, i)

			for j, comp := range feature.Components {
				if comp.ref || comp.origin != "" {
					continue
				}

				comp.origin = fmt.Sprintf("%s.components[%d]", featurePath, j)
				cfg.Components = append(cfg.Components, comp)
			}

			walk(feature.Features, featurePath+".features")
		}
	}

	walk(cfg.Features, "features")
}

// referenceFeatureComponents replaces each component defined inside a feature
// with a reference to it by id.
func referenceFeatureComponents(cfg *Config) {
	eachFeature(cfg.Features, func(f *Feature, _ int) {
		for j, comp := range f.Components {
			if !comp.ref {
				f.Components[j] = &Component{ID: comp.ID, ref: true}
			}
		}
	})
}

// componentIDs gives every component without an id one derived from its
// content: the key file's name, else the service name, else the directory.
func componentIDs(cfg *Config) {
	used := make(map[string]bool, len(cfg.Components))

	for _, comp := range cfg.Components {
		if comp.ID != "" {
			used[comp.ID] = true
		}
	}

	for _, comp := range cfg.Components {
		if comp.ID != "" {
			continue
		}

		base := "Component"

		switch {
		case keyFileByRef(comp, comp.KeyPath) != nil:
			base = fileName(keyFileByRef(comp, comp.KeyPath))
		case len(comp.Files) > 0:
			base = fileName(comp.Files[0])
		case len(comp.Services) > 0:
			base = comp.Services[0].Name
		case comp.Directory != "":
			base = comp.Directory
		}

		comp.ID = uniqueID(identifier(base), used)
	}
}

// defaultFeature puts every component into one feature when the file
// defines none.
func defaultFeature(cfg *Config) {
	if len(cfg.Features) > 0 || len(cfg.Components) == 0 {
		return
	}

	feature := &Feature{ID: DefaultFeatureID, Title: cfg.Product.Name, Display: "expanded"}

	if dir := findDirectory(cfg.Directories, cfg.InstallDir.ID); dir != nil && dir.Configurable {
		feature.Directory = dir.ID
	}

	for _, comp := range cfg.Components {
		feature.Components = append(feature.Components, &Component{ID: comp.ID, ref: true})
	}

	cfg.Features = []*Feature{feature}
}

// serviceArguments turns every argument with a condition into a private
// property, set by an immediate setProperty action before InstallServices
// under that condition, and references the property in the command line.
// A property that is never set formats to an empty string.
func serviceArguments(cfg *Config) {
	used := make(map[string]bool, len(cfg.CustomActions))
	for _, ca := range cfg.CustomActions {
		used[ca.ID] = true
	}

	for _, comp := range cfg.Components {
		for _, svc := range comp.Services {
			conditional := 0

			for i, arg := range svc.Arguments {
				if arg.Condition == "" {
					continue
				}

				conditional++

				action := uniqueID(identifier("SetArg_"+svc.Name+"_"+strconv.Itoa(conditional)), used)
				property := "Arg_" + strings.TrimPrefix(action, "SetArg_")

				cfg.CustomActions = append(cfg.CustomActions, &CustomAction{
					ID:          action,
					SetProperty: &SetProperty{Name: property, Value: arg.Value},
					Schedule: []Placement{{
						Sequence:  "execute",
						Before:    "InstallServices",
						Condition: arg.Condition,
					}},
				})

				svc.Arguments[i] = Argument{Value: "[" + property + "]"}
			}
		}
	}
}

// identifier turns text into a Windows Installer identifier: characters
// other than letters, digits, "_" and "." become "_", and a leading digit or
// "." gets a "_" in front.
func identifier(text string) string {
	var sb strings.Builder

	for _, char := range text {
		if !identifierChar(char) {
			char = '_'
		}

		if sb.Len() == 0 && !identifierStart(char) {
			sb.WriteByte('_')
		}

		sb.WriteRune(char)
	}

	if sb.Len() == 0 {
		return "_"
	}

	result := sb.String()

	// Leave room for the suffix of uniqueID and for prefixes.
	if len(result) > maxGeneratedIDLength {
		sum := sha256.Sum256([]byte(result))
		result = result[:maxGeneratedIDLength-hashSuffixLength] + "_" + hex.EncodeToString(sum[:4])
	}

	return result
}

func identifierStart(char rune) bool {
	return char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char == '_'
}

func identifierChar(char rune) bool {
	return identifierStart(char) || char >= '0' && char <= '9' || char == '.'
}

// uniqueID returns base, or base with a numeric suffix if base is taken, and
// marks the result as used.
func uniqueID(base string, used map[string]bool) string {
	result := base
	for n := 2; used[result]; n++ {
		result = base + "_" + strconv.Itoa(n)
	}

	used[result] = true

	return result
}

func findDirectory(dirs []*Directory, dirID string) *Directory {
	for _, dir := range dirs {
		if dir.ID == dirID {
			return dir
		}

		if found := findDirectory(dir.Children, dirID); found != nil {
			return found
		}
	}

	return nil
}

// dirTree adds directory nodes for the install dir and for folder paths.
type dirTree struct {
	cfg  *Config
	byID map[string]*Directory
}

func newDirTree(cfg *Config) *dirTree {
	tree := &dirTree{cfg: cfg, byID: make(map[string]*Directory)}

	var walk func([]*Directory)

	walk = func(dirs []*Directory) {
		for _, dir := range dirs {
			if _, dup := tree.byID[dir.ID]; !dup {
				tree.byID[dir.ID] = dir
			}

			walk(dir.Children)
		}
	}
	walk(cfg.Directories)

	return tree
}

// installDir fills in the install dir's defaults and adds it to the tree
// unless the directories define its id.
func (t *dirTree) installDir() {
	install := &t.cfg.InstallDir
	perUser := t.cfg.Package.Scope == ScopePerUser

	if install.ID == "" {
		install.ID = DefaultInstallDirID
	}

	if install.Parent == "" {
		install.Parent = defaultInstallParent(t.cfg.Package.Platform, perUser)
	}

	if install.Name == "" {
		install.Name = t.cfg.Product.Name
		if perUser {
			install.Name = "Programs/" + install.Name
		}
	}

	names := splitPath(install.Name)
	if t.byID[install.ID] != nil || !standardFolders[install.Parent] || len(names) == 0 {
		return
	}

	parent := t.top(install.Parent)
	for _, name := range names[:len(names)-1] {
		parent = t.child(parent, name, "")
	}

	t.child(parent, names[len(names)-1], install.ID).Configurable = true
}

// defaultInstallParent is Program Files for the package's bitness, or the
// user's local application data for a per-user package (ICE105).
func defaultInstallParent(platform Platform, perUser bool) string {
	switch {
	case perUser:
		return "LocalAppDataFolder"
	case platform == PlatformX86:
		return "ProgramFilesFolder"
	default:
		return "ProgramFiles64Folder"
	}
}

// resolve returns the directory id for a component's directory value: empty
// is the install dir, a known id or standard folder stays as it is, and
// anything else is a folder path. A path whose first segment is a known id or
// standard folder starts there; any other path starts in the install dir.
// "./" forces a path, for a folder name that looks like an id.
func (t *dirTree) resolve(value string) string {
	switch {
	case value == "":
		return t.cfg.InstallDir.ID
	case t.byID[value] != nil:
		return value
	case standardFolders[value]:
		t.top(value)

		return value
	}

	// A single upper-case word is meant as an id. Leave it for the
	// validator to report instead of creating a folder from a typo.
	if IsIdentifier(value) && IsPublicProperty(value) {
		return value
	}

	forced := strings.HasPrefix(value, "./") || strings.HasPrefix(value, `.\`)
	names := splitPath(value)

	parent, names := t.pathStart(names, forced)
	if parent == nil {
		// The install dir is not in the tree; the validator reports it.
		return t.cfg.InstallDir.ID
	}

	for _, name := range names {
		parent = t.child(parent, name, "")
	}

	return parent.ID
}

// pathStart returns the node a folder path starts at and the names below it.
func (t *dirTree) pathStart(names []string, forced bool) (*Directory, []string) {
	if forced || len(names) == 0 {
		return t.byID[t.cfg.InstallDir.ID], names
	}

	if dir := t.byID[names[0]]; dir != nil {
		return dir, names[1:]
	}

	if standardFolders[names[0]] {
		return t.top(names[0]), names[1:]
	}

	return t.byID[t.cfg.InstallDir.ID], names
}

// top returns the top-level node of a standard folder, adding it if needed.
func (t *dirTree) top(folder string) *Directory {
	for _, dir := range t.cfg.Directories {
		if dir.ID == folder {
			return dir
		}
	}

	dir := &Directory{ID: folder}
	t.cfg.Directories = append(t.cfg.Directories, dir)
	t.byID[folder] = dir

	return dir
}

// child returns parent's child folder called name, adding it if needed. A
// new node gets dirID, or one generated from the parent and the name.
func (t *dirTree) child(parent *Directory, name, dirID string) *Directory {
	long := longPart(name)

	for _, existing := range parent.Children {
		if strings.EqualFold(longPart(existing.Name), long) && (dirID == "" || existing.ID == dirID) {
			return existing
		}
	}

	if dirID == "" {
		used := make(map[string]bool, len(t.byID))
		for key := range t.byID {
			used[key] = true
		}

		dirID = uniqueID(identifier(parent.ID+"_"+long), used)
	}

	dir := &Directory{ID: dirID, Name: name}
	parent.Children = append(parent.Children, dir)
	t.byID[dirID] = dir

	return dir
}

// longPart returns the long name of "long" or "short|long".
func longPart(name string) string {
	if _, long, ok := strings.Cut(name, "|"); ok {
		return long
	}

	return name
}

// splitPath splits a folder path at "/" and "\", dropping "." and empty
// segments.
func splitPath(folders string) []string {
	var names []string

	for _, name := range strings.FieldsFunc(folders, func(r rune) bool { return r == '/' || r == '\\' }) {
		if name != "." {
			names = append(names, name)
		}
	}

	return names
}
