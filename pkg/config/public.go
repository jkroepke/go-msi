package config

// FileLongName returns the long name of a file: the long part of a
// short|long name, else the name, else the base name of the source path.
func FileLongName(f *File) string { return fileName(f) }

// FileKey returns the File table key: the explicit id, else the long name.
func FileKey(f *File) string { return fileID(f) }

// IsStandardFolder reports whether id is a standard folder property such as
// ProgramFiles64Folder or TARGETDIR.
func IsStandardFolder(id string) bool { return standardFolders[id] }

// IsProfileFolder reports whether id is a standard folder in the user
// profile, such as ProgramMenuFolder or DesktopFolder.
func IsProfileFolder(id string) bool { return profileFolders[id] }

// Is32BitFolder reports whether id is a standard folder that is the 32-bit
// location on 64-bit Windows, such as ProgramFilesFolder (ICE80).
func Is32BitFolder(id string) bool { return folders32[id] }

// IsStandardAction reports whether name is a standard action.
func IsStandardAction(name string) bool { return standardActions[name] }

// SequenceTable maps a schedule name (execute, ui, adminExecute, adminUI) to
// its table name.
func SequenceTable(schedule string) (string, bool) {
	t, ok := sequenceTables[schedule]

	return t, ok
}

// WalkFeatures calls fn for every feature depth first with its parent (nil at
// the top level).
func WalkFeatures(features []*Feature, fn func(f, parent *Feature)) {
	var walk func([]*Feature, *Feature)

	walk = func(fs []*Feature, parent *Feature) {
		for _, f := range fs {
			fn(f, parent)
			walk(f.Features, f)
		}
	}

	walk(features, nil)
}

// ValidShortName reports whether s is a valid 8.3 name.
func ValidShortName(s string) bool { return isShortName(s) }
