package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// fileTypes writes the file types of every file as Registry rows in the
// file's component, the way non-advertised ProgId, Extension and Verb
// elements are written (specification section 10.2): HKCR, which the engine
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

				writer.value(progID, "", fileType.Description)

				if fileType.IconIndex != nil {
					writer.value(progID+`\DefaultIcon`, "", fmt.Sprintf("%s,%d", target, *fileType.IconIndex))
				}

				for _, verb := range fileType.Verbs {
					key := progID + `\shell\` + verb.ID
					writer.value(key, "", verb.Text)
					writer.value(key+`\command`, "", strings.TrimSpace(target+" "+*verb.Arguments))
				}

				for _, ext := range fileType.Extensions {
					writer.value("."+ext.Extension, "", progID)
					writer.value("."+ext.Extension, "Content Type", ext.ContentType)
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

// value writes a string value; an empty one is left out, and a key without
// values is created by the values below it.
func (w classWriter) value(key, name, value string) {
	if value == "" {
		return
	}

	if strings.HasPrefix(value, "#") {
		value = "#" + value
	}

	key = w.prefix + key
	id := generatedID("reg", "FileType", w.comp, strconv.Itoa(w.root), strings.ToLower(key), strings.ToLower(name))
	w.b.table("Registry").Add(id, w.root, key, nullable(name), value, w.comp)
}
