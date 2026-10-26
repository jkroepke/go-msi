package config

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Column limits of the Shortcut table.
const (
	maxShortcutName      = 128
	maxShortcutText      = 255
	maxShortcutIconIndex = 32767
)

// shortcuts checks every shortcut under the files of the components.
func (v *validator) shortcuts(cfg *Config, known *refs) {
	ids := make(map[string]string)
	targets := make(map[string]string)

	for i, comp := range cfg.Components {
		for j, file := range comp.Files {
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
				target := shortcut.Directory + "\x1f" + strings.ToLower(longPart(shortcut.Name))
				if other, dup := targets[target]; dup {
					v.errorf(scPath+".name", "%s also creates %q in %s", other, longPart(shortcut.Name), shortcut.Directory)
				}

				targets[target] = scPath
			}
		}
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
		switch {
		case !validName(shortcut.Name):
			v.errorf(scPath+".name", "%q is not a valid file name or short|long pair", shortcut.Name)
		case utf8.RuneCountInString(shortcut.Name) > maxShortcutName:
			v.errorf(scPath+".name", "at most %d characters", maxShortcutName)
		case strings.HasSuffix(strings.ToLower(longPart(shortcut.Name)), ".lnk"):
			v.warnf(scPath+".name", "Windows adds .lnk to the name; %q becomes %s.lnk", shortcut.Name, longPart(shortcut.Name))
		}
	}

	if known.dirs[shortcut.Directory] == nil {
		v.errorf(scPath+".directory", "directory %q is not defined", shortcut.Directory)
	} else if top := known.topFolder(shortcut.Directory); cfg.Package.Scope == ScopePerUser && perMachineFolders[top] {
		// The shortcuts' component lives in this folder.
		v.errorf(scPath+".directory", "a per-user package cannot install under %s (ICE105)", top)
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
}
