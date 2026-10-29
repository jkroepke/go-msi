package config

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Column limits of the Shortcut table.
const (
	maxShortcutName      = 128
	maxShortcutText      = 255
	maxShortcutIconIndex = 32767
	maxShortcutTarget    = 72
	// minShortcutPropertyPages is the Page Count of Windows Installer 5.0,
	// the first engine that reads MsiShortcutProperty.
	minShortcutPropertyPages = 500
)

// clsidProperties are the shell properties of type CLSID that shortcuts
// commonly carry.
var clsidProperties = map[string]bool{"System.AppUserModel.ToastActivatorCLSID": true} //nolint:gochecknoglobals // lookup table

// bracedGUID matches a GUID in registry format, which the engine needs for
// a CLSID property.
var bracedGUID = regexp.MustCompile(`^\{[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}$`)

// propertyKeyName matches the canonical name of a shell property,
// dot-separated words such as System.AppUserModel.ID.
var propertyKeyName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+$`)

// shortcuts checks every shortcut under the files of the components.
func (v *validator) shortcuts(cfg *Config, known *refs) {
	ids := make(map[string]string)
	targets := make(map[string]string)

	for i, comp := range cfg.Components {
		for j, file := range comp.Files {
			v.shortcutTarget(fmt.Sprintf("%s.files[%d].shortcuts", comp.Path(i), j), file)

			for k, shortcut := range file.Shortcuts {
				scPath := fmt.Sprintf("%s.files[%d].shortcuts[%d]", comp.Path(i), j, k)
				if shortcut == nil {
					v.errorf(scPath, "empty entry")

					continue
				}

				v.shortcutID(scPath, shortcut, ids)
				v.shortcut(cfg, scPath, shortcut, known)

				// The engine writes [Directory_]Name.lnk: two shortcuts of
				// one name in one folder overwrite each other.
				target := shortcut.Directory + "\x1f" + strings.ToLower(shortcut.Name)
				if other, dup := targets[target]; dup {
					v.errorf(scPath+".name", "%s also creates %q in %s", other, shortcut.Name, shortcut.Directory)
				}

				targets[target] = scPath
			}
		}
	}
}

// shortcutTarget checks that [#key], the target the builder writes, fits
// the s72 Target column.
func (v *validator) shortcutTarget(scPath string, file *File) {
	if key := fileID(file); len(file.Shortcuts) > 0 && len(key)+3 > maxShortcutTarget {
		v.errorf(scPath, "the shortcut target [#%s] is %d characters, at most %d fit; use a shorter file id", key, len(key)+3, maxShortcutTarget)
	}
}

func (v *validator) shortcutID(scPath string, shortcut *Shortcut, ids map[string]string) {
	if shortcut.ID == "" {
		return
	}

	if !IsIdentifier(shortcut.ID) || len(shortcut.ID) > MaxIdentifierLength {
		v.errorf(scPath+".id", "%q is not an identifier of at most %d characters", shortcut.ID, MaxIdentifierLength)
	}

	if other, dup := ids[shortcut.ID]; dup {
		v.errorf(scPath+".id", "shortcut %s is also defined at %s", shortcut.ID, other)
	}

	ids[shortcut.ID] = scPath
}

func (v *validator) shortcut(cfg *Config, scPath string, shortcut *Shortcut, known *refs) { //nolint:cyclop // a flat list of independent checks
	if v.required(scPath+".name", shortcut.Name) {
		switch problem := nameProblem(shortcut.Name, "file name"); {
		case problem != "":
			v.errorf(scPath+".name", "%s", problem)
		case utf8.RuneCountInString(shortcut.Name) > maxShortcutName:
			v.errorf(scPath+".name", "at most %d characters", maxShortcutName)
		case strings.HasSuffix(strings.ToLower(shortcut.Name), ".lnk"):
			v.warnf(scPath+".name", "Windows adds .lnk to the name; %q becomes %s.lnk", shortcut.Name, shortcut.Name)
		}
	}

	if known.dirs[shortcut.Directory] == nil {
		v.errorf(scPath+".directory", "directory %q is not defined", shortcut.Directory)
	} else if top := known.topFolder(shortcut.Directory); v.forbiddenFolder(top) {
		// The shortcuts' component lives in this folder.
		v.errorf(scPath+".directory", "%s cannot install under %s (ICE105)", v.perUserPackage(), top)
	}

	if known.dirs[shortcut.WorkingDirectory] == nil {
		v.errorf(scPath+".workingDirectory", "directory %q is not defined", shortcut.WorkingDirectory)
	}

	if shortcut.Icon != "" {
		if _, ok := cfg.Icons[shortcut.Icon]; !ok {
			v.errorf(scPath+".icon", "icon %q is not defined in icons", shortcut.Icon)
		}
	}

	if shortcut.Icon == "" && shortcut.IconIndex != 0 {
		v.errorf(scPath+".iconIndex", "an icon index needs icon; without one the shell shows the target's first icon")
	}

	if shortcut.IconIndex < -maxShortcutIconIndex || shortcut.IconIndex > maxShortcutIconIndex {
		v.errorf(scPath+".iconIndex", "%d is out of range -%d to %d", shortcut.IconIndex, maxShortcutIconIndex, maxShortcutIconIndex)
	}

	for _, field := range []struct{ name, text string }{{"arguments", shortcut.Arguments}, {"description", shortcut.Description}} {
		if utf8.RuneCountInString(field.text) > maxShortcutText {
			v.errorf(scPath+"."+field.name, "at most %d characters", maxShortcutText)
		}
	}

	v.oneOf(scPath+".show", shortcut.Show, ShowNormal, ShowMaximized, ShowMinimized)
	v.shortcutProperties(cfg, scPath, shortcut)
}

// shortcutProperties checks the shell properties of a shortcut, the rows of
// MsiShortcutProperty (spec section 19.1).
func (v *validator) shortcutProperties(cfg *Config, scPath string, shortcut *Shortcut) {
	for _, name := range slices.Sorted(maps.Keys(shortcut.Properties)) {
		propPath := scPath + ".properties." + name
		if !propertyKeyName.MatchString(name) {
			v.errorf(propPath, "%q is not the canonical name of a shell property, such as System.AppUserModel.ID", name)
		}

		value := shortcut.Properties[name]

		switch {
		case value == "":
			v.errorf(propPath, "a value is required; PropVariantValue cannot be empty")
		case clsidProperties[name] && !strings.Contains(value, "[") && !bracedGUID.MatchString(value):
			// SHORTCUT-02: without braces the engine logs warning 1946
			// (HRESULT 87) and leaves the property unset.
			v.errorf(propPath, "%q is not a CLSID in braces, such as {6E3F1A3C-61B6-4C1B-9A0F-2C5D8B7E4A10}", value)
		}
	}

	if len(shortcut.Properties) == 0 {
		return
	}

	if pages, err := PageCount(cfg.Package.InstallerVersion); err == nil && pages < minShortcutPropertyPages {
		v.warnf(scPath+".properties", "Windows Installer %s ignores shortcut properties; they need installerVersion 5.0", cfg.Package.InstallerVersion)
	}
}
