package builder

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jkroepke/go-msi/pkg/config"
)

const (
	// removeOnUninstall is the RemoveFile InstallMode that removes at
	// uninstall.
	removeOnUninstall = 2
	// maxShortcutNameColumn is the width of Shortcut.Name (l128).
	maxShortcutNameColumn = 128
)

// showCommands maps a shortcut's show setting to ShowCmd; normal is the
// null default (RUN-28).
var showCommands = map[string]any{config.ShowNormal: nil, config.ShowMaximized: 3, config.ShowMinimized: 7} //nolint:gochecknoglobals // lookup table

// shortcuts writes the Shortcut rows. The shortcuts to the files of one
// component go into a generated component of their own, in the folder of
// the first shortcut, with an HKCU registry value as key path (ICE38, ICE43)
// and RemoveFile rows for the folders it creates in the user profile
// (ICE64). The component joins every feature of the file's component.
// RUN-28 installed and removed shortcuts from such a component and from the
// file's own one alike; the ICE suite asks for this layout.
func (b *build) shortcuts() error {
	for _, comp := range b.cfg.Components {
		info := b.components[comp.ID]
		path := info.cfg.Path(info.index)

		var carrier string

		removed := make(map[string]bool)

		for fileIndex, cfgFile := range comp.Files {
			for index, shortcut := range cfgFile.Shortcuts {
				if carrier == "" {
					carrier = b.shortcutComponent(info, shortcut.Directory)
				}

				b.writeShortcut(fmt.Sprintf("%s.files[%d].shortcuts[%d]", path, fileIndex, index), carrier, config.FileKey(cfgFile), shortcut)
				b.removeShortcutFolders(carrier, shortcut.Directory, removed)
			}
		}
	}

	return nil
}

// topFolder returns the standard folder above a directory.
func (b *build) topFolder(dirID string) string {
	dir := b.dirs[dirID]
	for dir != nil && !dir.standard {
		dir = b.dirs[dir.parent]
	}

	if dir == nil {
		return ""
	}

	return dir.id
}

// shortcutComponent writes the component that carries the shortcuts to the
// files of info and returns its key. The key and the key path value name
// hold the upgrade code and the case of the component id, so that neither
// two components nor two products share a key path.
func (b *build) shortcutComponent(info *componentInfo, dirID string) string {
	comp := info.cfg
	carrier := generatedID("cmp", "Shortcuts", b.upgradeCode, comp.ID)
	registry := generatedID("reg", "Shortcuts", b.upgradeCode, comp.ID)

	// The component's bitness follows its folder: a 64-bit component in a
	// 32-bit folder fails ICE80.
	is64 := info.is64 && !config.Is32BitFolder(b.topFolder(dirID))

	bitness := "32"
	attributes := compRegistryKeyPath | info.attributes&(compPermanent|compTransitive)

	if is64 {
		bitness = "64"
		attributes |= comp64Bit
	}

	guid := uuid5(namespaceComponent, b.upgradeCode+"|"+bitness+"|shortcuts|"+carrier)

	b.table("Component").Add(carrier, guid, dirID, attributes, nullable(comp.Condition), registry)
	b.table("Registry").Add(registry, registryRoots["HKCU"], `Software\[Manufacturer]\[ProductName]`, carrier, "#1", carrier)
	b.dirs[dirID].used = true
	b.shortcutCarrier[comp.ID] = carrier

	return carrier
}

func (b *build) writeShortcut(path, carrier, fileKey string, shortcut *config.Shortcut) {
	dirID := shortcut.Directory

	if b.shortcutNames[dirID] == nil {
		b.shortcutNames[dirID] = make(map[string]bool)
	}

	key := shortcut.ID
	if key == "" {
		key = generatedID("sc", "Shortcut", fileKey, dirID, strings.ToLower(longPart(shortcut.Name)))
	}

	// The column holds the generated short name too.
	name := filename(dirID, shortcut.Name, false, b.shortcutNames[dirID], config.ValidShortName)
	if n := utf8.RuneCountInString(name); n > maxShortcutNameColumn {
		b.errorf(path+".name", "with its short name the shortcut name is %d characters, at most %d fit", n, maxShortcutNameColumn)
	}

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
	if !config.IsProfileFolder(b.topFolder(dirID)) {
		return
	}

	for dir := b.dirs[dirID]; dir != nil && !dir.standard; dir = b.dirs[dir.parent] {
		if removed[dir.id] {
			continue
		}

		removed[dir.id] = true
		b.table("RemoveFile").Add(generatedID("rmf", "Shortcuts", carrier, dir.id), carrier, nil, dir.id, removeOnUninstall)
	}
}
