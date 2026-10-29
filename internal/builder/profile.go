package builder

import "github.com/jkroepke/msi-builder/pkg/config"

// profileKeyPath gives a component in the user profile an HKCU registry
// value under Software\[Manufacturer]\[ProductName] as its key path, as
// ICE38 asks, when the configuration leaves the key path to the builder and
// it would not be an HKCU value anyway. The value is written in every
// installation context, also when a dual package installs per machine
// (INST-04). It runs after componentGUID, so the ComponentId still follows
// the key file, registry value or folder the component had before, and a
// package keeps its component codes. A component with services keeps the
// service executable (resolveKeyPath) and the validator's warning.
func (b *build) profileKeyPath(info *componentInfo) {
	comp := info.cfg

	if comp.KeyPath != "" || len(comp.Services) > 0 ||
		!config.IsProfileFolder(b.topFolder(comp.Directory)) || b.userKeyPath(info) {
		return
	}

	registry := generatedID("reg", "ProfileKeyPath", b.upgradeCode, comp.ID)
	valueName := generatedID("cmp", "ProfileKeyPath", b.upgradeCode, comp.ID)

	info.keyPath = registry
	info.attributes |= compRegistryKeyPath

	b.table("Registry").Add(registry, registryRoots[rootHKCU], `Software\[Manufacturer]\[ProductName]`, valueName, "#1", comp.ID)
}

// userKeyPath reports whether the component's key path is a registry value
// under HKCU, or under HKMU in a package that may install per user, as the
// validator decides it.
func (b *build) userKeyPath(info *componentInfo) bool {
	if info.keyFile != nil || info.keyPath == nil {
		return false
	}

	for _, reg := range info.cfg.Registry {
		if b.registryID(info, reg) == info.keyPath {
			return reg.Root == rootHKCU || reg.Root == "HKMU" && b.cfg.Package.Scope.MayBePerUser()
		}
	}

	return false
}
