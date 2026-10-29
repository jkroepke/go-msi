package builder

import (
	"io"
	"slices"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// profileKeyPath gives a component in the user profile an HKCU registry
// value under Software\[Manufacturer]\[ProductName] as its key path, as
// ICE38 asks, when needsProfileKeyPath says so. The value is written in
// every installation context, also when a dual package installs per
// machine (INST-04). It runs after componentGUID, so the ComponentId
// still follows the key file, registry value or folder the component had
// before, and a package keeps its component codes.
func (b *build) profileKeyPath(info *componentInfo) {
	comp := info.cfg
	if !b.needsProfileKeyPath(comp) {
		return
	}

	registry := generatedID("reg", "ProfileKeyPath", b.upgradeCode, comp.ID)
	valueName := generatedID("cmp", "ProfileKeyPath", b.upgradeCode, comp.ID)

	info.keyPath = registry
	info.attributes |= compRegistryKeyPath

	b.table("Registry").Add(registry, registryRoots[rootHKCU], `Software\[Manufacturer]\[ProductName]`, valueName, "#1", comp.ID)
}

// needsProfileKeyPath reports whether a component below a profile folder
// leaves its key path to the builder and would not get an HKCU value
// anyway: no keyPath, no services (they need the executable), and neither
// neverOverwrite (also set by preserve) nor sharedDllRefCount, whose
// engine checks test the key path itself (UPG-05). An HKMU value counts as
// HKCU only in a per-user package: a dual package installed per machine
// writes it to HKLM. The validator warns about the components this leaves
// out.
func (b *build) needsProfileKeyPath(comp *config.Component) bool {
	if keepsKeyPath(comp) || shortcutsOnly(comp) || !config.IsProfileFolder(b.topFolder(comp.Directory)) {
		return false
	}

	if len(comp.Files) == 0 && len(comp.Registry) > 0 {
		root := comp.Registry[0].Root

		return root != rootHKCU && (root != "HKMU" || b.cfg.Package.Scope != config.ScopePerUser)
	}

	return true
}

// keepsKeyPath reports whether a component's key path must stay as the
// configuration or resolveKeyPath makes it.
func keepsKeyPath(comp *config.Component) bool {
	return comp.KeyPath != "" || len(comp.Services) > 0 || comp.NeverOverwrite || comp.SharedDLLRefCount
}

// hashProfileKeyPaths adds a marker to the ProductCode digest when a
// component gets an HKCU key path. A package built before msi-builder
// wrote these key paths (INST-04) differs from one built after, so the
// rebuild needs another code, or installing it over the old one fails
// with 1638.
func (b *build) hashProfileKeyPaths(hasher io.Writer) {
	if slices.ContainsFunc(b.cfg.Components, b.needsProfileKeyPath) {
		_, _ = hasher.Write([]byte("keypath:profile-hkcu"))
	}
}
