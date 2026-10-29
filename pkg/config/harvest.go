package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// HarvestFiles adds the files of every harvest entry that it has not added
// yet, with AddTree's rules: a component per file, folders below the
// entry's directory, and ids derived from the directory, the source as
// written and the path below it. A relative source is read from Config.FS
// when it is set, else from the folder of the YAML file (BaseDir); an
// absolute one from the file system. It applies the defaults first. The
// builder calls it before it validates the config; call it yourself to see
// the components a build would add. Entries that fail are reported and stay
// unexpanded; the others are added.
func (c *Config) HarvestFiles() Diagnostics {
	ApplyDefaults(c)

	var diags Diagnostics

	for i, entry := range c.Harvest {
		if entry == nil || entry.expanded {
			continue
		}

		entryPath := fmt.Sprintf("harvest[%d]", i)

		added, root, err := c.harvestEntry(entry)
		if err != nil {
			diags = append(diags, Diagnostic{Severity: SeverityError, Path: entryPath, Message: err.Error()})

			continue
		}

		for _, comp := range added {
			file := comp.Files[0]
			rel := file.Source

			if root != "." {
				rel = strings.TrimPrefix(rel, root+"/")
			}

			// Diagnostics of a harvested file name the entry and the path
			// below its source.
			comp.origin = fmt.Sprintf("%s{%s}", entryPath, rel)
			comp.Bitness = entry.Bitness
			comp.harvested = true
			file.AllowUnversionedExecutable = entry.AllowUnversionedExecutable
		}

		entry.expanded = true

		if c.holdsYAMLFile(entry.Source) {
			diags = append(diags, Diagnostic{
				Severity: SeverityWarning, Path: entryPath + ".source",
				Message: "the folder holds the YAML file, so a package built next to it is harvested by the next build; harvest a separate output folder",
			})
		}
	}

	// The default feature takes the new components when there was no
	// component before.
	ApplyDefaults(c)

	return diags
}

// harvestEntry adds the files of one entry and returns them with the root
// they were read from.
func (c *Config) harvestEntry(entry *Harvest) ([]*Component, string, error) {
	if strings.TrimSpace(entry.Source) == "" {
		return nil, "", errors.New("source is required")
	}

	filter, err := newHarvestFilter(entry.Include, entry.Exclude)
	if err != nil {
		return nil, "", err
	}

	// The checks of Validate, worded for YAML, for a caller that did not
	// run it.
	if findDirectory(c.Directories, entry.Directory) == nil {
		return nil, "", fmt.Errorf("directory %q is not defined", entry.Directory)
	}

	if entry.Feature == "" {
		if n := featureCount(c.Features); n > 1 {
			return nil, "", fmt.Errorf("feature is required: the package has %d features", n)
		}
	}

	fsys, root := c.harvestFS(entry.Source)

	added, err := c.harvestTree(fsys, root, harvestIDRoot(entry.Source), entry.Directory, TreeOptions{Feature: entry.Feature}, filter)
	if err != nil {
		return nil, "", errors.New(strings.TrimPrefix(err.Error(), "add tree: "))
	}

	return added, root, nil
}

// harvestFS returns the file system and the root in it that an entry reads.
// A relative source on disk is opened as a folder of its own, so that it
// may lie outside the YAML file's folder.
func (c *Config) harvestFS(src string) (fs.FS, string) {
	switch {
	case filepath.IsAbs(src):
		return os.DirFS(src), "."
	case c.FS != nil:
		return c.FS, harvestIDRoot(src)
	default:
		return os.DirFS(filepath.Join(c.BaseDir, filepath.FromSlash(harvestIDRoot(src)))), "."
	}
}

// holdsYAMLFile reports whether a source read from disk is the YAML file's
// folder or one above it.
func (c *Config) holdsYAMLFile(src string) bool {
	if c.BaseDir == "" || (c.FS != nil && !filepath.IsAbs(src)) {
		return false
	}

	dir := filepath.FromSlash(harvestIDRoot(src))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(c.BaseDir, dir)
	}

	// Load leaves BaseDir relative ("." for msi.yaml), which Rel cannot
	// compare with an absolute source.
	dir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	base, err := filepath.Abs(c.BaseDir)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(dir, base)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// featureCount counts the features of the tree.
func featureCount(features []*Feature) int {
	count := 0

	WalkFeatures(features, func(_, _ *Feature) { count++ })

	return count
}

// harvestIDRoot is the source as written in slash form, cleaned; the ids of
// the entry are derived from it, so they do not depend on where the build
// runs as long as the source is relative.
func harvestIDRoot(src string) string {
	return path.Clean(strings.ReplaceAll(src, `\`, "/"))
}

// harvest checks the harvest entries without reading the folders, which
// HarvestFiles does.
func (v *validator) harvest(cfg *Config, known *refs) {
	features := featureCount(cfg.Features)

	for i, entry := range cfg.Harvest {
		entryPath := fmt.Sprintf("harvest[%d]", i)

		if entry == nil {
			v.errorf(entryPath, "empty entry")

			continue
		}

		v.required(entryPath+".source", strings.TrimSpace(entry.Source))

		if cfg.Package.LongFileNames != nil && !*cfg.Package.LongFileNames {
			v.errorf(entryPath, "longFileNames is false, which needs 8.3 file names, but harvested files keep their own names")
		}

		v.harvestDirectory(cfg, known, entryPath, entry)

		v.harvestFeature(cfg, entryPath, entry, features)

		if entry.Bitness != "" {
			v.oneOf(entryPath+".bitness", entry.Bitness, Auto, "32", "64")
		}

		v.harvestPatterns(entryPath+".include", entry.Include)
		v.harvestPatterns(entryPath+".exclude", entry.Exclude)
	}
}

// harvestDirectory checks the directory an entry installs to.
func (v *validator) harvestDirectory(cfg *Config, known *refs, entryPath string, entry *Harvest) {
	if known.dirs[entry.Directory] == nil {
		v.errorf(entryPath+".directory", "directory %q is not defined", entry.Directory)

		return
	}

	top := known.topFolder(entry.Directory)
	if cfg.Package.Platform.Is64Bit() && entry.Bitness != "32" && folders32[top] {
		v.warnf(entryPath+".directory", "%s is the 32-bit folder on 64-bit Windows (ICE80); "+
			"set bitness: 32 for 32-bit files, or use its 64-bit counterpart", top)
	}
}

// harvestFeature checks the feature an entry joins.
func (v *validator) harvestFeature(cfg *Config, entryPath string, entry *Harvest, features int) {
	switch {
	case entry.Feature != "" && findFeature(cfg.Features, entry.Feature) == nil:
		v.errorf(entryPath+".feature", "feature %q is not defined", entry.Feature)
	case entry.Feature == "" && features > 1:
		v.errorf(entryPath+".feature", "required: the package has %d features", features)
	}
}

// harvestPatterns checks the glob patterns of an include or exclude list.
func (v *validator) harvestPatterns(listPath string, patterns StringList) {
	for i, pattern := range patterns {
		if _, err := parseGlob(pattern); err != nil {
			v.errorf(fmt.Sprintf("%s[%d]", listPath, i), "%q: %v", pattern, err)
		}
	}
}
