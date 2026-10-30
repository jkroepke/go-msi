package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/jkroepke/msi-builder/internal/folderglob"
)

// TreeOptions controls AddTree.
type TreeOptions struct {
	// Feature is the id of an existing feature that the generated components
	// join. Empty means the only feature of the config, or, while it has
	// none, the default feature that ApplyDefaults adds later.
	Feature string

	// Prefix starts every generated id. The ids also cover the target
	// directory and root, so trees with a different target directory, root
	// or prefix never share a component or file id; a prefix only keeps the
	// generated ids apart from your own. It is at most
	// MaxTreePrefixLength characters. The default is "tree".
	Prefix string
}

// MaxTreePrefixLength is the longest TreeOptions.Prefix: a generated id adds
// an underscore, a kind letter and 20 hex digits to it.
const MaxTreePrefixLength = MaxIdentifierLength - 22

// AddTree adds every regular file under the folder root in fsys below the
// existing directory dirID. The directory of the installDir shorthand exists
// after ApplyDefaults, so call it first for such a config. With an empty
// TreeOptions.Feature the components join the only feature, or the default
// feature like every other component when there is none yet. Each subdirectory that holds a file, directly or
// further down, becomes a child Directory, and each file becomes its own
// Component with that one file, so every file has its own key path and GUID.
// A subfolder that already exists below dirID under the same long name, for
// example from an earlier AddTree, is reused. The generated ids are derived
// from Prefix, dirID, root and the slash path below root, so the same calls
// in the same order always give the same ids and GUIDs.
//
// Root must be a non-empty path of a directory. A symlink or other
// non-regular file below root is an error, and so is a name below root that
// the config does not accept as a long file name; root itself must not
// contain a colon. On an error the config is
// unchanged. Files are read from fsys when the package is built. AddTree
// returns the new components in walk order, for example to set permissions
// or allowUnversionedExecutable on some of them.
func (c *Config) AddTree(fsys fs.FS, root, dirID string, opts TreeOptions) ([]*Component, error) {
	if root == "" {
		return nil, errors.New("add tree: root is empty; use \".\" for the whole file system")
	}

	root = path.Clean(root)

	if opts.Prefix == "" {
		opts.Prefix = "tree"
	}

	feature, err := c.treeFeature(opts.Feature)
	if err != nil {
		return nil, err
	}

	added, err := c.walkTree(fsys, root, root, dirID, opts.Prefix, nil)
	if err != nil {
		return nil, err
	}

	c.Components = append(c.Components, added...)

	if feature != nil {
		for _, comp := range added {
			feature.Components = append(feature.Components, ComponentRef(comp.ID))
		}
	}

	return added, nil
}

// walkTree returns a component for every file under root that filter, when
// set, keeps, and adds the folders they need below dirID. The ids are
// derived from prefix, dirID, idRoot and the path below root; a folder
// source takes idRoot from its source as written instead of the fs.FS root.
// The components are not added to the config.
func (c *Config) walkTree(fsys fs.FS, root, idRoot, dirID, prefix string, filter *folderglob.Filter) ([]*Component, error) {
	parent, err := c.treeTarget(fsys, root, dirID, prefix)
	if err != nil {
		return nil, err
	}

	walk := &tree{root: root, prefix: prefix, base: dirID + "\x1f" + idRoot, dirs: map[string]*Directory{".": parent}, filter: filter}

	err = fs.WalkDir(fsys, root, walk.visit)
	if err != nil {
		return nil, fmt.Errorf("add tree: %w", err)
	}

	if len(walk.files) == 0 {
		if filter != nil {
			return nil, errors.New("add tree: no file under " + idRoot + " matches the include and exclude patterns")
		}

		return nil, errors.New("add tree: no files under " + idRoot)
	}

	added := make([]*Component, 0, len(walk.files))

	for _, rel := range walk.files {
		added = append(added, &Component{
			ID:        walk.id("c", rel),
			Directory: walk.directory(path.Dir(rel)).ID,
			Files: []*File{{
				Source: path.Join(root, rel),
				ID:     walk.id("f", rel),
				Name:   path.Base(rel),
				FS:     fsys,
			}},
		})
	}

	return added, nil
}

// treeTarget checks the arguments of AddTree and returns the directory
// the tree goes below.
func (c *Config) treeTarget(fsys fs.FS, root, dirID, prefix string) (*Directory, error) {
	if !IsIdentifier(prefix) || len(prefix) > MaxTreePrefixLength {
		return nil, fmt.Errorf("add tree: prefix %q is not an identifier of at most %d characters", prefix, MaxTreePrefixLength)
	}

	if c.Package.LongFileNames != nil && !*c.Package.LongFileNames {
		return nil, errors.New("add tree: package.longFileNames is false, which needs 8.3 file names, but files of a folder keep their own names")
	}

	if err := checkTreeRoot(fsys, root); err != nil {
		return nil, err
	}

	parent := findDirectory(c.Directories, dirID)
	if parent == nil {
		return nil, fmt.Errorf("add tree: directory %s is not defined; with installDir, call ApplyDefaults first", dirID)
	}

	return parent, nil
}

// treeFeature returns the feature that AddTree's components join, or nil
// while the config has none: ApplyDefaults later puts every component into
// the default feature. With one feature, an empty name means it.
func (c *Config) treeFeature(name string) (*Feature, error) {
	if name == "" {
		var all []*Feature

		WalkFeatures(c.Features, func(f, _ *Feature) { all = append(all, f) })

		switch len(all) {
		case 0:
			return nil, nil //nolint:nilnil // no feature yet is not an error
		case 1:
			name = all[0].ID
		default:
			return nil, fmt.Errorf("add tree: the config has %d features; set TreeOptions.Feature", len(all))
		}
	}

	feature := findFeature(c.Features, name)
	if feature == nil {
		return nil, fmt.Errorf("add tree: feature %q is not defined", name)
	}

	return feature, nil
}

// checkTreeRoot checks that root names a directory in fsys that the build
// can read back: slashPath would turn a backslash into a separator. A root
// that is a symlink to a directory is followed.
func checkTreeRoot(fsys fs.FS, root string) error {
	if !fs.ValidPath(root) || strings.Contains(root, `\`) {
		return fmt.Errorf("add tree: %q is not a valid fs path", root)
	}

	// A root such as C: would make its sources absolute Windows paths, which
	// are read from the disk instead of fsys. Other root names are only read
	// from fsys and never installed, so they need not be Windows names.
	if strings.Contains(root, ":") {
		return fmt.Errorf("add tree: root %q contains a colon", root)
	}

	info, err := fs.Stat(fsys, root)
	if err != nil {
		return fmt.Errorf("add tree: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("add tree: %s is not a directory", root)
	}

	return nil
}

// checkTreeName rejects a slash path with a segment that is not a valid long
// file name.
func checkTreeName(rel string) error {
	for name := range strings.SplitSeq(rel, "/") {
		if !validLongName(name) {
			return fmt.Errorf("%q is not a valid Windows file name", name)
		}
	}

	return nil
}

// tree collects the files of one AddTree call and creates their directories.
type tree struct {
	dirs   map[string]*Directory
	filter *folderglob.Filter
	root   string
	prefix string
	base   string
	files  []string
}

// visit is the fs.WalkDirFunc of AddTree: it checks each file and records its
// slash path below root, without changing the config.
func (t *tree) visit(filePath string, entry fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	rel := filePath
	if t.root != "." {
		rel = strings.TrimPrefix(filePath, t.root+"/")
	}

	// The patterns run first, so that they can leave out a special file
	// or a name Windows does not allow.
	if skip := t.skip(filePath, rel, entry); skip != nil || entry.IsDir() {
		if errors.Is(skip, errLeaveOut) {
			return nil
		}

		return skip
	}

	if !entry.Type().IsRegular() {
		return fmt.Errorf("%s is not a regular file", filePath)
	}

	if err := checkTreeName(rel); err != nil {
		return fmt.Errorf("%s: %w", filePath, err)
	}

	t.files = append(t.files, rel)

	return nil
}

// errLeaveOut tells visit that the patterns leave a file out.
var errLeaveOut = errors.New("left out")

// skip applies the include and exclude patterns to an entry: fs.SkipDir for an excluded
// folder, errLeaveOut for a file that is not included, else nil.
func (t *tree) skip(filePath, rel string, entry fs.DirEntry) error {
	if t.filter == nil || filePath == t.root {
		return nil
	}

	if entry.IsDir() {
		if t.filter.Excluded(rel, true) {
			return fs.SkipDir
		}

		return nil
	}

	if !t.filter.Included(rel) {
		return errLeaveOut
	}

	return nil
}

// directory returns the Directory for the slash path rel, reusing a child with
// the same long name or creating it and its parents on first use.
func (t *tree) directory(rel string) *Directory {
	if d, ok := t.dirs[rel]; ok {
		return d
	}

	parent := t.directory(path.Dir(rel))
	name := path.Base(rel)

	for _, child := range parent.Children {
		if strings.EqualFold(child.Name, name) {
			t.dirs[rel] = child

			return child
		}
	}

	d := &Directory{ID: t.id("d", rel), Name: name}
	parent.Children = append(parent.Children, d)
	t.dirs[rel] = d

	return d
}

// id derives an identifier from the prefix, a kind letter, the target
// directory and a path.
func (t *tree) id(kind, rel string) string {
	sum := sha256.Sum256([]byte(kind + "\x1f" + t.base + "\x1f" + rel))

	return t.prefix + "_" + kind + strings.ToUpper(hex.EncodeToString(sum[:10]))
}

func findFeature(features []*Feature, id string) *Feature {
	var found *Feature

	WalkFeatures(features, func(f, _ *Feature) {
		if found == nil && f.ID == id {
			found = f
		}
	})

	return found
}
