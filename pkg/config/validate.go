package config

import (
	"fmt"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"unicode"

	"github.com/jkroepke/go-msi/internal/condition"
	"github.com/jkroepke/go-msi/pkg/msidb"
)

// Limits checked by Validate. Section references are to docs/spec/MSISPEC.md;
// test IDs to docs/spec/MSISPEC-RESULTS.md.
const (
	MaxIdentifierLength  = 72
	MaxFeatureIDLength   = 38
	MaxFeatureDepth      = 16
	MaxProductNameLength = 63
	MaxStreamNameLength  = 62 // DB-11
	MaxTableNameLength   = 31
	MaxColumns           = 31 // DB-07: the 32nd column is rejected
	maxIconKeyLength     = MaxStreamNameLength - len("Icon.")
	maxBinaryKeyLength   = MaxStreamNameLength - len("Binary.")
	minSequenceNumber    = -4
	maxSequenceNumber    = 32767
)

// cabinetForbidden are the characters a cabinet file name cannot hold.
const cabinetForbidden = `\/:*?"<>|#`

// plainFileName reports whether name holds none of cabinetForbidden and no
// control characters, which also keeps the cabinet off the streams that
// start with U+0005, such as \x05SummaryInformation.
func plainFileName(name string) bool {
	return !strings.ContainsAny(name, cabinetForbidden) && !strings.ContainsFunc(name, unicode.IsControl)
}

// Validate fills in defaults, checks cfg and returns every finding. It reads
// no input files; checks that need them (PE versions, sequence numbering,
// conditions) run in the builder. Call Diagnostics.Err to decide whether to
// build.
func Validate(cfg *Config) Diagnostics {
	ApplyDefaults(cfg)

	v := &validator{}
	r := newRefs(cfg)

	v.product(cfg)
	v.pkg(cfg)
	v.upgrade(cfg)
	v.directories(cfg, r)
	v.features(cfg, r)
	v.components(cfg, r)
	v.properties(cfg)
	v.launchConditions(cfg)
	v.customActions(cfg, r)
	v.streams(cfg)
	v.firewall(cfg, r)
	v.ui(cfg)
	v.tables(cfg)

	return v.diags
}

// refs indexes the ids that other sections refer to.
type refs struct {
	dirs       map[string]*Directory
	dirParent  map[string]string
	components map[string]*Component
	fileIDs    map[string]bool
	inFeature  map[string]bool
}

func newRefs(cfg *Config) *refs {
	r := &refs{
		dirs:       map[string]*Directory{},
		dirParent:  map[string]string{},
		components: map[string]*Component{},
		fileIDs:    map[string]bool{},
		inFeature:  map[string]bool{},
	}

	var walk func([]*Directory, string)

	walk = func(ds []*Directory, parent string) {
		for _, d := range ds {
			if _, dup := r.dirs[d.ID]; !dup {
				r.dirs[d.ID] = d
				r.dirParent[d.ID] = parent
			}

			walk(d.Children, d.ID)
		}
	}
	walk(cfg.Directories, "")

	for _, c := range cfg.Components {
		r.components[c.ID] = c

		for _, f := range c.Files {
			r.fileIDs[fileID(f)] = true
		}
	}

	eachFeature(cfg.Features, func(f *Feature, _ int) {
		for _, c := range f.Components {
			r.inFeature[c.ID] = true
		}
	})

	return r
}

// fileName returns the long name of a file.
func fileName(f *File) string {
	name := f.Name
	if name == "" {
		name = path.Base(strings.ReplaceAll(f.Source, `\`, "/"))
	}

	if _, long, ok := strings.Cut(name, "|"); ok {
		return long
	}

	return name
}

// fileID returns the File key: the explicit id, else the long name.
func fileID(f *File) string {
	if f.ID != "" {
		return f.ID
	}

	return fileName(f)
}

// topFolder returns the standard folder at the root of a directory's chain.
func (r *refs) topFolder(id string) string {
	for i := 0; i < 64; i++ {
		parent, ok := r.dirParent[id]
		if !ok || parent == "" {
			return id
		}

		id = parent
	}

	return id
}

func (v *validator) product(cfg *Config) {
	p := cfg.Product

	if v.required(productNamePath, p.Name) && len([]rune(p.Name)) > MaxProductNameLength {
		v.errorf(productNamePath, "at most %d characters", MaxProductNameLength)
	}

	v.required("product.manufacturer", p.Manufacturer)

	if v.required("product.version", p.Version) {
		ver, err := ParseVersion(p.Version)
		if err != nil {
			v.errorf("product.version", "%v", err)
		} else if ver.Fields == 4 {
			v.warnf("product.version", "Windows Installer compares only the first three fields; "+
				"releases that differ in the fourth are the same version to upgrades (RUN-03)")
		}
	}

	if *p.Language < 0 || *p.Language > 0xFFFF {
		v.errorf("product.language", "%d is not an LCID", *p.Language)
	}

	v.upgradeCode("product.upgradeCode", p.UpgradeCode)

	if p.ProductCode != Auto {
		v.guid("product.productCode", p.ProductCode)
		v.warnf("product.productCode", "a fixed ProductCode makes every rebuild of this version fail "+
			"with 1638 over an installed copy (RUN-11); use auto unless this is a minor upgrade")
	}

	if p.PackageCode != Auto {
		v.guid("product.packageCode", p.PackageCode)
	}

	if p.ARP.Icon != "" {
		if _, ok := cfg.Icons[p.ARP.Icon]; !ok {
			v.errorf("product.arp.icon", "icon %q is not defined in icons", p.ARP.Icon)
		}
	}

	if p.ARP.Size < 0 {
		v.errorf("product.arp.size", "must not be negative")
	}
}

func (v *validator) guid(path, s string) {
	if _, err := NormalizeGUID(s); err != nil {
		v.errorf(path, "%v", err)
	}
}

// upgradeCode requires a GUID other than the nil GUID: products sharing an
// upgrade code upgrade each other, so a placeholder copied from an example
// would tie unrelated products together.
func (v *validator) upgradeCode(key, s string) {
	if !v.required(key, s) {
		return
	}

	g, err := NormalizeGUID(s)

	switch {
	case err != nil:
		v.errorf(key, "%v", err)
	case g == NilGUID:
		v.errorf(key, "the nil GUID is a placeholder, not an upgrade code; generate a new GUID")
	}
}

func (v *validator) pkg(cfg *Config) {
	pkg := cfg.Package

	if pkg.Platform == "" {
		v.errorf("package.platform", "required: x86, x64 or arm64")
	}

	pc, err := PageCount(pkg.InstallerVersion)

	switch {
	case err != nil:
		v.errorf("package.installerVersion", "%v", err)
	case pc > MaxPageCount:
		v.errorf("package.installerVersion", "%s is above 5.0; Windows Installer 5.0 refuses such packages with 1613 (SUM-03)",
			pkg.InstallerVersion)
	case pkg.Platform != "" && pc < MinPageCount(pkg.Platform):
		v.errorf("package.installerVersion", "%s needs at least %d.%d", pkg.Platform,
			MinPageCount(pkg.Platform)/100, MinPageCount(pkg.Platform)%100)
	}

	if pkg.Codepage != 65001 && pkg.Codepage != 0 && !ansiCodepages[pkg.Codepage] {
		v.errorf("package.codepage", "%d is not supported; use an ANSI code page or 65001", pkg.Codepage)
	}

	v.oneOf("package.compression", pkg.Compression, "mszip", "none")

	if !plainFileName(pkg.Cabinet) {
		v.errorf("package.cabinet", "%q is not a plain file name", pkg.Cabinet)
	} else if err := msidb.CheckStreamName(pkg.Cabinet); err != nil {
		v.errorf("package.cabinet", "not a valid stream name (DB-11): %v", err)
	}
}

func (v *validator) upgrade(cfg *Config) {
	u := cfg.Upgrade
	if u == nil || u.Disabled {
		return
	}

	v.oneOf("upgrade.downgrade", u.Downgrade, "error", "allow", "ignore")
	v.oneOf("upgrade.removeOld", u.RemoveOld,
		"afterInstallValidate", "afterInstallInitialize", "afterInstallExecute", "afterInstallFinalize")

	seen := map[string]string{}

	actionProperty := func(path, name string) {
		if !v.required(path, name) {
			return
		}

		if !IsIdentifier(name) || !IsPublicProperty(name) {
			v.errorf(path, "%q must be an upper-case identifier, so that it is public", name)
		}

		if other, dup := seen[name]; dup {
			v.errorf(path, "%s is already used by %s", name, other)
		}

		seen[name] = path

		if _, ok := cfg.Properties[name]; ok {
			v.errorf(path, "%s must not be defined in properties (ICE61)", name)
		}
	}

	actionProperty("upgrade.property", u.Property)

	if u.Downgrade != "allow" {
		actionProperty("upgrade.newerProperty", u.NewerProperty)
	}

	for i, rel := range u.Related {
		p := fmt.Sprintf("upgrade.related[%d]", i)

		v.upgradeCode(p+".upgradeCode", rel.UpgradeCode)

		if rel.MinVersion == "" && rel.MaxVersion == "" {
			v.errorf(p, "minVersion or maxVersion is required")
		}

		for _, f := range []struct{ path, val string }{{p + ".minVersion", rel.MinVersion}, {p + ".maxVersion", rel.MaxVersion}} {
			if f.val == "" {
				continue
			}

			if _, err := ParseVersion(f.val); err != nil {
				v.errorf(f.path, "%v", err)
			}
		}

		actionProperty(p+".property", rel.Property)
	}
}

func (v *validator) directories(cfg *Config, r *refs) {
	seen := map[string]bool{}

	var walk func([]*Directory, string, bool)

	walk = func(ds []*Directory, prefix string, top bool) {
		names := make(map[string]string)

		for i, d := range ds {
			p := fmt.Sprintf("%s[%d]", prefix, i)

			v.sameFolder(names, p, d, top)

			if v.required(p+".id", d.ID) {
				if !IsIdentifier(d.ID) || len(d.ID) > MaxIdentifierLength {
					v.errorf(p+".id", "%q is not an identifier of at most %d characters", d.ID, MaxIdentifierLength)
				}

				if seen[d.ID] {
					v.errorf(p+".id", "directory %s is defined twice", d.ID)
				}

				seen[d.ID] = true
			}

			switch {
			case top && !standardFolders[d.ID]:
				v.errorf(p+".id", "a top-level directory must be a standard folder such as ProgramFiles64Folder")
			case top && d.Name != "":
				v.errorf(p+".name", "a standard folder has no name")
			case !top && !v.required(p+".name", d.Name):
			case !top && !validName(d.Name) && d.origin == productNamePath:
				v.errorf(d.origin, "%q cannot be used as a folder name; set installDir.name", d.Name)
			case !top && !validName(d.Name) && d.origin != "":
				v.errorf(d.origin, "%q is not a valid folder name", d.Name)
			case !top && !validName(d.Name):
				v.errorf(p+".name", "%q is not a valid name or short|long pair", d.Name)
			}

			if d.Configurable && !IsPublicProperty(d.ID) {
				v.errorf(p+".id", "a configurable directory needs an upper-case id")
			}

			walk(d.Children, p+".children", false)
		}
	}

	walk(cfg.Directories, "directories", true)

	if install := cfg.InstallDir; r.dirs[install.ID] == nil {
		switch {
		case !IsIdentifier(install.ID) || !IsPublicProperty(install.ID):
			v.errorf("installDir.id", "%q must be an upper-case identifier, so that it can be set on the command line", install.ID)
		case !standardFolders[install.Parent]:
			v.errorf("installDir.parent", "%q is not a standard folder such as ProgramFiles64Folder", install.Parent)
		case len(splitPath(install.Name)) == 0:
			v.errorf("installDir.name", "required")
		}
	}

	if cfg.Product.ARP.InstallLocation != "" && r.dirs[cfg.Product.ARP.InstallLocation] == nil {
		v.errorf("product.arp.installLocation", "directory %q is not defined", cfg.Product.ARP.InstallLocation)
	}
}

// sameFolder warns when two sibling directories name the same folder: the
// rows install to one place, but setting one on the command line leaves the
// other behind.
func (v *validator) sameFolder(names map[string]string, dirPath string, dir *Directory, top bool) {
	// A standard folder's path comes from its property, not its name.
	if top || dir.Name == "" || standardFolders[dir.ID] {
		return
	}

	long := strings.ToLower(longPart(dir.Name))
	if other, dup := names[long]; dup {
		field := dirPath + ".name"
		if dir.origin != "" {
			field = dir.origin
		}

		v.warnf(field, "directories %s and %s are the same folder; moving one with a property "+
			"leaves the other behind, so use one id", other, dir.ID)

		return
	}

	names[long] = dir.ID
}

func (v *validator) features(cfg *Config, r *refs) {
	seen := map[string]bool{}

	var walk func([]*Feature, string, int)

	walk = func(fs []*Feature, prefix string, depth int) {
		for i, f := range fs {
			p := fmt.Sprintf("%s[%d]", prefix, i)

			v.featureID(p, f.ID, seen)

			if depth > MaxFeatureDepth {
				v.errorf(p, "features nest at most %d deep", MaxFeatureDepth)
			}

			if *f.Level < 0 || *f.Level > maxSequenceNumber {
				v.errorf(p+".level", "must be between 0 and %d", maxSequenceNumber)
			}

			v.oneOf(p+".display", f.Display, "expanded", "collapsed", "hidden")

			v.featureRefs(p, f, r)

			walk(f.Features, p+".features", depth+1)
		}
	}

	walk(cfg.Features, "features", 1)

	if len(cfg.Components) > 0 && len(cfg.Features) == 0 {
		v.errorf("features", "at least one feature is required")
	}
}

// featureID checks a feature's id and records it in seen.
func (v *validator) featureID(featurePath, featureID string, seen map[string]bool) {
	if !v.required(featurePath+".id", featureID) {
		return
	}

	if !IsIdentifier(featureID) || len(featureID) > MaxFeatureIDLength {
		v.errorf(featurePath+".id", "%q is not an identifier of at most %d characters", featureID, MaxFeatureIDLength)
	}

	if seen[featureID] {
		v.errorf(featurePath+".id", "feature %s is defined twice", featureID)
	}

	seen[featureID] = true
}

// featureRefs checks the directory and components a feature refers to.
func (v *validator) featureRefs(featurePath string, feature *Feature, known *refs) {
	if feature.Directory != "" {
		if d := known.dirs[feature.Directory]; d == nil {
			v.errorf(featurePath+".directory", "directory %q is not defined", feature.Directory)
		} else if !d.Configurable {
			v.errorf(featurePath+".directory", "directory %s must be configurable", feature.Directory)
		}
	}

	for j, c := range feature.Components {
		if known.components[c.ID] == nil {
			v.errorf(fmt.Sprintf("%s.components[%d]", featurePath, j), "component %q is not defined", c.ID)
		}
	}
}

// preserve checks components that keep what the user changed. RUN-24: with
// the old product removed first, a NeverOverwrite component without
// Permanent loses its key file or registry value, because the new product
// skips it and the removal of the old one deletes it. A late removal keeps
// it, also when the old version had the same component without the flag.
// Permanent in both versions also kept it, but leaves it after uninstall.
func (v *validator) preserve(cfg *Config, field string, comp *Component) {
	if comp.Preserve && len(comp.Services) > 0 {
		v.errorf(field+".preserve", "a preserved component is never updated; keep the service in its own component")
	}

	if cfg.Upgrade == nil || cfg.Upgrade.Disabled {
		return
	}

	v.removeOld(cfg.Upgrade.RemoveOld, field, comp)
}

// removeOld checks a component's NeverOverwrite against the removeOld window.
func (v *validator) removeOld(removeOld, field string, comp *Component) {
	// RUN-24 ran the early removal right after InstallValidate (1401).
	// afterInstallInitialize also removes the old product before the new one
	// installs its files, so it is treated the same (not tested).
	early := removeOld == "afterInstallValidate" || removeOld == "afterInstallInitialize"

	switch {
	case comp.Preserve && early:
		// Also with permanent: RUN-24 lost the edit when only the new
		// version had both flags.
		v.errorf(field+".preserve", "needs removeOld afterInstallExecute: %s removes the old product first, "+
			"which deleted the preserved file and registry value in RUN-24 (tested right after InstallValidate)", removeOld)
	case comp.Preserve && removeOld != PreserveRemoveOld:
		v.warnf(field+".preserve", "only removeOld afterInstallExecute is tested to keep preserved files and "+
			"registry values (RUN-24); %s is not", removeOld)
	case comp.NeverOverwrite && !comp.Permanent && early:
		hint := "use preserve: true"
		if len(comp.Services) > 0 {
			hint = "move the service to its own component and use preserve: true"
		}

		v.warnf(field+".neverOverwrite", "removeOld %s removes the old product first, which deleted a neverOverwrite "+
			"component's file and registry value in RUN-24 (tested right after InstallValidate); %s", removeOld, hint)
	}
}

// purge checks that a purged directory is a folder of the product: the
// action deletes whatever the directory property points to at uninstall.
func (v *validator) purge(cfg *Config, known *refs, field string, comp *Component) {
	if !comp.Purge.Enabled() {
		return
	}

	v.purgeFolder(cfg, known, field, comp)

	if comp.Purge.Condition != "" {
		if err := condition.Check(comp.Purge.Condition); err != nil {
			v.errorf(field+".purge.condition", "%v", err)
		}
	}
}

// purgeFolder rejects a purge of a folder that may not belong to the
// product: a standard folder, a folder that is its parent, or one below a
// folder a property can move.
func (v *validator) purgeFolder(cfg *Config, known *refs, field string, comp *Component) {
	dir := known.dirs[comp.Directory]

	switch {
	case dir == nil:
		// Reported for the directory already.
	case IsStandardFolder(comp.Directory) || known.dirParent[comp.Directory] == "":
		v.errorf(field+".purge", "%s is a standard folder; purge a folder below it", comp.Directory)
	case dir.Name == "" || dir.Name == ".":
		v.errorf(field+".purge", "%s is its parent folder; purge a folder with its own name", comp.Directory)
	default:
		if movable := movableAncestor(cfg, known, comp.Directory); movable != "" {
			v.errorf(field+".purge", "%s can be pointed at any folder (a public id, configurable, or set by an action, "+
				"a property or a dialog field), so the purge could delete a folder the product does not own; purge a "+
				"folder with private ids below a fixed standard folder, such as CommonAppDataFolder/Vendor/Product", movable)
		}
	}
}

func (v *validator) components(cfg *Config, r *refs) {
	seen := map[string]bool{}
	fileIDs := map[string]string{}
	targets := map[string]string{}
	regIDs := map[string]bool{}
	perUser := cfg.Package.Scope == ScopePerUser

	for i, c := range cfg.Components {
		p := c.Path(i) //nolint:varnamelen // the YAML path, named p throughout this file

		if c.ref {
			v.errorf(p, "%q: a component here is a mapping; ids only refer to components from features", c.ID)

			continue
		}

		if v.required(p+".id", c.ID) {
			if !IsIdentifier(c.ID) || len(c.ID) > MaxIdentifierLength {
				v.errorf(p+".id", "%q is not an identifier of at most %d characters", c.ID, MaxIdentifierLength)
			}

			if seen[c.ID] {
				v.errorf(p+".id", "component %s is defined twice", c.ID)
			}

			seen[c.ID] = true

			if !r.inFeature[c.ID] {
				v.errorf(p, "component %s is in no feature (ICE21)", c.ID)
			}
		}

		if v.required(p+".directory", c.Directory) {
			switch {
			case r.dirs[c.Directory] == nil && c.Directory == cfg.InstallDir.ID && !validInstallDir(cfg.InstallDir):
				// Reported at installDir.
			case r.dirs[c.Directory] == nil:
				v.errorf(p+".directory", "directory %q is not defined", c.Directory)
			default:
				top := r.topFolder(c.Directory)
				if cfg.Package.Platform.Is64Bit() && c.Bitness != "32" && folders32[top] {
					v.warnf(p+".directory", "%s is the 32-bit folder on 64-bit Windows (ICE80); "+
						"use its 64-bit counterpart", top)
				}

				if perUser && perMachineFolders[top] {
					v.errorf(p+".directory", "a per-user package cannot install under %s (ICE105)", top)
				}

				switch {
				case !profileFolders[top]:
				case !userRegistryKeyPath(c, perUser):
					v.warnf(p+".directory", "a component in the user profile (%s) needs an HKCU registry value "+
						"as key path (ICE38), and its folders need RemoveFolder rows (ICE64), which go-msi "+
						"does not write yet", top)
				default:
					v.warnf(p+".directory", "the folders of a component in the user profile (%s) need "+
						"RemoveFolder rows (ICE64), which go-msi does not write yet", top)
				}
			}
		}

		switch c.GUID {
		case Auto:
		case None:
			if !c.Permanent {
				v.errorf(p+".guid", "none is only allowed with permanent: true; "+
					"a component without a GUID is never removed")
			}
		default:
			v.guid(p+".guid", c.GUID)
		}

		v.oneOf(p+".bitness", c.Bitness, Auto, "32", "64")
		v.preserve(cfg, p, c)
		v.purge(cfg, r, p, c)

		if c.Bitness == "64" && !cfg.Package.Platform.Is64Bit() {
			v.errorf(p+".bitness", "64-bit components need an x64 or arm64 package")
		}

		keyFile := v.componentFiles(cfg, p, c, fileIDs, targets)
		v.componentRegistry(p, c, regIDs, perUser)

		switch kp := c.KeyPath; {
		case kp == "" || kp == "directory":
		case keyFileByRef(c, kp) != nil:
			keyFile = keyFileByRef(c, kp)
		case hasRegistryID(c, kp):
			keyFile = nil
		default:
			v.errorf(p+".keyPath", "%q is neither a file nor a registry id of this component", kp)
		}

		if c.KeyPath == "" && len(c.Files) == 0 {
			keyFile = nil
		}

		if keyFile != nil && keyFile.Version == None && len(c.Files) > 1 {
			v.warnf(p+".keyPath", "the key file is unversioned; other files of the component are only "+
				"replaced together with it")
		}

		for j, s := range c.Services {
			v.service(fmt.Sprintf("%s.services[%d]", p, j), s, keyFile, perUser)
			v.permissions(fmt.Sprintf("%s.services[%d].permissions", p, j), s.Permissions)
		}

		for j, f := range c.Files {
			v.permissions(fmt.Sprintf("%s.files[%d].permissions", p, j), f.Permissions)
		}

		for j, r := range c.Registry {
			v.permissions(fmt.Sprintf("%s.registry[%d].permissions", p, j), r.Permissions)
		}

		directoryKey := c.KeyPath == "directory" || c.KeyPath == "" && len(c.Files) == 0 && len(c.Registry) == 0
		if v.permissions(p+".folderPermissions", c.FolderPermissions) && !c.CreateFolder && !directoryKey {
			v.errorf(p+".folderPermissions", "the component creates no folder row; set createFolder: true")
		}

		for j, e := range c.EventLog {
			ep := fmt.Sprintf("%s.eventLog[%d]", p, j)
			v.required(ep+".source", e.Source)
			v.required(ep+".messageFile", e.MessageFile)

			if perUser {
				v.errorf(ep, "event log sources are per-machine (ICE105)")
			}
		}

		for j, e := range c.Environment {
			v.environment(fmt.Sprintf("%s.environment[%d]", p, j), e, perUser)
		}
	}
}

// permissions validates an SDDL block and reports whether one is set.
func (v *validator) permissions(field string, perm *Permissions) bool {
	if perm == nil {
		return false
	}

	warnings, err := CheckSDDL(perm.SDDL)
	if err != nil {
		v.errorf(field+".sddl", "%v", err)
	}

	for _, warning := range warnings {
		v.warnf(field+".sddl", "%s", warning)
	}

	if perm.Condition != "" {
		if err := condition.Check(perm.Condition); err != nil {
			v.errorf(field+".condition", "%v", err)
		}
	}

	return true
}

// userRegistryKeyPath reports whether the component's key path, resolved as
// the builder does, is a registry value under HKCU (HKMU is HKCU in a
// per-user package).
func userRegistryKeyPath(c *Component, perUser bool) bool {
	key := registryKeyPath(c)

	return key != nil && (key.Root == "HKCU" || perUser && key.Root == "HKMU")
}

// registryKeyPath returns the registry value that is the component's key
// path, or nil when the key path is a file or the directory.
func registryKeyPath(comp *Component) *RegistryValue {
	switch {
	case comp.KeyPath == "" && len(comp.Files) == 0 && len(comp.Registry) > 0:
		return comp.Registry[0]
	case comp.KeyPath == "" || comp.KeyPath == "directory":
		return nil
	}

	for _, value := range comp.Registry {
		if value.ID == comp.KeyPath {
			return value
		}
	}

	return nil
}

func keyFileByRef(c *Component, ref string) *File {
	for _, f := range c.Files {
		if fileID(f) == ref || fileName(f) == ref {
			return f
		}
	}

	return nil
}

func hasRegistryID(c *Component, id string) bool {
	for _, r := range c.Registry {
		if r.ID == id {
			return true
		}
	}

	return false
}

// componentFiles validates the files of c and returns its default key file.
func (v *validator) componentFiles(cfg *Config, p string, c *Component, ids, targets map[string]string) *File {
	versioned := 0

	for j, f := range c.Files {
		fp := fmt.Sprintf("%s.files[%d]", p, j)

		v.required(fp+".source", f.Source)

		name := f.Name
		if name == "" {
			name = fileName(f)
		}

		if !validName(name) {
			v.errorf(fp+".name", "%q is not a valid file name or short|long pair", name)
		}

		if !*cfg.Package.LongFileNames && !strings.Contains(name, "|") && !isShortName(name) {
			v.errorf(fp+".name", "longFileNames is false, so %q needs a short name", name)
		}

		id := fileID(f)
		if !IsIdentifier(id) || len(id) > MaxIdentifierLength {
			v.errorf(fp+".id", "file key %q is not an identifier; set id", id)
		}

		if other, dup := ids[id]; dup {
			v.errorf(fp+".id", "file key %q is also used by %s; set id on one of them", id, other)
		}

		ids[id] = fp

		target := c.Directory + `\` + strings.ToLower(fileName(f))
		if other, dup := targets[target]; dup {
			v.errorf(fp, "%s installs the same file as %s", fileName(f), other)
		}

		targets[target] = fp

		v.fileVersion(fp, f, c)

		if versionedFile(f) {
			versioned++
		}

		switch {
		case f.Language == Auto, f.Language == None:
		default:
			for _, lcid := range strings.Split(f.Language, ",") {
				if n, err := strconv.Atoi(lcid); err != nil || n < 0 || n > 0xFFFF {
					v.errorf(fp+".language", "%q is not auto, none or a comma-separated LCID list", f.Language)

					break
				}
			}
		}
	}

	if c.GUID == Auto && versioned > 1 && len(c.Files) > 1 {
		v.warnf(p+".files", "more than one versioned file in a component: only the key file decides "+
			"whether the others are replaced")
	}

	if len(c.Files) > 0 {
		return c.Files[0]
	}

	return nil
}

// versionedFile reports whether the File row gets a Version: an explicit
// version, or auto on a PE file, whose version resource the builder reads.
// Auto on any other file stores no version.
func versionedFile(f *File) bool {
	switch f.Version {
	case None:
		return false
	case Auto:
		return peExtensions[strings.ToLower(path.Ext(fileName(f)))]
	default:
		return true
	}
}

// fileVersion enforces the RUN-02 rule: a PE file is never unversioned by
// accident, since an upgrade with early removal then deletes it.
func (v *validator) fileVersion(fp string, f *File, c *Component) {
	isPE := peExtensions[strings.ToLower(path.Ext(fileName(f)))]

	switch f.Version {
	case Auto:
	case None:
		if isPE && !f.AllowUnversionedExecutable {
			v.errorf(fp+".version", "%s is an executable; without a version an upgrade can delete it "+
				"(RUN-02). Set allowUnversionedExecutable: true if this is intended", fileName(f))
		} else if isPE {
			v.warnf(fp+".version", "unversioned executable: an upgrade with removeOld before "+
				"InstallFinalize deletes it when the installed copy was versioned (RUN-02)")
		}
	default:
		if _, err := ParseVersion(f.Version); err == nil {
			return
		}

		companion := false

		for _, other := range c.Files {
			if other != f && fileID(other) == f.Version {
				companion = true
			}
		}

		if !companion {
			v.errorf(fp+".version", "%q is not auto, none, a version or the id of a file in this component", f.Version)
		}
	}
}

func (v *validator) componentRegistry(p string, c *Component, ids map[string]bool, perUser bool) {
	for j, rv := range c.Registry {
		rp := fmt.Sprintf("%s.registry[%d]", p, j)

		if rv.ID != "" {
			if !IsIdentifier(rv.ID) {
				v.errorf(rp+".id", "%q is not an identifier", rv.ID)
			}

			if ids[rv.ID] {
				v.errorf(rp+".id", "registry id %s is defined twice", rv.ID)
			}

			ids[rv.ID] = true
		}

		v.oneOf(rp+".root", rv.Root, "HKLM", "HKCU", "HKCR", "HKU", "HKMU")

		if perUser && (rv.Root == "HKLM" || rv.Root == "HKCR") {
			v.errorf(rp+".root", "a per-user package cannot write %s (ICE105)", rv.Root)
		}

		v.required(rp+".key", rv.Key)

		if strings.HasPrefix(rv.Key, `\`) || strings.HasSuffix(rv.Key, `\`) {
			v.errorf(rp+".key", "no leading or trailing backslash")
		}

		v.oneOf(rp+".type", rv.Type, "string", "expandString", "multiString", "dword", "binary", "key")

		switch rv.Type {
		case "key":
			if len(rv.Value) > 0 || rv.Name != "" {
				v.errorf(rp, "type key takes neither name nor value")
			}
		case "multiString":
			if len(rv.Value) == 0 {
				v.errorf(rp+".value", "required")
			}
		default:
			if len(rv.Value) != 1 {
				v.errorf(rp+".value", "exactly one value is required for type %s", rv.Type)

				continue
			}

			v.registryScalar(rp+".value", rv.Type, rv.Value[0])
		}

		if (rv.Append || rv.Prepend) && rv.Type != "multiString" {
			v.errorf(rp, "append and prepend need type multiString")
		}

		if rv.Append && rv.Prepend {
			v.errorf(rp, "append and prepend are exclusive")
		}

		if rv.RemoveOnUninstall && rv.Type != "key" {
			v.errorf(rp+".removeOnUninstall", "only for type key; values are always removed with their component")
		}
	}
}

func (v *validator) registryScalar(path, typ, val string) {
	switch typ {
	case "dword":
		if strings.Contains(val, "[") {
			return
		}

		n, err := strconv.ParseInt(val, 0, 64)
		if err != nil || n < math.MinInt32 || n > math.MaxUint32 {
			v.errorf(path, "%q is not a 32-bit integer", val)
		}
	case "binary":
		h := strings.TrimPrefix(strings.ToLower(val), "0x")
		if h == "" || strings.Trim(h, "0123456789abcdef") != "" {
			v.errorf(path, "%q is not hexadecimal", val)
		}
	case "string", "expandString":
		if strings.Contains(val, "[~]") {
			v.errorf(path, "[~] makes the value REG_MULTI_SZ; use type multiString")
		}
	}
}

func (v *validator) service(p string, s *Service, keyFile *File, perUser bool) {
	if perUser {
		v.errorf(p, "a per-user package cannot install services (ICE105)")
	}

	if keyFile == nil {
		v.errorf(p, "a service needs its executable as the component's key file")
	} else if !strings.EqualFold(path.Ext(fileName(keyFile)), ".exe") {
		v.errorf(p, "the key file %s is not an .exe", fileName(keyFile))
	}

	v.required(p+".name", s.Name)
	v.oneOf(p+".type", s.Type, "ownProcess", "shareProcess")
	v.oneOf(p+".start", s.Start, "auto", "demand", "disabled")
	v.oneOf(p+".errorControl", s.ErrorControl, "ignore", "normal", "critical")

	if s.DelayedAutoStart && s.Start != "auto" {
		v.errorf(p+".delayedAutoStart", "needs start: auto")
	}

	if s.Password != "" && !strings.Contains(s.Password, "[") {
		v.errorf(p+".password", "use a hidden property such as [SERVICE_PASSWORD], never a literal password")
	}

	if !*s.Control.DeleteOnUninstall {
		v.warnf(p+".control.deleteOnUninstall", "without it the service stays registered after uninstall (RUN-05)")
	}

	if rec := s.Recovery; rec != nil {
		if len(rec.Actions) == 0 || len(rec.Actions) > 3 {
			v.errorf(p+".recovery.actions", "one to three actions")
		}

		for j, a := range rec.Actions {
			step := fmt.Sprintf("%s.recovery.actions[%d]", p, j)
			v.oneOf(step, a.Action, "restart", "none")

			if a.Restart != nil {
				if a.Action != recoveryRestart {
					v.errorf(step+".restart", "a delay is only allowed on a restart action")
				}

				v.recoveryDuration(step+".restart", *a.Restart, time.Millisecond)
			}
		}

		v.recoveryDuration(p+".recovery.restartDelay", rec.RestartDelay, time.Millisecond)

		if rec.ResetAfterDays < 0 {
			v.errorf(p+".recovery.resetAfterDays", "must not be negative")
		}

		if rec.ResetAfterDays > math.MaxInt32/secondsPerDay {
			v.errorf(p+".recovery.resetAfterDays", "is too long for sc.exe")
		}

		if rec.ResetAfter != nil {
			if rec.ResetAfterDays != 0 {
				v.errorf(p+".recovery.resetAfter", "set resetAfter or resetAfterDays, not both")
			}

			v.recoveryDuration(p+".recovery.resetAfter", *rec.ResetAfter, time.Second)
		}
	}
}

// secondsPerDay converts resetAfterDays to the seconds sc.exe takes.
const secondsPerDay = 86400

// recoveryDuration checks a delay or reset period that sc.exe takes as a
// whole number of units (milliseconds or seconds) in a 32-bit value.
func (v *validator) recoveryDuration(field string, value Duration, unit time.Duration) {
	switch {
	case value < 0:
		v.errorf(field, "must not be negative")
	case time.Duration(value)%unit != 0:
		v.errorf(field, "must be a whole number of %s", map[time.Duration]string{time.Millisecond: "milliseconds", time.Second: "seconds"}[unit])
	case int64(time.Duration(value)/unit) > math.MaxInt32:
		v.errorf(field, "is too long for sc.exe")
	}
}

func (v *validator) environment(p string, e *Environment, perUser bool) {
	if v.required(p+".name", e.Name) && strings.ContainsAny(e.Name[:1], "=+-!*") {
		v.errorf(p+".name", "write the plain name; action, scope and removeOnUninstall set the prefix")
	}

	v.oneOf(p+".action", e.Action, "set", "setIfAbsent", "append", "prepend", "remove")
	v.oneOf(p+".scope", e.Scope, "machine", "user")

	if e.Scope == "machine" && perUser {
		v.errorf(p+".scope", "a per-user package cannot change machine variables")
	}

	if e.Action != "remove" && e.Value == "" {
		v.errorf(p+".value", "required")
	}

	if strings.Contains(e.Value, "[~]") {
		v.errorf(p+".value", "use action append or prepend instead of [~]")
	}

	if e.Action == "set" && strings.EqualFold(e.Name, "PATH") {
		v.warnf(p+".action", "set replaces the whole PATH; use append or prepend")
	}
}

func (v *validator) properties(cfg *Config) {
	for _, name := range cfg.Properties.Names() {
		prop := cfg.Properties[name]
		p := "properties." + name

		if !IsIdentifier(name) || len(name) > MaxIdentifierLength {
			v.errorf(p, "%q is not an identifier", name)
		}

		if builderProperties[name] {
			v.errorf(p, "%s is set by the builder or the installer; it cannot be defined here", name)
		}

		public := IsPublicProperty(name)
		if prop.Secure != nil && *prop.Secure && !public {
			v.errorf(p+".secure", "only public (upper-case) properties can be secure")
		}
	}
}

func (v *validator) launchConditions(cfg *Config) {
	for i, lc := range cfg.LaunchConditions {
		p := fmt.Sprintf("launchConditions[%d]", i)
		v.required(p+".condition", lc.Condition)
		v.required(p+".message", lc.Message)
	}
}

func (v *validator) customActions(cfg *Config, r *refs) {
	seen := map[string]bool{}

	for i, ca := range cfg.CustomActions {
		p := fmt.Sprintf("customActions[%d]", i)

		if ca.origin != "" {
			v.generatedArgument(ca)

			continue
		}

		if v.required(p+".id", ca.ID) {
			if !IsIdentifier(ca.ID) || len(ca.ID) > MaxIdentifierLength {
				v.errorf(p+".id", "%q is not an identifier", ca.ID)
			}

			if standardActions[ca.ID] {
				v.errorf(p+".id", "%s is a standard action name (ICE93)", ca.ID)
			}

			if seen[ca.ID] {
				v.errorf(p+".id", "custom action %s is defined twice", ca.ID)
			}

			seen[ca.ID] = true
		}

		kinds := 0
		for _, set := range []bool{ca.SetProperty != nil, ca.SetDirectory != nil, ca.Error != "",
			ca.Exec != nil, ca.ExecProperty != nil, ca.DLL != nil} {
			if set {
				kinds++
			}
		}

		if kinds != 1 {
			v.errorf(p, "exactly one of setProperty, setDirectory, error, exec, execProperty or dll is required")
		}

		v.oneOf(p+".execute", ca.Execute, "immediate", "deferred", "rollback", "commit")
		inScript := ca.Execute != "immediate"
		isExec := ca.Exec != nil || ca.ExecProperty != nil

		switch {
		case ca.SetProperty != nil:
			if v.required(p+".setProperty.name", ca.SetProperty.Name) && !IsIdentifier(ca.SetProperty.Name) {
				v.errorf(p+".setProperty.name", "%q is not an identifier", ca.SetProperty.Name)
			}

			if inScript {
				v.errorf(p+".execute", "setProperty cannot be deferred; deferred actions read CustomActionData")
			}
		case ca.SetDirectory != nil:
			if r.dirs[ca.SetDirectory.Directory] == nil {
				v.errorf(p+".setDirectory.directory", "directory %q is not defined", ca.SetDirectory.Directory)
			}

			v.required(p+".setDirectory.value", ca.SetDirectory.Value)

			if inScript {
				v.errorf(p+".execute", "setDirectory cannot be deferred")
			}
		case ca.Error != "":
			if inScript {
				v.errorf(p+".execute", "error actions cannot be deferred")
			}
		case ca.Exec != nil:
			v.required(p+".exec.command", ca.Exec.Command)

			if ca.Exec.Directory != "TARGETDIR" && r.dirs[ca.Exec.Directory] == nil && !standardFolders[ca.Exec.Directory] {
				v.errorf(p+".exec.directory", "directory %q is not defined", ca.Exec.Directory)
			}
		case ca.ExecProperty != nil:
			v.required(p+".execProperty.property", ca.ExecProperty.Property)
		case ca.DLL != nil:
			if _, ok := cfg.Binaries[ca.DLL.Binary]; !ok {
				v.errorf(p+".dll.binary", "binary %q is not defined", ca.DLL.Binary)
			}

			v.required(p+".dll.entry", ca.DLL.Entry)
		}

		if !*ca.Impersonate && !inScript {
			v.errorf(p+".impersonate", "impersonate: false needs a deferred, rollback or commit action")
		}

		if !*ca.Impersonate && cfg.Package.Scope == ScopePerUser {
			v.errorf(p+".impersonate", "a per-user package cannot run actions without impersonation (ICE105)")
		}

		if isExec && ca.Return == "" {
			v.errorf(p+".return", "required for exec actions: check fails the install on a non-zero exit "+
				"code, ignore does not (RUN-18)")
		} else if ca.Return != "" {
			v.oneOf(p+".return", ca.Return, "check", "ignore", "asyncWait", "asyncNoWait")
		}

		if (ca.Return == "asyncWait" || ca.Return == "asyncNoWait") && (!isExec || ca.Execute == "rollback") {
			v.errorf(p+".return", "async needs an exec action that is not a rollback action")
		}

		if ca.Once != "" {
			v.oneOf(p+".once", ca.Once, "firstSequence", "oncePerProcess", "clientRepeat")

			if inScript {
				v.errorf(p+".once", "only for immediate actions")
			}
		}

		if len(ca.Schedule) == 0 {
			v.errorf(p+".schedule", "required: an action that is never scheduled never runs")
		}

		for j, pl := range ca.Schedule {
			v.placement(fmt.Sprintf("%s.schedule[%d]", p, j), pl, inScript, cfg)
		}
	}
}

func (v *validator) placement(p string, pl Placement, inScript bool, cfg *Config) {
	if _, ok := sequenceTables[pl.Sequence]; !ok {
		v.errorf(p+".sequence", "%q is not one of execute, ui, adminExecute, adminUI", pl.Sequence)
	}

	if inScript && (pl.Sequence == "ui" || pl.Sequence == "adminUI") {
		v.errorf(p+".sequence", "deferred actions run only in execute sequences (ICE77)")
	}

	n := 0

	for _, set := range []bool{pl.Before != "", pl.After != "", pl.At != 0} {
		if set {
			n++
		}
	}

	if n != 1 {
		v.errorf(p, "exactly one of before, after or at is required")
	}

	if pl.At != 0 && (pl.At < minSequenceNumber || pl.At > maxSequenceNumber) {
		v.errorf(p+".at", "must be between 1 and %d, or -1 to -4", maxSequenceNumber)
	}

	if pl.At < 0 && pl.Sequence != "ui" && pl.Sequence != "adminUI" {
		v.errorf(p+".at", "termination numbers belong in UI sequences")
	}

	for _, anchor := range []string{pl.Before, pl.After} {
		if anchor == "" || standardActions[anchor] {
			continue
		}

		if !hasCustomAction(cfg, anchor) && !hasDialog(cfg, anchor) {
			v.errorf(p, "anchor %q is neither a standard action nor a custom action", anchor)
		}
	}
}

// generatedArgument checks the action made from a conditional service
// argument at the argument's place in the file; the rest of the action is
// generated and valid by construction.
func (v *validator) generatedArgument(action *CustomAction) {
	if action.SetProperty.Value == "" {
		v.errorf(action.origin+".value", "required")
	}

	if err := condition.Check(action.Schedule[0].Condition); err != nil {
		v.errorf(action.origin+".condition", "%v", err)
	}
}

func hasCustomAction(cfg *Config, id string) bool {
	for _, ca := range cfg.CustomActions {
		if ca.ID == id {
			return true
		}
	}

	return false
}

func hasDialog(cfg *Config, id string) bool {
	for _, d := range cfg.UI.Dialogs {
		if d.ID == id {
			return true
		}
	}

	return false
}

func (v *validator) streams(cfg *Config) {
	check := func(section string, m map[string]string, limit int) {
		for key, src := range m {
			p := section + "." + key

			if !IsIdentifier(key) {
				v.errorf(p, "%q is not an identifier", key)
			}

			if len(key) > limit {
				v.errorf(p, "at most %d characters, so that the stream name fits in %d (DB-11)", limit, MaxStreamNameLength)
			}

			v.required(p, src)
		}
	}

	check("icons", cfg.Icons, maxIconKeyLength)
	check("binaries", cfg.Binaries, maxBinaryKeyLength)
}

func (v *validator) firewall(cfg *Config, r *refs) {
	names := map[string]bool{}

	for i, fw := range cfg.Firewall {
		p := fmt.Sprintf("firewall[%d]", i)

		if v.required(p+".name", fw.Name) {
			if names[fw.Name] {
				v.errorf(p+".name", "rule %q is defined twice", fw.Name)
			}

			names[fw.Name] = true

			if strings.ContainsAny(fw.Name, `"`) {
				v.errorf(p+".name", "must not contain a double quote")
			}
		}

		if v.required(p+".component", fw.Component) && r.components[fw.Component] == nil {
			v.errorf(p+".component", "component %q is not defined", fw.Component)
		}

		v.required(p+".program", fw.Program)
		v.oneOf(p+".protocol", fw.Protocol, "tcp", "udp", "any")

		if fw.Protocol == "any" && fw.Port != "" {
			v.errorf(p+".port", "a port needs protocol tcp or udp")
		}

		for j, prof := range fw.Profiles {
			v.oneOf(fmt.Sprintf("%s.profiles[%d]", p, j), prof, "all", "domain", "private", "public")
		}

		if cfg.Package.Scope == ScopePerUser {
			v.errorf(p, "firewall rules need a per-machine package")
		}
	}
}

func (v *validator) ui(cfg *Config) {
	v.oneOf("ui.wizard", cfg.UI.Wizard, "none", "minimal", "featureTree")

	if cfg.UI.Wizard == "none" && len(cfg.UI.Dialogs) > 0 {
		v.errorf("ui.dialogs", "dialogs need a wizard")
	}

	v.theme(cfg)

	seen := map[string]bool{}

	for i, d := range cfg.UI.Dialogs {
		p := fmt.Sprintf("ui.dialogs[%d]", i)

		if v.required(p+".id", d.ID) {
			if !IsIdentifier(d.ID) {
				v.errorf(p+".id", "%q is not an identifier", d.ID)
			}

			if seen[d.ID] {
				v.errorf(p+".id", "dialog %s is defined twice", d.ID)
			}

			seen[d.ID] = true
		}

		v.required(p+".title", d.Title)
		v.required(p+".after", d.After)

		if len(d.Fields) == 0 {
			v.errorf(p+".fields", "at least one field")
		}

		for j, f := range d.Fields {
			fp := fmt.Sprintf("%s.fields[%d]", p, j)
			if v.required(fp+".property", f.Property) && !IsPublicProperty(f.Property) {
				v.errorf(fp+".property", "UI values reach the server side only through public properties")
			}

			if _, ok := cfg.Properties[f.Property]; f.Property != "" && !ok {
				v.warnf(fp+".property", "%s is not declared in properties, so it is not secure", f.Property)
			}
		}
	}
}

// colorPattern is the #RRGGBB form of the theme colours.
var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (v *validator) theme(cfg *Config) {
	none := cfg.UI.Wizard == "none"

	if none && cfg.UI.License != "" {
		v.errorf("ui.license", "a license page needs a wizard")
	}

	theme := &cfg.UI.Theme
	if none && *theme != (Theme{}) {
		v.errorf("ui.theme", "a theme needs a wizard")
	}

	for _, c := range []struct{ name, value string }{
		{"dark", theme.Colors.Dark}, {"light", theme.Colors.Light}, {"accent", theme.Colors.Accent}, {"paper", theme.Colors.Paper},
	} {
		if c.value != "" && !colorPattern.MatchString(c.value) {
			v.errorf("ui.theme.colors."+c.name, "%q is not a colour in the form #RRGGBB", c.value)
		}
	}
}

func (v *validator) tables(cfg *Config) {
	for i, t := range cfg.Tables {
		p := fmt.Sprintf("tables[%d]", i)

		if v.required(p+".name", t.Name) {
			if !IsIdentifier(t.Name) || len(t.Name) > MaxTableNameLength {
				v.errorf(p+".name", "%q is not an identifier of at most %d characters", t.Name, MaxTableNameLength)
			}

			if standardTables[t.Name] {
				v.errorf(p+".name", "%s is written by the builder", t.Name)
			}
		}

		if len(t.Columns) == 0 || len(t.Columns) > MaxColumns {
			v.errorf(p+".columns", "one to %d columns (DB-07)", MaxColumns)
		}

		keys := 0

		for j, c := range t.Columns {
			cp := fmt.Sprintf("%s.columns[%d]", p, j)

			if v.required(cp+".name", c.Name) && !IsIdentifier(c.Name) {
				v.errorf(cp+".name", "%q is not an identifier", c.Name)
			}

			if !columnTypePattern.MatchString(c.Type) {
				v.errorf(cp+".type", "%q is not a column type such as s72, S255, l0, i2, I4 or v0", c.Type)
			}

			if c.Key {
				keys++
			}
		}

		if keys == 0 && len(t.Columns) > 0 {
			v.errorf(p+".columns", "at least one key column")
		}

		for j, row := range t.Rows {
			v.tableRow(fmt.Sprintf("%s.rows[%d]", p, j), t.Columns, row)
		}
	}
}

func (v *validator) tableRow(p string, cols []Column, row []any) {
	if len(row) != len(cols) {
		v.errorf(p, "%d values for %d columns", len(row), len(cols))

		return
	}

	for k, val := range row {
		c := cols[k]
		if len(c.Type) < 2 {
			continue
		}

		nullable := c.Type[0] >= 'A' && c.Type[0] <= 'Z'
		cp := fmt.Sprintf("%s[%d]", p, k)

		if val == nil {
			if !nullable || c.Key {
				v.errorf(cp, "column %s is not nullable", c.Name)
			}

			continue
		}

		switch c.Type[0] {
		case 'i', 'I':
			n, ok := val.(int)
			if !ok {
				v.errorf(cp, "column %s needs an integer", c.Name)

				continue
			}

			// The type's minimum is stored as NULL (DB-02).
			lo, hi := math.MinInt16+1, math.MaxInt16
			if c.Type[1] == '4' {
				lo, hi = math.MinInt32+1, math.MaxInt32
			}

			if n < lo || n > hi {
				v.errorf(cp, "%d is outside %d…%d", n, lo, hi)
			}
		case 's', 'S', 'l', 'L':
			s, ok := val.(string)
			if !ok {
				v.errorf(cp, "column %s needs a string", c.Name)

				continue
			}

			if size, _ := strconv.Atoi(c.Type[1:]); size > 0 && len([]rune(s)) > size {
				v.errorf(cp, "longer than %d characters", size)
			}

			if s == "" && !nullable {
				v.errorf(cp, "column %s is not nullable and an empty string is NULL", c.Name)
			}
		case 'v', 'V':
			if _, ok := val.(string); !ok {
				v.errorf(cp, "column %s needs a source file path", c.Name)
			}
		}
	}
}

// setProperties lists the properties that a properties entry, a
// setDirectory or setProperty action or a dialog field sets.
func setProperties(cfg *Config) map[string]bool {
	set := make(map[string]bool, len(cfg.Properties))

	for name := range cfg.Properties {
		set[name] = true
	}

	for _, ca := range cfg.CustomActions {
		switch {
		case ca.SetDirectory != nil:
			set[ca.SetDirectory.Directory] = true
		case ca.SetProperty != nil:
			set[ca.SetProperty.Name] = true
		}
	}

	for _, dialog := range cfg.UI.Dialogs {
		for _, field := range dialog.Fields {
			set[field.Property] = true
		}
	}

	return set
}

// movableAncestor returns the first directory from dirID up to (not
// including) its root folder whose path something besides the Directory
// table can set (specification section 8.3): a public id, which the command
// line can set; a configurable one; or one named by a setDirectory or
// setProperty action, a properties entry or a dialog field.
func movableAncestor(cfg *Config, known *refs, dirID string) string {
	set := setProperties(cfg)

	for range 64 {
		parent, ok := known.dirParent[dirID]
		if !ok || parent == "" {
			return ""
		}

		if dir := known.dirs[dirID]; dir != nil && dir.Configurable || set[dirID] || IsPublicProperty(dirID) {
			return dirID
		}

		dirID = parent
	}

	return ""
}
