package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// fileTypes writes the file types of every file as Registry rows in the
// file's component, the way non-advertised ProgId, Extension and Verb
// elements are written (specification section 10.2), plus the ProgID in
// each extension's OpenWithProgids list: HKCR, which the engine
// maps to HKLM\Software\Classes per machine and HKCU\Software\Classes per
// user. A per-user package names HKCU\Software\Classes itself (ICE105).
func (b *build) fileTypes() error {
	root, prefix := registryRoots["HKCR"], ""
	if b.cfg.Package.Scope == config.ScopePerUser {
		root, prefix = registryRoots["HKCU"], `Software\Classes\`
	}

	for _, comp := range b.cfg.Components {
		for _, file := range comp.Files {
			for _, fileType := range file.FileTypes {
				writer := classWriter{b: b, comp: comp.ID, root: root, prefix: prefix}
				target := `"[#` + config.FileKey(file) + `]"`
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
					key := "." + ext.Extension
					if !ext.OpenWithOnly {
						writer.value(key, "", progID)
						writer.value(key, "Content Type", escapeFormatted(ext.ContentType))
					}

					writer.emptyValue(key+`\OpenWithProgids`, progID)
				}
			}
		}
	}

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
// engine stores as an empty REG_SZ (REG-04). Shell lists such as
// OpenWithProgids name their entries this way.
func (w classWriter) emptyValue(key, name string) {
	w.add(key, escapeFormatted(name), nil)
}

func (w classWriter) add(key, name string, value any) {
	key = w.prefix + key
	id := generatedID("reg", "FileType", w.comp, strconv.Itoa(w.root), strings.ToLower(key), strings.ToLower(name))
	w.b.table("Registry").Add(id, w.root, key, nullable(name), value, w.comp)
}
