package config_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestNullEntries checks that a null entry in any list of the schema is an
// error with its path, not a panic in the defaults or the checks.
func TestNullEntries(t *testing.T) {
	t.Parallel()

	const head = `schema: msi-builder/v1
product: { name: App, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
`

	const file = "{ source: app.exe, "

	for path, doc := range map[string]string{
		"directories[0]":                      "directories: [null]\n",
		"directories[0].children[0]":          "directories: [{ id: ProgramFiles64Folder, children: [~] }]\n",
		"features[0]":                         "features: [null]\n",
		"features[0].components[0]":           "features: [{ id: Main, components: [null] }]\n",
		"features[0].features[0]":             "features: [{ id: Main, features: [null] }]\n",
		"features[0].firewall[0]":             "features: [{ id: Main, firewall: [null] }]\n",
		"components[0]":                       "components: [null]\n",
		"components[0].files[0]":              "components: [{ files: [null] }]\n",
		"components[0].registry[0]":           "components: [{ registry: [null] }]\n",
		"components[0].removeRegistry[0]":     "components: [{ removeRegistry: [null] }]\n",
		"components[0].removeFiles[0]":        "components: [{ removeFiles: [null] }]\n",
		"components[0].services[0]":           "components: [{ services: [null] }]\n",
		"components[0].eventLog[0]":           "components: [{ eventLog: [null] }]\n",
		"components[0].environment[0]":        "components: [{ environment: [null] }]\n",
		"components[0].files[0].shortcuts[0]": "components: [{ files: [" + file + "shortcuts: [null] }] }]\n",
		"components[0].files[0].fileTypes[0]": "components: [{ files: [" + file + "fileTypes: [null] }] }]\n",
		"components[0].files[0].fileTypes[0].verbs[0]": "components: [{ files: [" + file +
			"fileTypes: [{ progId: Acme.Doc, extensions: [{ extension: doc }], verbs: [null] }] }] }]\n",
		"components[0].files[0].fileTypes[0].extensions[0]": "components: [{ files: [" + file + "fileTypes: [{ progId: Acme.Doc, extensions: [null] }] }] }]\n",
		"customActions[0]": "customActions: [null]\n",
		"firewall[0]":      "firewall: [null]\n",
		"tables[0]":        "tables: [null]\n",
		"ui.dialogs[0]":    "ui: { wizard: minimal, dialogs: [null] }\n",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			if _, err := config.Parse([]byte(head+doc), config.Options{}); err == nil || !strings.Contains(err.Error(), path+": empty entry") {
				t.Errorf("parse: %v, want %s: empty entry", err, path)
			}
		})
	}
}

// TestValidateNullEntries checks that Validate reports a nil entry of a
// Config built in Go instead of panicking.
func TestValidateNullEntries(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Components: []*config.Component{{ID: "App", Files: []*config.File{nil}}},
		Tables:     []*config.Table{nil},
		UI:         config.UI{Wizard: "minimal", Dialogs: []*config.Dialog{nil}},
	}

	diags := config.Validate(cfg)

	got := make([]string, 0, len(diags))
	for _, d := range diags {
		got = append(got, d.Path+": "+d.Message)
	}

	want := []string{"components[0].files[0]: empty entry", "ui.dialogs[0]: empty entry", "tables[0]: empty entry"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("diagnostics\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if diags.Err() == nil {
		t.Error("Err is nil")
	}
}
