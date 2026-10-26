package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// TreeOptions controls AddTree.
type TreeOptions struct {
	// Feature is the id of an existing feature that the generated components
	// join. It is required.
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
// existing directory dirID. Directories and features that only the
// shorthands (installDir, the default feature) create exist after
// ApplyDefaults, so call it first for such a config. Each subdirectory that holds a file, directly or
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
	if opts.Prefix == "" {
		opts.Prefix = "tree"
	}

	if root == "" {
		return nil, errors.New("add tree: root is empty; use \".\" for the whole file system")
	}

	root = path.Clean(root)

	parent, feature, err := c.treeTarget(fsys, root, dirID, opts)
	if err != nil {
		return nil, err
	}

	harvest := &tree{root: root, prefix: opts.Prefix, base: dirID + "\x1f" + root, dirs: map[string]*Directory{".": parent}}

	err = fs.WalkDir(fsys, root, harvest.visit)
	if err != nil {
		return nil, fmt.Errorf("add tree: %w", err)
	}

	if len(harvest.files) == 0 {
		return nil, errors.New("add tree: no files under " + root)
	}

	added := make([]*Component, 0, len(harvest.files))

	for _, rel := range harvest.files {
		added = append(added, &Component{
			ID:        harvest.id("c", rel),
			Directory: harvest.directory(path.Dir(rel)).ID,
			Files: []*File{{
				Source: path.Join(root, rel),
				ID:     harvest.id("f", rel),
				Name:   path.Base(rel),
				FS:     fsys,
			}},
		})
	}

	c.Components = append(c.Components, added...)

	for _, comp := range added {
		feature.Components = append(feature.Components, ComponentRef(comp.ID))
	}

	return added, nil
}

// treeTarget checks the arguments of AddTree and returns the directory and
// the feature that the tree joins.
func (c *Config) treeTarget(fsys fs.FS, root, dirID string, opts TreeOptions) (*Directory, *Feature, error) {
	if !IsIdentifier(opts.Prefix) || len(opts.Prefix) > MaxTreePrefixLength {
		return nil, nil, fmt.Errorf("add tree: prefix %q is not an identifier of at most %d characters", opts.Prefix, MaxTreePrefixLength)
	}

	if c.Package.LongFileNames != nil && !*c.Package.LongFileNames {
		return nil, nil, errors.New("add tree: package.longFileNames is false, but harvested files have no short names")
	}

	if err := checkTreeRoot(fsys, root); err != nil {
		return nil, nil, err
	}

	parent := findDirectory(c.Directories, dirID)
	if parent == nil {
		return nil, nil, fmt.Errorf("add tree: directory %s is not defined; with installDir, call ApplyDefaults first", dirID)
	}

	feature := findFeature(c.Features, opts.Feature)
	if feature == nil {
		return nil, nil, fmt.Errorf("add tree: feature %q is not defined; for the default feature, call ApplyDefaults first", opts.Feature)
	}

	return parent, feature, nil
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

	if entry.IsDir() {
		return nil
	}

	if !entry.Type().IsRegular() {
		return fmt.Errorf("%s is not a regular file", filePath)
	}

	rel := filePath
	if t.root != "." {
		rel = strings.TrimPrefix(filePath, t.root+"/")
	}

	if err := checkTreeName(rel); err != nil {
		return fmt.Errorf("%s: %w", filePath, err)
	}

	t.files = append(t.files, rel)

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
		long := child.Name
		if _, l, ok := strings.Cut(long, "|"); ok {
			long = l
		}

		if strings.EqualFold(long, name) {
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
