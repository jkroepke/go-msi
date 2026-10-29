package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// fileTypes writes the file types of every file as Registry rows in the
// file's component, the way non-advertised ProgId, Extension and Verb
// elements are written (specification section 10.2), plus the ProgID in
// each extension's OpenWithProgids list: HKCR, which the engine maps to
// HKLM\Software\Classes per machine and HKCU\Software\Classes per user. A
// per-user package names HKCU\Software\Classes itself (ICE105).
//
// An extension's default value goes into a permanent component of its own,
// so that uninstalling leaves it in place, as Microsoft asks: another
// program may have taken the extension over since, and Windows ignores a
// default that names a ProgID nobody registers. Its Content Type goes with
// it, which is msi-builder's choice: the value describes the extension, not
// the program. The package then tells the shell that associations changed
// (scheduleAssocChanged).
func (b *build) fileTypes() error {
	root, prefix := registryRoots["HKCR"], ""
	if b.cfg.Package.Scope == config.ScopePerUser {
		root, prefix = registryRoots["HKCU"], `Software\Classes\`
	}

	for _, comp := range b.cfg.Components {
		writer := classWriter{b: b, comp: comp.ID, root: root, prefix: prefix}

		for _, file := range comp.Files {
			for _, fileType := range file.FileTypes {
				b.assocChange = true

				writeFileType(writer, config.FileKey(file), fileType)

				for _, ext := range fileType.Extensions {
					if !ext.OpenWithOnly {
						b.extensionDefault(b.components[comp.ID], writer, ext, fileType.ProgID)
					}
				}
			}
		}
	}

	if b.assocChange {
		b.addAssocChanged()
	}

	return nil
}

// addAssocChanged adds the custom action DLL and the type-1 action on its
// BuilderAssocChanged entry, whose name a custom action may not take.
func (b *build) addAssocChanged() {
	for i, custom := range b.cfg.CustomActions {
		if custom.ID == cadll.AssocChangedEntry {
			b.errorf(fmt.Sprintf("customActions[%d].id", i), "%s is reserved for the action that tells the shell about file types", custom.ID)
		}
	}

	if b.useRunDLL() {
		b.addAction(&actionInfo{name: cadll.AssocChangedEntry, typ: caDLL | caContinue, source: cadll.Key, target: cadll.AssocChangedEntry})
	}
}

// writeFileType writes the rows of one file type that live in the file's
// component: the ProgID, its icon and verbs, and its OpenWithProgids
// entries.
func writeFileType(writer classWriter, fileKey string, fileType *config.FileType) {
	target := `"[#` + fileKey + `]"`
	progID := fileType.ProgID

	writer.value(progID, "", escapeFormatted(fileType.Description))

	if fileType.IconIndex != nil {
		writer.value(progID+`\DefaultIcon`, "", fmt.Sprintf("%s,%d", target, *fileType.IconIndex))
	}

	for _, verb := range fileType.Verbs {
		key := progID + `\shell\` + verb.ID
		writer.value(key, "", escapeFormatted(verb.Text))
		writer.value(key+`\command`, "", strings.TrimSpace(target+" "+*verb.Arguments))
	}

	for _, ext := range fileType.Extensions {
		writer.emptyValue("."+ext.Extension+`\OpenWithProgids`, progID)
	}
}

// extensionDefault writes the permanent component that holds the default
// value and Content Type of one extension, with the default as key path.
// It follows info's folder, bitness, condition and features (features).
// The GUID follows the extension, the component's only key path: renaming
// the extension gives a new component and leaves the old one's value behind,
// as uninstalling does.
func (b *build) extensionDefault(info *componentInfo, fileWriter classWriter, ext *config.FileTypeExtension, progID string) {
	comp := info.cfg
	bitness := "32"
	attributes := compRegistryKeyPath | compPermanent | info.attributes&compTransitive

	if info.is64 {
		bitness = "64"
		attributes |= comp64Bit
	}

	lower := strings.ToLower(ext.Extension)
	carrier := generatedID("cmp", "FileTypeDefault", b.upgradeCode, bitness, lower)
	writer := fileWriter
	writer.comp = carrier

	keyPath := writer.value("."+ext.Extension, "", progID)
	writer.value("."+ext.Extension, "Content Type", escapeFormatted(ext.ContentType))

	guid := uuid5(namespaceComponent, b.upgradeCode+"|"+bitness+"|filetype|"+strconv.Itoa(writer.root)+"|"+lower)

	b.table("Component").Add(carrier, guid, comp.Directory, attributes, nullable(comp.Condition), keyPath)
	b.fileTypeCarrier[comp.ID] = append(b.fileTypeCarrier[comp.ID], carrier)
}

// scheduleAssocChanged places the DLL entry point that calls SHChangeNotify
// with SHCNE_ASSOCCHANGED at the end of the execute sequence, on install,
// repair and removal alike: after InstallFinalize, or after
// RemoveExistingProducts when that follows InstallFinalize, so that the
// old product's removal in a major upgrade is covered too. It is
// immediate, so it runs once the registry is written, in the custom action
// server of the installing user (CA-03 saw the user's token; REG-05 logs
// the session), and its return value is ignored: a missed notification
// only delays what Explorer shows.
func (b *build) scheduleAssocChanged() error {
	if !b.assocChange || b.action(cadll.AssocChangedEntry) == nil {
		return nil
	}

	after := actInstallFinalize

	for _, scheduled := range b.schedules[seqInstallExecute] {
		if scheduled.Name == "RemoveExistingProducts" && scheduled.After == actInstallFinalize {
			after = scheduled.Name
		}
	}

	b.schedule(seqInstallExecute, cadll.AssocChangedEntry, "", "", after, 0)

	return nil
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
func (w classWriter) value(key, name, value string) string {
	if value == "" {
		return ""
	}

	if strings.HasPrefix(value, "#") {
		value = "#" + value
	}

	return w.add(key, name, value)
}

// emptyValue writes a named value with a Null Value column, which the
// engine stores as an empty REG_SZ (REG-04; Microsoft documents a Null
// Value only for the names +, - and *). Shell lists such as
// OpenWithProgids name their entries this way.
func (w classWriter) emptyValue(key, name string) {
	w.add(key, escapeFormatted(name), nil)
}

// add writes one Registry row and returns its key.
func (w classWriter) add(key, name string, value any) string {
	key = w.prefix + key
	id := generatedID("reg", "FileType", w.comp, strconv.Itoa(w.root), strings.ToLower(key), strings.ToLower(name))
	w.b.table("Registry").Add(id, w.root, key, nullable(name), value, w.comp)

	return id
}
