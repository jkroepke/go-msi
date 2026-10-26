package builder

import (
	"strings"

	"github.com/jkroepke/go-msi/pkg/config"
)

// removeOnUninstall is the RemoveFile InstallMode that removes at uninstall.
const removeOnUninstall = 2

// showCommands maps a shortcut's show setting to ShowCmd; normal is the
// null default (RUN-28).
var showCommands = map[string]any{config.ShowNormal: nil, config.ShowMaximized: 3, config.ShowMinimized: 7} //nolint:gochecknoglobals // lookup table

// shortcuts writes the Shortcut rows. The shortcuts to the files of one
// component go into a generated component of their own, in the folder of
// the first shortcut, with an HKCU registry value as key path (ICE38, ICE43)
// and RemoveFile rows for the folders it creates in the user profile
// (ICE64). The component joins every feature of the file's component. Both
// this layout and shortcuts in the file's own component install and remove
// cleanly (RUN-28); this one also passes the ICE suite.
func (b *build) shortcuts() error {
	for _, comp := range b.cfg.Components {
		info := b.components[comp.ID]

		var carrier string

		removed := make(map[string]bool)

		for _, cfgFile := range comp.Files {
			for _, shortcut := range cfgFile.Shortcuts {
				if carrier == "" {
					carrier = b.shortcutComponent(info, shortcut.Directory)
				}

				b.writeShortcut(carrier, config.FileKey(cfgFile), shortcut)
				b.removeShortcutFolders(carrier, shortcut.Directory, removed)
			}
		}
	}

	return nil
}

// shortcutComponent writes the component that carries the shortcuts to the
// files of info and returns its key.
func (b *build) shortcutComponent(info *componentInfo, dirID string) string {
	comp := info.cfg
	carrier := generatedID("cmp", "Shortcuts", comp.ID)
	registry := generatedID("reg", "Shortcuts", comp.ID)

	bitness := "32"
	if info.is64 {
		bitness = "64"
	}

	guid := uuid5(namespaceComponent, b.upgradeCode+"|"+bitness+"|shortcuts|"+strings.ToLower(comp.ID))
	attributes := compRegistryKeyPath | info.attributes&(comp64Bit|compPermanent|compTransitive)

	b.table("Component").Add(carrier, guid, dirID, attributes, nullable(comp.Condition), registry)
	b.table("Registry").Add(registry, registryRoots["HKCU"], `Software\[Manufacturer]\[ProductName]`, comp.ID, "#1", carrier)
	b.dirs[dirID].used = true
	b.shortcutCarrier[comp.ID] = carrier

	return carrier
}

func (b *build) writeShortcut(carrier, fileKey string, shortcut *config.Shortcut) {
	dirID := shortcut.Directory

	if b.shortcutNames[dirID] == nil {
		b.shortcutNames[dirID] = make(map[string]bool)
	}

	key := shortcut.ID
	if key == "" {
		key = generatedID("sc", "Shortcut", fileKey, dirID, strings.ToLower(longPart(shortcut.Name)))
	}

	name := filename(dirID, shortcut.Name, false, b.shortcutNames[dirID], config.ValidShortName)

	var icon, iconIndex any
	if shortcut.Icon != "" {
		icon, iconIndex = shortcut.Icon, shortcut.IconIndex
	}

	b.dirs[shortcut.WorkingDirectory].used = true
	b.table("Shortcut").Add(key, dirID, name, carrier, "[#"+fileKey+"]", nullable(shortcut.Arguments), nullable(shortcut.Description),
		nil, icon, iconIndex, showCommands[shortcut.Show], shortcut.WorkingDirectory, nil, nil, nil, nil)
}

// removeShortcutFolders writes a RemoveFile row for every folder between a
// shortcut's directory and the profile folder above it, once per carrier.
func (b *build) removeShortcutFolders(carrier, dirID string, removed map[string]bool) {
	var chain []string

	dir := b.dirs[dirID]
	for dir != nil && !dir.standard {
		chain = append(chain, dir.id)
		dir = b.dirs[dir.parent]
	}

	if dir == nil || !config.IsProfileFolder(dir.id) {
		return
	}

	for _, folder := range chain {
		if removed[folder] {
			continue
		}

		removed[folder] = true
		b.table("RemoveFile").Add(generatedID("rmf", "Shortcuts", carrier, folder), carrier, nil, folder, removeOnUninstall)
	}
}
