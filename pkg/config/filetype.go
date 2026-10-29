package config

import (
	"fmt"
	"strings"
	"unicode"
)

// maxProgIDLength is the longest ProgID Microsoft documents for COM.
const maxProgIDLength = 39

// maxVerbLength is the width of the Verb table's Verb column (s32).
const maxVerbLength = 32

// DefaultVerb is the shell verb a file type gets when it lists none.
const DefaultVerb = "open"

// DefaultVerbArguments follow the quoted file path in a verb's command.
const DefaultVerbArguments = `"%1"`

// systemExtensions are extensions that Windows or common programs already
// own. A package that registers one takes it over from them, and the
// engine's UnregisterExtensionInfo removes the extension's default on
// uninstall (observed, REG-05).
var systemExtensions = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"bat": true, "bmp": true, "cer": true, "cmd": true, "crt": true, "csv": true, "dll": true,
	"exe": true, "gif": true, "htm": true, "html": true, "inf": true, "ini": true, "jpeg": true,
	"jpg": true, "js": true, "json": true, "lnk": true, "log": true, "md": true, "msi": true,
	"pdf": true, "pem": true, "png": true, "ps1": true, "reg": true, "sys": true, "txt": true,
	"url": true, "vbs": true, "xml": true, "zip": true,
}

// defaultFileTypes gives a file type without verbs an open verb and every
// verb its default arguments.
func defaultFileTypes(file *File) {
	for _, fileType := range file.FileTypes {
		if fileType == nil {
			continue
		}

		if len(fileType.Verbs) == 0 {
			fileType.Verbs = []*FileTypeVerb{{ID: DefaultVerb}}
		}

		for _, verb := range fileType.Verbs {
			if verb != nil && verb.Arguments == nil {
				verb.Arguments = new(DefaultVerbArguments)
			}
		}
	}
}

// fileTypes checks the file types of every file. ProgIDs and extensions are
// keys under HKEY_CLASSES_ROOT, so each ProgID and each extension's default
// may be registered once per package; any number of ProgIDs may list an
// extension with openWithOnly.
func (v *validator) fileTypes(cfg *Config) {
	progIDs := make(map[string]string)
	extensions := make(map[string]string)

	for i, comp := range cfg.Components {
		for j, file := range comp.Files {
			for k, fileType := range file.FileTypes {
				typePath := fmt.Sprintf("%s.files[%d].fileTypes[%d]", comp.Path(i), j, k)
				if fileType == nil {
					v.errorf(typePath, "empty entry")

					continue
				}

				v.progID(typePath+".progId", fileType.ProgID, progIDs)

				listed := make(map[string]bool)

				for l, ext := range fileType.Extensions {
					v.fileTypeExtension(fmt.Sprintf("%s.extensions[%d]", typePath, l), ext, extensions, listed)
				}

				verbs := make(map[string]bool)

				for l, verb := range fileType.Verbs {
					v.fileTypeVerb(fmt.Sprintf("%s.verbs[%d]", typePath, l), verb, verbs)
				}
			}
		}
	}
}

func (v *validator) progID(field, progID string, seen map[string]string) {
	if !v.required(field, progID) {
		return
	}

	if problem := progIDProblem(progID); problem != "" {
		v.errorf(field, "%q %s", progID, problem)

		return
	}

	if other, dup := seen[strings.ToLower(progID)]; dup {
		v.errorf(field, "%s registers %q too", other, progID)
	}

	seen[strings.ToLower(progID)] = field

	// Microsoft's rules for COM ProgIDs; Explorer itself accepts more.
	if len(progID) > maxProgIDLength || unicode.IsDigit(rune(progID[0])) ||
		strings.ContainsFunc(progID, func(r rune) bool { return r != '.' && !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		v.warnf(field, "%q is not a COM-style ProgID (at most %d letters, digits and periods, not starting with a digit)", progID, maxProgIDLength)
	}
}

func (v *validator) fileTypeExtension(field string, ext *FileTypeExtension, defaults map[string]string, listed map[string]bool) {
	if ext == nil {
		v.errorf(field, "empty entry")

		return
	}

	name := ext.Extension
	if !v.required(field+".extension", name) {
		return
	}

	if problem := extensionProblem(name); problem != "" {
		v.errorf(field+".extension", "%q %s", name, problem)

		return
	}

	lower := strings.ToLower(name)
	if listed[lower] {
		v.errorf(field+".extension", ".%s is listed twice", name)

		return
	}

	listed[lower] = true

	if ext.OpenWithOnly {
		if ext.ContentType != "" {
			v.errorf(field+".contentType", "openWithOnly leaves the content type of .%s alone", name)
		}

		return
	}

	if other, dup := defaults[lower]; dup {
		v.errorf(field+".extension", "%s registers .%s too; the last one written wins (set openWithOnly on one of them)", other, name)
	}

	defaults[lower] = field

	if systemExtensions[lower] {
		v.warnf(field+".extension", ".%s belongs to Windows or other programs: installing takes it over as its default program; "+
			"openWithOnly adds the program to its Open with menu instead", name)
	}
}

func (v *validator) fileTypeVerb(field string, verb *FileTypeVerb, seen map[string]bool) {
	if verb == nil {
		v.errorf(field, "empty entry")

		return
	}

	if !v.required(field+".id", verb.ID) {
		return
	}

	if problem := classKeyProblem(verb.ID); problem != "" {
		v.errorf(field+".id", "%q %s", verb.ID, problem)

		return
	}

	if len(verb.ID) > maxVerbLength {
		v.errorf(field+".id", "verb %q is longer than %d characters, the width of the Verb table's Verb column", verb.ID, maxVerbLength)
	}

	if seen[strings.ToLower(verb.ID)] {
		v.errorf(field+".id", "verb %q is listed twice", verb.ID)
	}

	seen[strings.ToLower(verb.ID)] = true
}

// progIDProblem reports why progID cannot be a ProgID, or "".
func progIDProblem(progID string) string {
	switch {
	case strings.HasPrefix(progID, "."):
		return "starts with a dot, which makes it an extension key"
	// HKCR\* holds every file's verbs, and +, - and * are the Registry
	// table's key operators in the OpenWithProgids value name.
	case progID == "*" || progID == "+" || progID == "-":
		return "is not a ProgID"
	default:
		return classKeyProblem(progID)
	}
}

// extensionProblem reports why name cannot be an extension, or "".
func extensionProblem(name string) string {
	switch {
	case strings.HasPrefix(name, "."):
		return "has a dot: write the extension without it"
	case classKeyProblem(name) != "" || strings.ContainsAny(name, " *?."):
		return "is not a file name extension"
	default:
		return ""
	}
}

// classKeyProblem reports why name cannot be one key below
// HKEY_CLASSES_ROOT, or "".
func classKeyProblem(name string) string {
	switch {
	case strings.ContainsAny(name, `\[]{}`):
		return `must not hold \, [, ], { or }`
	case strings.TrimSpace(name) != name:
		return "must not start or end with a space"
	default:
		return ""
	}
}
