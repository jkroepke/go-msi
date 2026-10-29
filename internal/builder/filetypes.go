package builder

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// fileTypes writes the file types of every file into the ProgId, Extension
// and Verb tables, which the engine's RegisterExtensionInfo and
// RegisterProgIdInfo actions register on install and their Unregister
// counterparts remove (specification section 10.2). The engine also tells
// the shell that associations changed (REG-05). A file type needs at least
// one extension it owns: the engine registers a ProgId without a class only
// through an Extension row with a verb. The rest goes into Registry rows of
// the file's component: the description, DefaultIcon, the extension's Content Type and its
// OpenWithProgids entry. A file type whose extensions are all openWithOnly
// owns no extension, so its ProgID, description and verbs are Registry rows
// too.
//
// The Registry rows use HKCR, which the engine maps to
// HKLM\Software\Classes per machine and HKCU\Software\Classes per user. A
// per-user package names HKCU\Software\Classes itself (ICE105); a dual one
// keeps HKCR, which follows the context it installs in (INST-02).
func (b *build) fileTypes() error {
	root, prefix := registryRoots["HKCR"], ""
	if b.cfg.Package.Scope == config.ScopePerUser {
		root, prefix = registryRoots["HKCU"], `Software\Classes\`
	}

	for i, comp := range b.cfg.Components {
		writer := classWriter{b: b, comp: comp.ID, root: root, prefix: prefix}

		for j, file := range comp.Files {
			for k, fileType := range file.FileTypes {
				b.fileType(fmt.Sprintf("%s.files[%d].fileTypes[%d]", comp.Path(i), j, k), writer, file, fileType)
			}
		}
	}

	return nil
}

// fileType writes one file type of file, which writer's component holds.
func (b *build) fileType(path string, writer classWriter, file *config.File, fileType *config.FileType) {
	target := `"[#` + config.FileKey(file) + `]"`

	if owned := ownedExtensions(fileType); len(owned) > 0 {
		if info := b.components[writer.comp]; info.keyFile == nil || info.keyFile.cfg != file {
			b.errorf(path, "the engine runs the verbs with the key path of the component (observed, REG-06), so %s must be the key path of %s: "+
				"set keyPath or give the file a component of its own", file.Source, writer.comp)
		}

		// An Extension row names one feature, and the engine registers it
		// only when that feature is installed, so a component installed
		// through another feature would leave the type half registered.
		features := b.componentFeatures(writer.comp)
		if len(features) > 1 {
			b.errorf(path, "the Extension table registers an extension with one feature, so %s must be in one feature only, not %s: "+
				"give the file a component of its own", writer.comp, strings.Join(features, ", "))
		}

		if len(features) == 1 {
			b.nativeFileType(writer.comp, features[0], fileType, owned)
		}

		writer.value(fileType.ProgID, "", escapeFormatted(fileType.Description))
	} else {
		writeProgID(writer, target, fileType)
	}

	if fileType.IconIndex != nil {
		writer.value(fileType.ProgID+`\DefaultIcon`, "", fmt.Sprintf("%s,%d", target, *fileType.IconIndex))
	}

	for _, ext := range fileType.Extensions {
		if !ext.OpenWithOnly {
			writer.value("."+ext.Extension, "Content Type", escapeFormatted(ext.ContentType))
		}

		writer.emptyValue("."+ext.Extension+`\OpenWithProgids`, fileType.ProgID)
	}
}

// ownedExtensions returns the extensions whose default the file type sets.
func ownedExtensions(fileType *config.FileType) []*config.FileTypeExtension {
	var owned []*config.FileTypeExtension

	for _, ext := range fileType.Extensions {
		if !ext.OpenWithOnly {
			owned = append(owned, ext)
		}
	}

	return owned
}

// nativeFileType writes the ProgId row of a file type and an Extension row
// with the verbs for each extension it owns, in the feature that installs
// the component. Verbs are numbered in their order, so the first
// one is the shell key's default.
func (b *build) nativeFileType(compID, feature string, fileType *config.FileType, owned []*config.FileTypeExtension) {
	// RegisterProgIdInfo leaves the ProgID's default unset for a ProgId
	// row without a class (observed, REG-06), so the description is a
	// Registry row (fileType).
	b.table("ProgId").Add(fileType.ProgID, nil, nil, nil, nil, nil)

	for _, ext := range owned {
		b.table("Extension").Add(ext.Extension, compID, fileType.ProgID, nil, feature)

		for i, verb := range fileType.Verbs {
			b.table("Verb").Add(ext.Extension, verb.ID, i+1, nullable(escapeFormatted(verb.Text)), nullable(*verb.Arguments))
		}
	}
}

// componentFeatures returns the features, in tree order, that install
// compID. Validation puts every component in at least one.
func (b *build) componentFeatures(compID string) []string {
	var features []string

	config.WalkFeatures(b.cfg.Features, func(feature, _ *config.Feature) {
		for _, comp := range feature.Components {
			if comp.ID == compID && !slices.Contains(features, feature.ID) {
				features = append(features, feature.ID)
			}
		}
	})

	return features
}

// writeProgID writes a ProgID that owns no extension as Registry rows: its
// description, and each verb's text and command.
func writeProgID(writer classWriter, target string, fileType *config.FileType) {
	progID := fileType.ProgID

	writer.value(progID, "", escapeFormatted(fileType.Description))

	for _, verb := range fileType.Verbs {
		key := progID + `\shell\` + verb.ID
		writer.value(key, "", escapeFormatted(verb.Text))
		writer.value(key+`\command`, "", strings.TrimSpace(target+" "+*verb.Arguments))
	}
}

// classWriter writes the Registry rows of one file type.
type classWriter struct {
	b      *build
	comp   string
	prefix string
	root   int
}

// value writes a string value of formatted text; an empty one is left out,
// and a key without values is created by the values below it.
func (w classWriter) value(key, name, value string) {
	if value == "" {
		return
	}

	if strings.HasPrefix(value, "#") {
		value = "#" + value
	}

	w.add(key, name, value)
}

// emptyValue writes a named value with a Null Value column, which the
// engine stores as an empty REG_SZ (REG-04; Microsoft documents a Null
// Value only for the names +, - and *). Shell lists such as
// OpenWithProgids name their entries this way.
func (w classWriter) emptyValue(key, name string) {
	w.add(key, escapeFormatted(name), nil)
}

// add writes one Registry row.
func (w classWriter) add(key, name string, value any) {
	key = w.prefix + key
	id := generatedID("reg", "FileType", w.comp, strconv.Itoa(w.root), strings.ToLower(key), strings.ToLower(name))
	w.b.table("Registry").Add(id, w.root, key, nullable(name), value, w.comp)
}
