package config

import (
	"cmp"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"unicode"

	"github.com/jkroepke/msi-builder/internal/condition"
	"github.com/jkroepke/msi-builder/internal/msidb"
)

// Limits checked by Validate. Section references are to docs/spec/MSISPEC.md;
// test IDs to docs/spec/MSISPEC-RESULTS.md.
const (
	MaxIdentifierLength  = 72
	MaxFeatureIDLength   = 38
	MaxFeatureDepth      = 16
	MaxProductNameLength = 63
	MaxStreamNameLength  = 62 // MSIDB-11
	MaxTableNameLength   = 31
	MaxColumns           = 31 // MSIDB-07: the 32nd column is rejected
	maxIconKeyLength     = MaxStreamNameLength - len("Icon.")
	maxBinaryKeyLength   = MaxStreamNameLength - len("Binary.")
	minSequenceNumber    = -4
	maxSequenceNumber    = 32767
)

// cabinetForbidden are the characters a cabinet file name cannot hold.
const cabinetForbidden = `\/:*?"<>|#`

// numberedName is the name of the second cabinet when the first is named
// first, as the builder derives it.
func numberedName(first string) string {
	ext := filepath.Ext(first)

	return strings.TrimSuffix(first, ext) + "2" + ext
}

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

	checker := &validator{}
	known := newRefs(cfg)

	checker.product(cfg)
	checker.pkg(cfg)
	checker.upgrade(cfg)
	checker.directories(cfg, known)
	checker.features(cfg, known)
	checker.components(cfg, known)
	checker.folderComponents(cfg, known)
	checker.shortcuts(cfg, known)
	checker.properties(cfg)
	checker.launchConditions(cfg)
	checker.customActions(cfg, known)
	checker.streams(cfg)
	checker.firewall(cfg, known)
	checker.ui(cfg)
	checker.wizardOptions(cfg, known)
	checker.tables(cfg)

	return checker.diags
}

// refs indexes the ids that other sections refer to.
type refs struct {
	dirs       map[string]*Directory
	dirParent  map[string]string
	components map[string]*Component
	fileIDs    map[string]bool
	fileNames  map[string]bool // long names of files without an id
	inFeature  map[string]bool
}

// hasFile reports whether ref is a File key or the long name of a file
// without an id.
func (r *refs) hasFile(ref string) bool {
	return r.fileIDs[ref] || r.fileNames[ref]
}

func newRefs(cfg *Config) *refs {
	known := &refs{
		dirs:       make(map[string]*Directory),
		dirParent:  make(map[string]string),
		components: make(map[string]*Component),
		fileIDs:    make(map[string]bool),
		fileNames:  make(map[string]bool),
		inFeature:  make(map[string]bool),
	}

	var walk func([]*Directory, string)

	walk = func(ds []*Directory, parent string) {
		for _, dir := range ds {
			if _, dup := known.dirs[dir.ID]; !dup {
				known.dirs[dir.ID] = dir
				known.dirParent[dir.ID] = parent
			}

			walk(dir.Children, dir.ID)
		}
	}
	walk(cfg.Directories, "")

	for _, c := range cfg.Components {
		known.components[c.ID] = c

		for _, f := range c.Files {
			known.fileIDs[fileID(f)] = true

			if f.ID == "" {
				known.fileNames[fileName(f)] = true
			}
		}
	}

	eachFeature(cfg.Features, func(f *Feature, _ int) {
		for _, c := range f.Components {
			known.inFeature[c.ID] = true
		}
	})

	return known
}

// fileName returns the long name of a file.
func fileName(f *File) string {
	if f.Name != "" {
		return f.Name
	}

	return path.Base(strings.ReplaceAll(f.Source, `\`, "/"))
}

// FileByRef returns the file that ref names: its key, or the long name of a
// file without an id, so that "fleetspeak-client.exe" finds the file keyed
// "fleetspeak_client.exe". It returns nil when no file matches.
func FileByRef(files []*File, ref string) *File {
	for _, f := range files {
		if fileID(f) == ref || f.ID == "" && fileName(f) == ref {
			return f
		}
	}

	return nil
}

// fileID returns the File key: the explicit id, else the long name, with
// every character that an identifier cannot hold replaced by "_" (so
// "fleetspeak-client.exe" becomes "fleetspeak_client.exe").
func fileID(f *File) string {
	if f.ID != "" {
		return f.ID
	}

	name := fileName(f)
	if IsIdentifier(name) && len(name) <= MaxIdentifierLength {
		return name
	}

	return identifier(name)
}

// topFolder returns the standard folder at the root of a directory's chain.
func (r *refs) topFolder(dirID string) string {
	for range 64 {
		parent, ok := r.dirParent[dirID]
		if !ok || parent == "" {
			return dirID
		}

		dirID = parent
	}

	return dirID
}

func (v *validator) product(cfg *Config) { //nolint:cyclop // a flat list of independent checks
	product := cfg.Product

	if v.required(productNamePath, product.Name) && len([]rune(product.Name)) > MaxProductNameLength {
		v.errorf(productNamePath, "at most %d characters", MaxProductNameLength)
	}

	v.required("product.manufacturer", product.Manufacturer)

	if v.required("product.version", product.Version) {
		ver, err := ParseVersion(product.Version)
		if err != nil {
			v.errorf("product.version", "%v", err)
		} else if ver.Fields == 4 {
			v.warnf("product.version", "Windows Installer compares only the first three fields; "+
				"releases that differ in the fourth are the same version to upgrades (UPG-01)")
		}
	}

	if *product.Language < 0 || *product.Language > 0xFFFF {
		v.errorf("product.language", "%d is not an LCID", *product.Language)
	}

	v.upgradeCode("product.upgradeCode", product.UpgradeCode)

	if product.ProductCode != Auto {
		v.guid("product.productCode", product.ProductCode)
		v.warnf("product.productCode", "a fixed ProductCode makes every rebuild of this version fail "+
			"with 1638 over an installed copy (UPG-03); use auto unless this is a minor upgrade")
	}

	if product.PackageCode != Auto {
		v.guid("product.packageCode", product.PackageCode)
	}

	if product.ARP.Icon != "" {
		if _, ok := cfg.Icons[product.ARP.Icon]; !ok {
			v.errorf("product.arp.icon", "icon %q is not defined in icons", product.ARP.Icon)
		}
	}

	if product.ARP.Size < 0 {
		v.errorf("product.arp.size", "must not be negative")
	}
}

func (v *validator) guid(fieldPath, s string) {
	if _, err := NormalizeGUID(s); err != nil {
		v.errorf(fieldPath, "%v", err)
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

func (v *validator) pkg(cfg *Config) { //nolint:cyclop // a flat list of independent checks
	pkg := cfg.Package

	if pkg.Platform == "" {
		v.errorf("package.platform", "required: x86, x64 or arm64")
	}

	pages, err := PageCount(pkg.InstallerVersion)

	switch {
	case err != nil:
		v.errorf("package.installerVersion", "%v", err)
	case pages > MaxPageCount:
		v.errorf("package.installerVersion", "%s is above 5.0; Windows Installer 5.0 refuses such packages with 1613 (SUM-03)",
			pkg.InstallerVersion)
	case pkg.Platform != "" && pages < MinPageCount(pkg.Platform):
		v.errorf("package.installerVersion", "%s needs at least %d.%d", pkg.Platform,
			MinPageCount(pkg.Platform)/100, MinPageCount(pkg.Platform)%100)
	}

	if pkg.Codepage != 65001 && pkg.Codepage != 0 && !ansiCodepages[pkg.Codepage] {
		v.errorf("package.codepage", "%d is not supported; use an ANSI code page or 65001", pkg.Codepage)
	}

	v.oneOf("package.compression", pkg.Compression, Compressions()...)

	switch {
	case !plainFileName(pkg.Cabinet):
		v.errorf("package.cabinet", "%q is not a plain file name", pkg.Cabinet)
	// Further cabinets insert their number before the extension, so a
	// base name of COM or LPT turns into the device COM2.
	case !pkg.CabinetsEmbedded() && (!validLongName(pkg.Cabinet) || !validLongName(numberedName(pkg.Cabinet))):
		v.errorf("package.cabinet", "%q cannot be a file name on Windows (a device name, a trailing dot or space, or . or ..)", pkg.Cabinet)
	case !pkg.CabinetsEmbedded():
		// An external cabinet is a file beside the package, not a stream.
	case msidb.CheckStreamName(pkg.Cabinet) != nil:
		v.errorf("package.cabinet", "not a valid stream name (MSIDB-11): %v", msidb.CheckStreamName(pkg.Cabinet))
	case !IsIdentifier(pkg.Cabinet):
		v.warnf("package.cabinet", "ICE03 reports bad cabinet syntax: an embedded cabinet name must be an identifier")
	}
}

func (v *validator) upgrade(cfg *Config) { //nolint:cyclop // a flat list of independent checks
	upg := cfg.Upgrade
	if upg == nil || upg.Disabled {
		return
	}

	v.oneOf("upgrade.downgrade", upg.Downgrade, "error", "allow", "ignore")
	v.oneOf("upgrade.removeOld", upg.RemoveOld,
		"afterInstallValidate", "afterInstallInitialize", "afterInstallExecute", "afterInstallFinalize")

	seen := make(map[string]string)

	actionProperty := func(fieldPath, name string) {
		if !v.required(fieldPath, name) {
			return
		}

		if !IsIdentifier(name) || !IsPublicProperty(name) {
			v.errorf(fieldPath, "%q must be an upper-case identifier, so that it is public", name)
		}

		if other, dup := seen[name]; dup {
			v.errorf(fieldPath, "%s is already used by %s", name, other)
		}

		seen[name] = fieldPath

		if _, ok := cfg.Properties[name]; ok {
			v.errorf(fieldPath, "%s must not be defined in properties (ICE61)", name)
		}
	}

	actionProperty("upgrade.property", upg.Property)

	if upg.Downgrade != "allow" {
		actionProperty("upgrade.newerProperty", upg.NewerProperty)
	}

	for i, rel := range upg.Related {
		relPath := fmt.Sprintf("upgrade.related[%d]", i)

		v.upgradeCode(relPath+".upgradeCode", rel.UpgradeCode)

		if rel.MinVersion == "" && rel.MaxVersion == "" {
			v.errorf(relPath, "minVersion or maxVersion is required")
		}

		for _, check := range []struct{ path, val string }{{relPath + ".minVersion", rel.MinVersion}, {relPath + ".maxVersion", rel.MaxVersion}} {
			if check.val == "" {
				continue
			}

			if _, err := ParseVersion(check.val); err != nil {
				v.errorf(check.path, "%v", err)
			}
		}

		actionProperty(relPath+".property", rel.Property)
	}
}

func (v *validator) directories(cfg *Config, known *refs) { //nolint:cyclop,gocognit // a flat list of independent checks
	seen := make(map[string]bool)

	var walk func([]*Directory, string, bool)

	walk = func(ds []*Directory, prefix string, top bool) {
		names := make(map[string]string)

		for i, dir := range ds {
			dirPath := fmt.Sprintf("%s[%d]", prefix, i)

			v.sameFolder(names, dirPath, dir, top)

			if v.required(dirPath+".id", dir.ID) {
				if !IsIdentifier(dir.ID) || len(dir.ID) > MaxIdentifierLength {
					v.errorf(dirPath+".id", "%q is not an identifier of at most %d characters", dir.ID, MaxIdentifierLength)
				}

				if seen[dir.ID] {
					v.errorf(dirPath+".id", "directory %s is defined twice", dir.ID)
				}

				seen[dir.ID] = true
			}

			problem := nameProblem(dir.Name, "folder name")

			switch {
			case top && !standardFolders[dir.ID]:
				v.errorf(dirPath+".id", "a top-level directory must be a standard folder such as ProgramFiles64Folder")
			case top && dir.Name != "":
				v.errorf(dirPath+".name", "a standard folder has no name")
			case !top && !v.required(dirPath+".name", dir.Name):
			case !top && problem != "" && dir.origin == productNamePath:
				v.errorf(dir.origin, "%q cannot be used as a folder name; set installDir.name", dir.Name)
			case !top && problem != "" && dir.origin != "":
				v.errorf(dir.origin, "%s", problem)
			case !top && problem != "":
				v.errorf(dirPath+".name", "%s", problem)
			}

			if dir.Configurable && !IsPublicProperty(dir.ID) {
				v.errorf(dirPath+".id", "a configurable directory needs an upper-case id")
			}

			walk(dir.Children, dirPath+".children", false)
		}
	}

	walk(cfg.Directories, "directories", true)

	if install := cfg.InstallDir; known.dirs[install.ID] == nil {
		switch {
		case !IsIdentifier(install.ID) || !IsPublicProperty(install.ID):
			v.errorf("installDir.id", "%q must be an upper-case identifier, so that it can be set on the command line", install.ID)
		case !standardFolders[install.Parent]:
			v.errorf("installDir.parent", "%q is not a standard folder such as ProgramFiles64Folder", install.Parent)
		case len(splitPath(install.Name)) == 0:
			v.errorf("installDir.name", "required")
		}
	}

	v.rememberComponent(cfg, known)

	if cfg.Product.ARP.InstallLocation != "" && known.dirs[cfg.Product.ARP.InstallLocation] == nil {
		v.errorf("product.arp.installLocation", "directory %q is not defined", cfg.Product.ARP.InstallLocation)
	}
}

// rememberComponent checks installDir.remember: the install dir must be
// public and configurable, and no component or action may take the ID of
// the one it adds.
func (v *validator) rememberComponent(cfg *Config, known *refs) {
	if !cfg.InstallDir.Remember {
		return
	}

	// SEARCH-03 only covers a public, configurable install dir: the command
	// line must be able to set it, and AppSearch and secure properties must
	// be public (ICE52).
	if dir := known.dirs[cfg.InstallDir.ID]; dir != nil && (!dir.Configurable || !IsPublicProperty(dir.ID)) {
		v.errorf("installDir.remember", "%s must be a configurable directory with an upper-case id", dir.ID)
	}

	v.rememberIDs(cfg)
}

// rememberIDs rejects a component or action that takes the ID of the one
// installDir.remember adds.
func (v *validator) rememberIDs(cfg *Config) {
	for i, action := range cfg.CustomActions {
		if action != nil && action.ID == RememberActionID {
			v.errorf(fmt.Sprintf("customActions[%d].id", i), "%s is the action installDir.remember adds; rename this one", action.ID)
		}
	}

	for i, comp := range cfg.Components {
		if comp != nil && comp.ID == RememberComponentID && comp.origin != rememberOrigin {
			v.errorf(comp.Path(i)+".id", "%s is the component installDir.remember adds; rename this one", comp.ID)
		}
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

	long := strings.ToLower(dir.Name)
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

func (v *validator) features(cfg *Config, known *refs) {
	seen := make(map[string]bool)

	var walk func([]*Feature, string, int)

	walk = func(fs []*Feature, prefix string, depth int) {
		for i, feature := range fs {
			featurePath := fmt.Sprintf("%s[%d]", prefix, i)

			v.featureID(featurePath, feature.ID, seen)

			if depth > MaxFeatureDepth {
				v.errorf(featurePath, "features nest at most %d deep", MaxFeatureDepth)
			}

			if *feature.Level < 0 || *feature.Level > maxSequenceNumber {
				v.errorf(featurePath+".level", "must be between 0 and %d", maxSequenceNumber)
			}

			v.oneOf(featurePath+".display", feature.Display, "expanded", "collapsed", "hidden")
			v.featureDefault(featurePath, feature)

			v.featureRefs(featurePath, feature, known)

			walk(feature.Features, featurePath+".features", depth+1)
		}
	}

	walk(cfg.Features, "features", 1)

	if len(cfg.Components) > 0 && len(cfg.Features) == 0 {
		v.errorf("features", "at least one feature is required")
	}
}

// featureDefault checks default, which replaces level.
func (v *validator) featureDefault(featurePath string, feature *Feature) {
	if feature.Default == "" {
		return
	}

	v.oneOf(featurePath+".default", feature.Default, FeatureOn, FeatureOff, FeatureDisabled)

	if !feature.levelIsDefault() {
		v.errorf(featurePath+".default", "set default or level, not both")
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

// preserve checks components that keep what the user changed. UPG-05: with
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
	// UPG-05 ran the early removal right after InstallValidate (1401).
	// afterInstallInitialize also removes the old product before the new one
	// installs its files, so it is treated the same (not tested).
	early := removeOld == "afterInstallValidate" || removeOld == "afterInstallInitialize"

	switch {
	case comp.Preserve && early:
		// Also with permanent: UPG-05 lost the edit when only the new
		// version had both flags.
		v.errorf(field+".preserve", "needs removeOld afterInstallExecute: %s removes the old product first, "+
			"which deleted the preserved file and registry value in UPG-05 (tested right after InstallValidate)", removeOld)
	case comp.Preserve && removeOld != PreserveRemoveOld:
		v.warnf(field+".preserve", "only removeOld afterInstallExecute is tested to keep preserved files and "+
			"registry values (UPG-05); %s is not", removeOld)
	case comp.NeverOverwrite && !comp.Permanent && early:
		hint := "use preserve: true"
		if len(comp.Services) > 0 {
			hint = "move the service to its own component and use preserve: true"
		}

		v.warnf(field+".neverOverwrite", "removeOld %s removes the old product first, which deleted a neverOverwrite "+
			"component's file and registry value in UPG-05 (tested right after InstallValidate); %s", removeOld, hint)
	}
}

// directoryPaths maps each directory id to its default target path: the
// root id and the lower-case long names below it. This is the path that
// automatic component GUIDs are derived from, so two ids with the same path
// install to the same folder.
func directoryPaths(dirs []*Directory, parent string, out map[string]string) map[string]string {
	for _, dir := range dirs {
		dirPath := dir.ID

		if parent != "" {
			dirPath = parent + `\` + strings.ToLower(dir.Name)
		}

		out[dir.ID] = dirPath
		directoryPaths(dir.Children, dirPath, out)
	}

	return out
}

func (v *validator) components(cfg *Config, known *refs) {
	seen := make(map[string]bool)
	fileIDs := make(map[string]string)
	paths := newFolderPaths(cfg.Directories)
	regIDs := make(map[string]bool)
	removeIDs := make(map[string]bool)
	removeFileIDs := make(map[string]bool)
	perUser := cfg.Package.Scope == ScopePerUser

	for i, comp := range cfg.Components {
		p := comp.Path(i) //nolint:varnamelen // the YAML path, named p throughout this file

		if comp.ref {
			v.errorf(p, "%q: a component here is a mapping; ids only refer to components from features", comp.ID)

			continue
		}

		v.componentID(p, comp, seen, known)
		v.componentDirectory(cfg, p, comp, known)

		if folderComponent(comp) {
			v.folderComponent(cfg, p, comp)

			continue
		}

		v.componentGUID(p, comp)
		v.filePatterns(p, comp)

		v.oneOf(p+".bitness", comp.Bitness, Auto, "32", "64")

		if !preserveReported(comp) {
			v.preserve(cfg, p, comp)
		}

		v.purge(cfg, known, p, comp)

		if comp.Bitness == "64" && !cfg.Package.Platform.Is64Bit() {
			v.errorf(p+".bitness", "64-bit components need an x64 or arm64 package")
		}

		files := v.componentFiles(cfg, p, comp, fileIDs, paths)
		v.componentRegistry(p, comp, regIDs, perUser)
		v.componentRemoveRegistry(p, comp, removeIDs, perUser)
		v.componentRemoveFiles(cfg, p, comp, known, removeFileIDs)
		keyFile := v.componentKeyFile(p, comp, files)

		for j, svc := range comp.Services {
			svcPath := fmt.Sprintf("%s.services[%d]", p, j)
			v.service(svcPath, svc, keyFile, perUser)
			v.permissions(svcPath+".permissions", svc.Permissions)
		}

		v.componentPermissions(p, comp)
		v.componentEvents(p, comp, perUser)
	}
}

// componentID checks a component's id and records it in seen.
func (v *validator) componentID(compPath string, comp *Component, seen map[string]bool, known *refs) {
	if !v.required(compPath+".id", comp.ID) {
		return
	}

	if !IsIdentifier(comp.ID) || len(comp.ID) > MaxIdentifierLength {
		v.errorf(compPath+".id", "%q is not an identifier of at most %d characters", comp.ID, MaxIdentifierLength)
	}

	if seen[comp.ID] {
		v.errorf(compPath+".id", "component %s is defined twice", comp.ID)
	}

	seen[comp.ID] = true

	if !known.inFeature[comp.ID] {
		v.errorf(compPath, "component %s is in no feature (ICE21)", comp.ID)
	}
}

// componentDirectory checks a component's directory and where it lies.
func (v *validator) componentDirectory(cfg *Config, compPath string, comp *Component, known *refs) { //nolint:cyclop // a flat list of independent checks
	if !v.required(compPath+".directory", comp.Directory) {
		return
	}

	if known.dirs[comp.Directory] == nil {
		// An invalid installDir is reported there.
		if comp.Directory != cfg.InstallDir.ID || validInstallDir(cfg.InstallDir) {
			v.errorf(compPath+".directory", "directory %q is not defined", comp.Directory)
		}

		return
	}

	perUser := cfg.Package.Scope == ScopePerUser
	top := known.topFolder(comp.Directory)

	// The component of a folder source reports these once, not once per
	// file.
	if directoryReported(known, comp) {
		return
	}

	if cfg.Package.Platform.Is64Bit() && comp.Bitness != "32" && folders32[top] {
		v.warnf(compPath+".directory", "%s is the 32-bit folder on 64-bit Windows (ICE80); "+
			"use its 64-bit counterpart, or bitness: 32 for 32-bit files", top)
	}

	if perUser && perMachineFolders[top] {
		v.errorf(compPath+".directory", "a per-user package cannot install under %s (ICE105)", top)
	}

	switch {
	case !profileFolders[top]:
	case !userRegistryKeyPath(comp, perUser):
		v.warnf(compPath+".directory", "a component in the user profile (%s) needs an HKCU registry value "+
			"as key path (ICE38)", top)
	}
}

// componentGUID checks a component's GUID.
func (v *validator) componentGUID(compPath string, comp *Component) {
	switch comp.GUID {
	case Auto:
	case None:
		if !comp.Permanent {
			v.errorf(compPath+".guid", "none is only allowed with permanent: true; "+
				"a component without a GUID is never removed")
		}
	default:
		v.guid(compPath+".guid", comp.GUID)
	}
}

// componentKeyFile checks keyPath and returns the component's key file, if
// its key path is a file. keyFile is the file componentFiles picked.
func (v *validator) componentKeyFile(compPath string, comp *Component, keyFile *File) *File { //nolint:cyclop // a flat list of independent checks
	switch keyPath := comp.KeyPath; {
	case keyPath == "" || keyPath == KeyPathDirectory:
	case keyFileByRef(comp, keyPath) != nil:
		keyFile = keyFileByRef(comp, keyPath)
	case hasRegistryID(comp, keyPath):
		keyFile = nil
	default:
		v.errorf(compPath+".keyPath", "%q is neither a file nor a registry id of this component", keyPath)
	}

	if comp.KeyPath == "" && len(comp.Files) == 0 {
		keyFile = nil
	}

	if keyFile != nil && keyFile.Version == None && len(comp.Files) > 1 {
		v.warnf(compPath+".keyPath", "the key file is unversioned; other files of the component are only "+
			"replaced together with it")
	}

	return keyFile
}

// componentPermissions checks the permissions of a component's files,
// registry values and folder.
func (v *validator) componentPermissions(compPath string, comp *Component) {
	for j, file := range comp.Files {
		v.permissions(fmt.Sprintf("%s.files[%d].permissions", compPath, j), file.Permissions)
	}

	for j, value := range comp.Registry {
		v.permissions(fmt.Sprintf("%s.registry[%d].permissions", compPath, j), value.Permissions)
	}

	directoryKey := comp.KeyPath == KeyPathDirectory || comp.KeyPath == "" && len(comp.Files) == 0 && len(comp.Registry) == 0
	if v.permissions(compPath+".folderPermissions", comp.FolderPermissions) && !comp.CreateFolder && !directoryKey {
		v.errorf(compPath+".folderPermissions", "the component creates no folder row; set createFolder: true")
	}
}

// componentEvents checks a component's event log sources and environment
// variables.
func (v *validator) componentEvents(compPath string, comp *Component, perUser bool) {
	for j, source := range comp.EventLog {
		sourcePath := fmt.Sprintf("%s.eventLog[%d]", compPath, j)
		v.required(sourcePath+".source", source.Source)
		v.required(sourcePath+".messageFile", source.MessageFile)

		if perUser {
			v.errorf(sourcePath, "event log sources are per-machine (ICE105)")
		}
	}

	// The builder keys an Environment row by name, action and scope, so a
	// second entry with all three equal would be a duplicate key. Names are
	// compared without case to be safe; how the engine treats FOO and foo
	// in two Environment rows is not tested.
	seen := make(map[[3]string]bool)

	for j, env := range comp.Environment {
		envPath := fmt.Sprintf("%s.environment[%d]", compPath, j)
		v.environment(envPath, env, perUser)

		key := [3]string{strings.ToUpper(env.Name), env.Action, env.Scope}
		if env.Name != "" && seen[key] {
			v.errorf(envPath, "%s with action %s and scope %s is set twice in this component", env.Name, env.Action, env.Scope)
		}

		seen[key] = true
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
	case comp.KeyPath == "" || comp.KeyPath == KeyPathDirectory:
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

// folderPaths holds the default target paths of the directories and the
// files installed to them so far.
type folderPaths struct {
	byID   map[string]string          // directory id to path
	byPath map[string]string          // path to the smallest directory id with it
	files  map[string][]installedFile // file path to the files installed there
}

// installedFile is a file of a component, by its config path.
type installedFile struct {
	comp *Component
	path string
}

func newFolderPaths(dirs []*Directory) *folderPaths {
	byID := directoryPaths(dirs, "", make(map[string]string))
	byPath := make(map[string]string, len(byID))

	for id, dirPath := range byID {
		if other, ok := byPath[dirPath]; !ok || id < other {
			byPath[dirPath] = id
		}
	}

	return &folderPaths{byID: byID, byPath: byPath, files: make(map[string][]installedFile)}
}

// uniqueTarget reports a file whose target path, resolved through the
// directory tree, another file already installs to or a folder already has.
// Two conditional components in different directory ids only get a warning,
// as in ICE30, which also warns for one directory id: their conditions may
// never both be true. The builder rejects them if they end up with one
// ComponentId.
func (v *validator) uniqueTarget(filePath string, comp *Component, name string, paths *folderPaths) {
	dirPath, ok := paths.byID[comp.Directory]
	if !ok {
		dirPath = comp.Directory
	}

	target := dirPath + `\` + strings.ToLower(name)
	if folder, ok := paths.byPath[target]; ok {
		v.errorf(filePath, "%s has the same path as directory %s", name, folder)
	}

	earlier := paths.files[target]
	paths.files[target] = append(earlier, installedFile{comp: comp, path: filePath})

	for _, other := range earlier {
		if !exclusiveComponents(comp, other.comp) {
			v.errorf(filePath, "%s installs the same file as %s", name, other.path)

			return
		}
	}

	if len(earlier) > 0 {
		v.warnf(filePath, "%s installs the same file as %s; their conditions must never both be true (ICE30)", name, earlier[0].path)
	}
}

// exclusiveComponents reports two components that may install one file
// path: both are conditional and use different directory ids.
func exclusiveComponents(a, b *Component) bool {
	return a.Directory != b.Directory && a.Condition != "" && b.Condition != ""
}

// componentFiles validates the files of c and returns its default key file.
//
//nolint:cyclop // flat checks
func (v *validator) componentFiles(cfg *Config, compPath string, comp *Component, ids map[string]string, paths *folderPaths) *File {
	versioned := 0

	for j, file := range comp.Files {
		filePath := fmt.Sprintf("%s.files[%d]", compPath, j)

		v.required(filePath+".source", file.Source)

		name, nameField := file.Name, filePath+".name"
		if name == "" {
			name, nameField = fileName(file), filePath+".source"
		}

		if problem := nameProblem(name, "file name"); problem != "" {
			v.errorf(nameField, "%s", problem)
		} else if !*cfg.Package.LongFileNames && !isShortName(name) {
			v.errorf(nameField, "longFileNames is false, so %q must be a valid 8.3 name", name)
		}

		fileKey := fileID(file)
		if !IsIdentifier(fileKey) || len(fileKey) > MaxIdentifierLength {
			v.errorf(filePath+".id", "%q is not an identifier of at most %d characters", fileKey, MaxIdentifierLength)
		}

		if other, dup := ids[fileKey]; dup {
			v.errorf(filePath+".id", "file key %q is also used by %s; set id on one of them", fileKey, other)
		}

		ids[fileKey] = filePath

		v.uniqueTarget(filePath, comp, fileName(file), paths)

		v.fileVersion(filePath, file, comp)

		if versionedFile(file) {
			versioned++
		}

		switch file.Language {
		case Auto, None:
		default:
			for lcid := range strings.SplitSeq(file.Language, ",") {
				if n, err := strconv.Atoi(lcid); err != nil || n < 0 || n > 0xFFFF {
					v.errorf(filePath+".language", "%q is not auto, none or a comma-separated LCID list", file.Language)

					break
				}
			}
		}
	}

	if comp.GUID == Auto && versioned > 1 && len(comp.Files) > 1 {
		v.warnf(compPath+".files", "more than one versioned file in a component: only the key file decides "+
			"whether the others are replaced")
	}

	if len(comp.Files) > 0 {
		return comp.Files[0]
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

// fileVersion enforces the FILE-01 rule: a PE file is never unversioned by
// accident, since an upgrade with early removal then deletes it.
func (v *validator) fileVersion(filePath string, file *File, comp *Component) {
	isPE := peExtensions[strings.ToLower(path.Ext(fileName(file)))]

	switch file.Version {
	case Auto:
	case None:
		if isPE && !file.AllowUnversionedExecutable {
			v.errorf(filePath+".version", "%s is an executable; without a version an upgrade can delete it "+
				"(FILE-01). Set allowUnversionedExecutable: true if this is intended", fileName(file))
		} else if isPE {
			v.warnf(filePath+".version", "unversioned executable: an upgrade with removeOld before "+
				"InstallFinalize deletes it when the installed copy was versioned (FILE-01)")
		}
	default:
		if _, err := ParseVersion(file.Version); err == nil {
			return
		}

		if other := FileByRef(comp.Files, file.Version); other == nil || other == file {
			v.errorf(filePath+".version", "%q is not auto, none, a version or the id or name of another file in this component", file.Version)
		}
	}
}

func (v *validator) componentRegistry(p string, c *Component, ids map[string]bool, perUser bool) { //nolint:cyclop,gocognit // a flat list of independent checks
	for j, value := range c.Registry {
		valuePath := fmt.Sprintf("%s.registry[%d]", p, j)

		if value.ID != "" {
			if !IsIdentifier(value.ID) {
				v.errorf(valuePath+".id", "%q is not an identifier", value.ID)
			}

			if ids[value.ID] {
				v.errorf(valuePath+".id", "registry id %s is defined twice", value.ID)
			}

			ids[value.ID] = true
		}

		v.registryLocation(valuePath, value.Root, value.Key, perUser)

		v.oneOf(valuePath+".type", value.Type, "string", "expandString", "multiString", "dword", "binary", "key")

		switch value.Type {
		case "key":
			if len(value.Value) > 0 || value.Name != "" {
				v.errorf(valuePath, "type key takes neither name nor value")
			}
		case "multiString":
			if len(value.Value) == 0 {
				v.errorf(valuePath+".value", "required")
			}
		default:
			if len(value.Value) != 1 {
				v.errorf(valuePath+".value", "exactly one value is required for type %s", value.Type)

				continue
			}

			v.registryScalar(valuePath+".value", value.Type, value.Value[0])
		}

		if (value.Append || value.Prepend) && value.Type != "multiString" {
			v.errorf(valuePath, "append and prepend need type multiString")
		}

		if value.Append && value.Prepend {
			v.errorf(valuePath, "append and prepend are exclusive")
		}

		if value.RemoveOnUninstall && value.Type != "key" {
			v.errorf(valuePath+".removeOnUninstall", "only for type key; values are always removed with their component")
		}
	}
}

// registryLocation checks the root and key of a registry entry.
func (v *validator) registryLocation(entryPath, root, key string, perUser bool) {
	v.oneOf(entryPath+".root", root, "HKLM", "HKCU", "HKCR", "HKU", "HKMU")

	if perUser && (root == "HKLM" || root == "HKCR") {
		v.errorf(entryPath+".root", "a per-user package cannot change %s (ICE105)", root)
	}

	v.required(entryPath+".key", key)

	if strings.HasPrefix(key, `\`) || strings.HasSuffix(key, `\`) {
		v.errorf(entryPath+".key", "no leading or trailing backslash")
	}
}

// componentRemoveRegistry checks the registry keys and values a component
// deletes when it is installed.
func (v *validator) componentRemoveRegistry(p string, c *Component, ids map[string]bool, perUser bool) {
	seen := make(map[[3]string]bool)

	for j, entry := range c.RemoveRegistry {
		entryPath := fmt.Sprintf("%s.removeRegistry[%d]", p, j)

		if entry.ID != "" {
			if !IsIdentifier(entry.ID) || len(entry.ID) > MaxIdentifierLength {
				v.errorf(entryPath+".id", "%q is not an identifier of at most %d characters", entry.ID, MaxIdentifierLength)
			}

			if ids[entry.ID] {
				v.errorf(entryPath+".id", "removeRegistry id %s is defined twice", entry.ID)
			}

			ids[entry.ID] = true
		}

		v.registryLocation(entryPath, entry.Root, entry.Key, perUser)

		if entry.Name == "-" {
			v.errorf(entryPath+".name", "leave name out to delete the whole key")
		}

		key := [3]string{entry.Root, strings.ToLower(entry.Key), strings.ToLower(entry.Name)}
		if seen[key] {
			v.errorf(entryPath, "this key or value is already removed by this component")
		}

		seen[key] = true
	}
}

// componentRemoveFiles checks the files and folders a component deletes.
func (v *validator) componentRemoveFiles(cfg *Config, p string, comp *Component, known *refs, ids map[string]bool) {
	seen := make(map[[3]string]bool)

	for j, entry := range comp.RemoveFiles {
		entryPath := fmt.Sprintf("%s.removeFiles[%d]", p, j)

		if entry.ID != "" {
			if !IsIdentifier(entry.ID) || len(entry.ID) > MaxIdentifierLength {
				v.errorf(entryPath+".id", "%q is not an identifier of at most %d characters", entry.ID, MaxIdentifierLength)
			}

			if ids[entry.ID] {
				v.errorf(entryPath+".id", "removeFiles id %s is defined twice", entry.ID)
			}

			ids[entry.ID] = true
		}

		v.oneOf(entryPath+".on", entry.On, RemoveOnInstall, RemoveOnUninstall, RemoveOnBoth)

		row := [3]string{cmp.Or(entry.Directory, comp.Directory), strings.ToLower(entry.Name), entry.On}
		if seen[row] {
			v.errorf(entryPath, "this component already removes %q there", entry.Name)
		}

		seen[row] = true

		if entry.Directory != "" && known.dirs[entry.Directory] == nil {
			v.errorf(entryPath+".directory", "directory %q is not defined", entry.Directory)
		}

		v.removeFilePattern(entryPath+".name", entry.Name, *cfg.Package.LongFileNames)
	}
}

// maxRemoveFilePattern leaves room in the 255 characters of
// RemoveFile.FileName for a generated "xxxxxxxx.xxx|" short name part.
const maxRemoveFilePattern = 255 - len("xxxxxxxx.xxx|")

// removeFilePattern checks a RemoveFile pattern. The engine matches * and ?
// itself; the rest must be a filename.
func (v *validator) removeFilePattern(field, pattern string, longNames bool) {
	plain := strings.NewReplacer("*", "x", "?", "x").Replace(pattern)

	switch {
	case pattern == "":
	case strings.Contains(pattern, "|"):
		v.errorf(field, "%q has a short name part; short names are generated, so give only the long name", pattern)
	case !validLongName(plain):
		v.errorf(field, "%q is not a filename pattern; a pattern matches files of one folder only", pattern)
	case !longNames && !isShortName(plain):
		v.errorf(field, "longFileNames is false, so %q must be a valid 8.3 pattern", pattern)
	case len(pattern) > maxRemoveFilePattern:
		v.errorf(field, "%q is longer than %d bytes; with its short name part it must fit RemoveFile.FileName (255)", pattern, maxRemoveFilePattern)
	}
}

func (v *validator) registryScalar(fieldPath, typ, val string) { //nolint:cyclop // a flat list of independent checks
	switch typ {
	case "dword":
		if strings.Contains(val, "[") {
			return
		}

		n, err := strconv.ParseInt(val, 0, 64)
		if err != nil || n < math.MinInt32 || n > math.MaxUint32 {
			v.errorf(fieldPath, "%q is not a 32-bit integer", val)
		}
	case "binary":
		h := strings.TrimPrefix(strings.ToLower(val), "0x")
		if h == "" || strings.Trim(h, "0123456789abcdef") != "" {
			v.errorf(fieldPath, "%q is not hexadecimal", val)
		}
	case "string", "expandString":
		if strings.Contains(val, "[~]") {
			v.errorf(fieldPath, "[~] makes the value REG_MULTI_SZ; use type multiString")
		}
	}
}

func (v *validator) service(svcPath string, svc *Service, keyFile *File, perUser bool) {
	if perUser {
		v.errorf(svcPath, "a per-user package cannot install services (ICE105)")
	}

	if keyFile == nil {
		v.errorf(svcPath, "a service needs its executable as the component's key file")
	} else if !strings.EqualFold(path.Ext(fileName(keyFile)), ".exe") {
		v.errorf(svcPath, "the key file %s is not an .exe", fileName(keyFile))
	}

	v.required(svcPath+".name", svc.Name)
	v.oneOf(svcPath+".type", svc.Type, "ownProcess", "shareProcess")
	v.oneOf(svcPath+".start", svc.Start, "auto", "demand", "disabled")
	v.oneOf(svcPath+".errorControl", svc.ErrorControl, "ignore", "normal", "critical")

	if svc.DelayedAutoStart && svc.Start != "auto" {
		v.errorf(svcPath+".delayedAutoStart", "needs start: auto")
	}

	if svc.Password != "" && !strings.Contains(svc.Password, "[") {
		v.errorf(svcPath+".password", "use a hidden property such as [SERVICE_PASSWORD], never a literal password")
	}

	v.serviceControl(svcPath+".control", &svc.Control)

	if svc.Recovery != nil {
		v.recovery(svcPath+".recovery", svc.Recovery)
	}
}

// serviceControl warns about ServiceControl settings that do not do what
// they look like.
func (v *validator) serviceControl(ctlPath string, ctl *ServiceControl) {
	if !*ctl.DeleteOnUninstall {
		v.warnf(ctlPath+".deleteOnUninstall", "without it the service stays registered after uninstall (SVC-01)")
	}

	if ctl.WaitForStart != nil && !*ctl.StartOnInstall {
		v.warnf(ctlPath+".waitForStart", "has no effect without startOnInstall")
	}
}

// secondsPerDay converts resetAfterDays to the seconds sc.exe takes.
const secondsPerDay = 86400

// recoveryDuration checks a delay or reset period that sc.exe takes as a
// whole number of units (milliseconds or seconds) in a 32-bit value.
func (v *validator) recoveryDuration(field string, value *Duration, unit time.Duration) {
	switch {
	case value == nil:
	case *value < 0:
		v.errorf(field, "must not be negative")
	case time.Duration(*value)%unit != 0:
		v.errorf(field, "must be a whole number of %s", map[time.Duration]string{time.Millisecond: "milliseconds", time.Second: "seconds"}[unit])
	case int64(time.Duration(*value)/unit) > math.MaxInt32:
		v.errorf(field, "is too long for sc.exe")
	}
}

// recovery checks a service's failure actions.
func (v *validator) recovery(recPath string, rec *ServiceRecovery) {
	if len(rec.Actions) == 0 || len(rec.Actions) > 3 {
		v.errorf(recPath+".actions", "one to three actions")
	}

	for j, recAction := range rec.Actions {
		step := fmt.Sprintf("%s.actions[%d]", recPath, j)
		v.oneOf(step, recAction.Action, "restart", "none")

		if recAction.Restart != nil {
			if recAction.Action != recoveryRestart {
				v.errorf(step+".restart", "a delay is only allowed on a restart action")
			}

			v.recoveryDuration(step+".restart", recAction.Restart, time.Millisecond)
		}
	}

	v.recoveryDuration(recPath+".restartDelay", rec.RestartDelay, time.Millisecond)

	if rec.ResetAfterDays < 0 {
		v.errorf(recPath+".resetAfterDays", "must not be negative")
	}

	if rec.ResetAfterDays > math.MaxInt32/secondsPerDay {
		v.errorf(recPath+".resetAfterDays", "is too long for sc.exe")
	}

	if rec.ResetAfter != nil {
		if rec.ResetAfterDays != 0 {
			v.errorf(recPath+".resetAfter", "set resetAfter or resetAfterDays, not both")
		}

		v.recoveryDuration(recPath+".resetAfter", rec.ResetAfter, time.Second)
	}
}

func (v *validator) environment(envPath string, env *Environment, perUser bool) {
	if v.required(envPath+".name", env.Name) && strings.ContainsAny(env.Name[:1], "=+-!*") {
		v.errorf(envPath+".name", "write the plain name; action, scope and removeOnUninstall set the prefix")
	}

	v.oneOf(envPath+".action", env.Action, "set", "setIfAbsent", "append", "prepend", "remove")
	v.oneOf(envPath+".scope", env.Scope, "machine", "user")

	if env.Scope == "machine" && perUser {
		v.errorf(envPath+".scope", "a per-user package cannot change machine variables")
	}

	if env.Action != "remove" && env.Value == "" {
		v.errorf(envPath+".value", "required")
	}

	if strings.Contains(env.Value, "[~]") {
		v.errorf(envPath+".value", "use action append or prepend instead of [~]")
	}

	if env.Action == "set" && strings.EqualFold(env.Name, "PATH") {
		v.warnf(envPath+".action", "set replaces the whole PATH; use append or prepend")
	}
}

func (v *validator) properties(cfg *Config) {
	for _, name := range cfg.Properties.Names() {
		prop := cfg.Properties[name]
		propPath := "properties." + name

		if !IsIdentifier(name) || len(name) > MaxIdentifierLength {
			v.errorf(propPath, "%q is not an identifier", name)
		}

		if builderProperties[name] {
			v.errorf(propPath, "%s is set by the builder or the installer; it cannot be defined here", name)
		}

		public := IsPublicProperty(name)
		if prop.Secure != nil && *prop.Secure && !public {
			v.errorf(propPath+".secure", "only public (upper-case) properties can be secure")
		}

		if prop.Search != nil {
			v.propertySearch(cfg, propPath, name, prop.Search)
		}
	}
}

// propertySearch checks the search of a property.
func (v *validator) propertySearch(cfg *Config, propPath, name string, search *Search) {
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

	if _, taken := cfg.Properties[SearchFoundProperty(name)]; taken {
		v.errorf(searchPath, "%s holds the search result; it cannot also be defined in properties", SearchFoundProperty(name))
	}

	reg := search.Registry
	if reg == nil {
		v.errorf(searchPath+".registry", "required")

		return
	}

	v.registrySearch(cfg, searchPath+".registry", reg)
}

// registrySearch checks the registry value a property search reads.
func (v *validator) registrySearch(cfg *Config, regPath string, reg *RegistrySearch) {
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

	if reg.Type != "" {
		v.oneOf(regPath+".type", reg.Type, SearchValue, SearchDirectory)
	}
}

func (v *validator) launchConditions(cfg *Config) {
	for i, lc := range cfg.LaunchConditions {
		p := fmt.Sprintf("launchConditions[%d]", i)
		v.required(p+".condition", lc.Condition)
		v.required(p+".message", lc.Message)
	}
}

func (v *validator) customActions(cfg *Config, known *refs) {
	seen := make(map[string]bool)

	for i, action := range cfg.CustomActions {
		actionPath := fmt.Sprintf("customActions[%d]", i)

		if action.origin != "" {
			v.generatedArgument(action)

			continue
		}

		v.actionID(actionPath, action.ID, seen)

		v.actionKind(cfg, actionPath, action, known)
		inScript := action.Execute != executeImmediate
		v.actionRun(cfg, actionPath, action)

		if len(action.Schedule) == 0 {
			v.errorf(actionPath+".schedule", "required: an action that is never scheduled never runs")
		}

		for j, pl := range action.Schedule {
			v.placement(fmt.Sprintf("%s.schedule[%d]", actionPath, j), pl, inScript, cfg)
		}
	}

	v.runDataIDs(cfg)
}

// runDataIDs rejects an action named like the <id>.Data action the builder
// adds for a run action, which would otherwise fail late as a duplicate.
func (v *validator) runDataIDs(cfg *Config) {
	index := make(map[string]int, len(cfg.CustomActions))
	for i, action := range cfg.CustomActions {
		index[action.ID] = i
	}

	for _, action := range cfg.CustomActions {
		if action.Run == nil {
			continue
		}

		if i, clash := index[action.ID+".Data"]; clash {
			v.errorf(fmt.Sprintf("customActions[%d].id", i), "%s.Data is the data action the builder adds for the run action %s", action.ID, action.ID)
		}
	}
}

// actionID checks a custom action's id and records it in seen.
func (v *validator) actionID(actionPath, actionID string, seen map[string]bool) {
	if !v.required(actionPath+".id", actionID) {
		return
	}

	if !IsIdentifier(actionID) || len(actionID) > MaxIdentifierLength {
		v.errorf(actionPath+".id", "%q is not an identifier", actionID)
	}

	if standardActions[actionID] {
		v.errorf(actionPath+".id", "%s is a standard action name (ICE93)", actionID)
	}

	if seen[actionID] {
		v.errorf(actionPath+".id", "custom action %s is defined twice", actionID)
	}

	seen[actionID] = true
}

// actionKind checks that a custom action does exactly one thing, its
// execute value, and the fields of that thing.
func (v *validator) actionKind(cfg *Config, actionPath string, action *CustomAction, known *refs) { //nolint:cyclop // a flat list of independent checks
	kinds := 0

	for _, set := range []bool{
		action.SetProperty != nil, action.SetDirectory != nil, action.Error != "",
		action.Exec != nil, action.ExecProperty != nil, action.DLL != nil, action.Run != nil,
	} {
		if set {
			kinds++
		}
	}

	if kinds != 1 {
		v.errorf(actionPath, "exactly one of setProperty, setDirectory, error, exec, execProperty, dll or run is required")
	}

	v.oneOf(actionPath+".execute", action.Execute, executeImmediate, "deferred", "rollback", "commit")
	inScript := action.Execute != executeImmediate

	switch {
	case action.SetProperty != nil:
		if v.required(actionPath+".setProperty.name", action.SetProperty.Name) && !IsIdentifier(action.SetProperty.Name) {
			v.errorf(actionPath+".setProperty.name", "%q is not an identifier", action.SetProperty.Name)
		}

		if inScript {
			v.errorf(actionPath+".execute", "setProperty cannot be deferred; deferred actions read CustomActionData")
		}
	case action.SetDirectory != nil:
		if known.dirs[action.SetDirectory.Directory] == nil {
			v.errorf(actionPath+".setDirectory.directory", "directory %q is not defined", action.SetDirectory.Directory)
		}

		v.required(actionPath+".setDirectory.value", action.SetDirectory.Value)

		if inScript {
			v.errorf(actionPath+".execute", "setDirectory cannot be deferred")
		}
	case action.Error != "":
		if inScript {
			v.errorf(actionPath+".execute", "error actions cannot be deferred")
		}
	case action.Exec != nil:
		v.required(actionPath+".exec.command", action.Exec.Command)

		if action.Exec.Directory != targetDir && known.dirs[action.Exec.Directory] == nil && !standardFolders[action.Exec.Directory] {
			v.errorf(actionPath+".exec.directory", "directory %q is not defined", action.Exec.Directory)
		}
	case action.ExecProperty != nil:
		v.required(actionPath+".execProperty.property", action.ExecProperty.Property)
	case action.DLL != nil:
		if _, ok := cfg.Binaries[action.DLL.Binary]; !ok {
			v.errorf(actionPath+".dll.binary", "binary %q is not defined", action.DLL.Binary)
		}

		v.required(actionPath+".dll.entry", action.DLL.Entry)
	case action.Run != nil:
		v.required(actionPath+".run.command", action.Run.Command)

		if dir := action.Run.Directory; dir != "" && dir != targetDir && known.dirs[dir] == nil && !standardFolders[dir] {
			v.errorf(actionPath+".run.directory", "directory %q is not defined", dir)
		}

		if action.Return == "asyncNoWait" {
			v.errorf(actionPath+".return", "run actions cannot use asyncNoWait; the DLL waits for the command")
		}

		if len(action.ID)+len(".Data") > MaxIdentifierLength {
			v.errorf(actionPath+".id", "%q is too long: the builder adds a %s.Data action and identifiers hold %d characters", action.ID, action.ID, MaxIdentifierLength)
		}
	}
}

// actionRun checks how a custom action runs: impersonation, return
// processing and once.
func (v *validator) actionRun(cfg *Config, actionPath string, action *CustomAction) { //nolint:cyclop // a flat list of independent checks
	inScript := action.Execute != executeImmediate
	isExec := action.Exec != nil || action.ExecProperty != nil || action.Run != nil

	if !*action.Impersonate && !inScript {
		v.errorf(actionPath+".impersonate", "impersonate: false needs a deferred, rollback or commit action")
	}

	if !*action.Impersonate && cfg.Package.Scope == ScopePerUser {
		v.errorf(actionPath+".impersonate", "a per-user package cannot run actions without impersonation (ICE105)")
	}

	if isExec && action.Return == "" {
		v.errorf(actionPath+".return", "required for exec and run actions: check fails the install on a non-zero exit "+
			"code, ignore does not (CA-02)")
	} else if action.Return != "" {
		v.oneOf(actionPath+".return", action.Return, "check", "ignore", "asyncWait", "asyncNoWait")
	}

	if (action.Return == "asyncWait" || action.Return == "asyncNoWait") && (!isExec || action.Execute == "rollback") {
		v.errorf(actionPath+".return", "async needs an exec action that is not a rollback action")
	}

	if action.Once != "" {
		v.oneOf(actionPath+".once", action.Once, "firstSequence", "oncePerProcess", "clientRepeat")

		if inScript {
			v.errorf(actionPath+".once", "only for immediate actions")
		}
	}
}

func (v *validator) placement(placementPath string, place Placement, inScript bool, cfg *Config) {
	if _, ok := sequenceTables[place.Sequence]; !ok {
		v.errorf(placementPath+".sequence", "%q is not one of execute, ui, adminExecute, adminUI", place.Sequence)
	}

	if inScript && (place.Sequence == "ui" || place.Sequence == sequenceAdminUI) {
		v.errorf(placementPath+".sequence", "deferred actions run only in execute sequences (ICE77)")
	}

	v.placementPosition(placementPath, place)
	v.placementWhen(placementPath, place, cfg)

	for _, anchor := range []string{place.Before, place.After} {
		if anchor == "" || standardActions[anchor] {
			continue
		}

		if !hasCustomAction(cfg, anchor) && !hasDialog(cfg, anchor) {
			v.errorf(placementPath, "anchor %q is neither a standard action nor a custom action", anchor)
		}
	}
}

// placementPosition checks that a placement has exactly one of before,
// after and at, and a valid at.
func (v *validator) placementPosition(placementPath string, placement Placement) {
	count := 0

	for _, set := range []bool{placement.Before != "", placement.After != "", placement.At != 0} {
		if set {
			count++
		}
	}

	if count != 1 {
		v.errorf(placementPath, "exactly one of before, after or at is required")
	}

	if placement.At != 0 && (placement.At < minSequenceNumber || placement.At > maxSequenceNumber) {
		v.errorf(placementPath+".at", "must be between 1 and %d, or -1 to -4", maxSequenceNumber)
	}

	if placement.At < 0 && placement.Sequence != "ui" && placement.Sequence != sequenceAdminUI {
		v.errorf(placementPath+".at", "termination numbers belong in UI sequences")
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
			fieldPath := section + "." + key

			if !IsIdentifier(key) {
				v.errorf(fieldPath, "%q is not an identifier", key)
			}

			if len(key) > limit {
				v.errorf(fieldPath, "at most %d characters, so that the stream name fits in %d (MSIDB-11)", limit, MaxStreamNameLength)
			}

			v.required(fieldPath, src)
		}
	}

	check("icons", cfg.Icons, maxIconKeyLength)
	check("binaries", cfg.Binaries, maxBinaryKeyLength)
}

func (v *validator) firewall(cfg *Config, known *refs) { //nolint:cyclop // a flat list of independent checks
	names := make(map[string]bool)

	for i, rule := range cfg.Firewall {
		p := rule.Path(i) //nolint:varnamelen // the path of the rule, as in the other checks

		if rule.ownComponent {
			v.errorf(p+".component", "a rule under a feature follows that feature; remove component")
		}

		if v.required(p+".name", rule.Name) {
			if names[rule.Name] {
				v.errorf(p+".name", "rule %q is defined twice", rule.Name)
			}

			names[rule.Name] = true

			if strings.ContainsAny(rule.Name, `"`) {
				v.errorf(p+".name", "must not contain a double quote")
			}
		}

		switch {
		case !v.required(p+".component", rule.Component):
		case isFolderComponentID(cfg, rule.Component):
			v.errorf(p+".component", "component %s has a folder source, which becomes one component per file; "+
				"put the program on a component of its own and exclude it from the folder", rule.Component)
		case known.components[rule.Component] == nil:
			v.errorf(p+".component", "component %q is not defined", rule.Component)
		}

		v.required(p+".program", rule.Program)
		v.oneOf(p+".protocol", rule.Protocol, "tcp", "udp", "any")

		// The description is quoted on the netsh command line like the name;
		// netsh drops quotes inside it (MSISPEC CA-04).
		if strings.ContainsAny(rule.Description, `"`) {
			v.errorf(p+".description", "must not contain a double quote")
		}

		v.remoteAddresses(p+".remoteAddresses", rule.RemoteAddresses)

		if rule.Protocol == "any" && rule.Port != "" {
			v.errorf(p+".port", "a port needs protocol tcp or udp")
		}

		for j, prof := range rule.Profiles {
			v.oneOf(fmt.Sprintf("%s.profiles[%d]", p, j), prof, "all", "domain", "private", "public")
		}

		if cfg.Package.Scope == ScopePerUser {
			v.errorf(p, "firewall rules need a per-machine package")
		}
	}
}

// remoteAddresses checks a firewall rule's remote address list, which the
// builder passes to netsh's remoteip= unquoted. netsh rejects any before or
// after an address and * before one, and the add action then fails the
// install (MSISPEC CA-04); * after an address is rejected too, untested. Other keywords such as localsubnet combine with
// addresses.
func (v *validator) remoteAddresses(field, list string) {
	if strings.ContainsFunc(list, func(r rune) bool { return unicode.IsSpace(r) || r == '"' }) {
		v.errorf(field, "must not contain spaces or double quotes; separate entries with commas only")

		return
	}

	entries := strings.Split(list, ",")
	if len(entries) < 2 {
		return
	}

	if slices.Contains(entries, "") {
		v.errorf(field, "%q has an empty entry", list)

		return
	}

	for _, entry := range entries {
		if strings.EqualFold(entry, "any") || entry == "*" {
			v.errorf(field, "%s cannot be combined with other addresses; leave remoteAddresses empty for any", entry)

			return
		}
	}
}

func (v *validator) ui(cfg *Config) { //nolint:cyclop // a flat list of independent checks
	v.oneOf("ui.wizard", cfg.UI.Wizard, "none", "minimal", "featureTree")

	if cfg.UI.Wizard == None && len(cfg.UI.Dialogs) > 0 {
		v.errorf("ui.dialogs", "dialogs need a wizard")
	}

	v.theme(cfg)
	v.uiStrings(cfg)
	v.uiLanguages(cfg)

	seen := make(map[string]bool)

	for i, dialog := range cfg.UI.Dialogs {
		dialogPath := fmt.Sprintf("ui.dialogs[%d]", i)

		if v.required(dialogPath+".id", dialog.ID) {
			if !IsIdentifier(dialog.ID) {
				v.errorf(dialogPath+".id", "%q is not an identifier", dialog.ID)
			}

			if seen[dialog.ID] {
				v.errorf(dialogPath+".id", "dialog %s is defined twice", dialog.ID)
			}

			seen[dialog.ID] = true
		}

		v.required(dialogPath+".title", dialog.Title)
		v.required(dialogPath+".after", dialog.After)

		if len(dialog.Fields) == 0 {
			v.errorf(dialogPath+".fields", "at least one field")
		}

		for index, field := range dialog.Fields {
			fieldPath := fmt.Sprintf("%s.fields[%d]", dialogPath, index)
			if v.required(fieldPath+".property", field.Property) && !IsPublicProperty(field.Property) {
				v.errorf(fieldPath+".property", "UI values reach the server side only through public properties")
			}

			if _, ok := cfg.Properties[field.Property]; field.Property != "" && !ok {
				v.warnf(fieldPath+".property", "%s is not declared in properties, so it is not secure", field.Property)
			}

			v.dialogField(cfg, fieldPath, &dialog.Fields[index])
		}
	}

	v.choiceProperties(cfg)
}

// launchProperty is the property of the ui.launch check box.
const launchProperty = "LaunchOnFinish"

// wizardOptions checks the install folder page and the launch check box.
func (v *validator) wizardOptions(cfg *Config, ids *refs) {
	none := cfg.UI.Wizard == None

	if cfg.UI.InstallDirPage {
		if none {
			v.errorf("ui.installDirPage", "an install folder page needs a wizard")
		}

		if dir := ids.dirs[cfg.InstallDir.ID]; dir == nil || !dir.Configurable {
			v.errorf("ui.installDirPage", "an install folder page needs installDir.id to name a configurable directory")
		}
	}

	launch := cfg.UI.Launch
	if launch == nil {
		return
	}

	if none {
		v.errorf("ui.launch", "a launch check box needs a wizard")
	}

	if v.required("ui.launch.file", launch.File) && !ids.hasFile(launch.File) {
		v.errorf("ui.launch.file", "file %q is not defined", launch.File)
	}

	// The check box sets the property; a value from properties would
	// override checked.
	if _, taken := cfg.Properties[launchProperty]; taken {
		v.errorf("properties."+launchProperty, "the property is set by the ui.launch check box")
	}
}

// finishText checks ui.finishText: it needs a wizard and some text.
func (v *validator) finishText(cfg *Config) {
	if cfg.UI.FinishText != "" && cfg.UI.Wizard == None {
		v.errorf("ui.finishText", "a finish page text needs a wizard")
	}

	v.checkFinishText("ui.finishText", cfg.UI.FinishText)
}

// checkFinishText reports a finish page text at path that is only blanks.
func (v *validator) checkFinishText(at, text string) {
	if text != "" && strings.TrimSpace(text) == "" {
		v.errorf(at, "the finish page text is blank")
	}
}

// colorPattern is the #RRGGBB form of the theme colours.
var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (v *validator) theme(cfg *Config) {
	none := cfg.UI.Wizard == None

	if none && cfg.UI.License != "" {
		v.errorf("ui.license", "a license page needs a wizard")
	}

	v.finishText(cfg)

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

func (v *validator) tables(cfg *Config) { //nolint:cyclop // a flat list of independent checks
	for i, table := range cfg.Tables {
		tablePath := fmt.Sprintf("tables[%d]", i)

		if v.required(tablePath+".name", table.Name) {
			if !IsIdentifier(table.Name) || len(table.Name) > MaxTableNameLength {
				v.errorf(tablePath+".name", "%q is not an identifier of at most %d characters", table.Name, MaxTableNameLength)
			}

			if standardTables[table.Name] {
				v.errorf(tablePath+".name", "%s is written by the builder", table.Name)
			}
		}

		if len(table.Columns) == 0 || len(table.Columns) > MaxColumns {
			v.errorf(tablePath+".columns", "one to %d columns (MSIDB-07)", MaxColumns)
		}

		keys := 0

		for j, col := range table.Columns {
			colPath := fmt.Sprintf("%s.columns[%d]", tablePath, j)

			if v.required(colPath+".name", col.Name) && !IsIdentifier(col.Name) {
				v.errorf(colPath+".name", "%q is not an identifier", col.Name)
			}

			if !columnTypePattern.MatchString(col.Type) {
				v.errorf(colPath+".type", "%q is not a column type such as s72, S255, l0, i2, I4 or v0", col.Type)
			}

			if col.Key {
				keys++
			}
		}

		if keys == 0 && len(table.Columns) > 0 {
			v.errorf(tablePath+".columns", "at least one key column")
		}

		for j, row := range table.Rows {
			v.tableRow(fmt.Sprintf("%s.rows[%d]", tablePath, j), table.Columns, row)
		}
	}
}

func (v *validator) tableRow(rowPath string, cols []Column, row []any) { //nolint:cyclop,gocognit // a flat list of independent checks
	if len(row) != len(cols) {
		v.errorf(rowPath, "%d values for %d columns", len(row), len(cols))

		return
	}

	for index, val := range row {
		col := cols[index]
		if len(col.Type) < 2 {
			continue
		}

		nullable := col.Type[0] >= 'A' && col.Type[0] <= 'Z'
		cellPath := fmt.Sprintf("%s[%d]", rowPath, index)

		if val == nil {
			if !nullable || col.Key {
				v.errorf(cellPath, "column %s is not nullable", col.Name)
			}

			continue
		}

		switch col.Type[0] {
		case 'i', 'I':
			num, ok := val.(int)
			if !ok {
				v.errorf(cellPath, "column %s needs an integer", col.Name)

				continue
			}

			// The type's minimum is stored as NULL (MSIDB-02).
			low, high := math.MinInt16+1, math.MaxInt16
			if col.Type[1] == '4' {
				low, high = math.MinInt32+1, math.MaxInt32
			}

			if num < low || num > high {
				v.errorf(cellPath, "%d is outside %d…%d", num, low, high)
			}
		case 's', 'S', 'l', 'L':
			str, ok := val.(string)
			if !ok {
				v.errorf(cellPath, "column %s needs a string", col.Name)

				continue
			}

			if size, _ := strconv.Atoi(col.Type[1:]); size > 0 && len([]rune(str)) > size {
				v.errorf(cellPath, "longer than %d characters", size)
			}

			if str == "" && !nullable {
				v.errorf(cellPath, "column %s is not nullable and an empty string is NULL", col.Name)
			}
		case 'v', 'V':
			if _, ok := val.(string); !ok {
				v.errorf(cellPath, "column %s needs a source file path", col.Name)
			}
		}
	}
}
