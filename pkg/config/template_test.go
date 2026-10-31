package config_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// templated is base with the platform and an arm64-only component chosen
// by a template.
const templated = `schema: msi-builder/v1
product:
  name: {{ var "NAME" "App" | quote }}
  manufacturer: Example
  version: ${VERSION}
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
package:
  platform: {{ .ARCH }}
upgrade: {}
directories:
  - id: ProgramFiles64Folder
    children:
      - id: INSTALLDIR
        name: App
        configurable: true
features:
  - id: Main
    components:
      - App
{{- if eq .ARCH "arm64" }}
      - Arm
{{- end }}
components:
  - id: App
    directory: INSTALLDIR
    files:
      - source: app.exe
{{- if eq .ARCH "arm64" }}
  - id: Arm
    directory: INSTALLDIR
    files:
      - source: arm.dll
{{- end }}
`

func TestTemplate(t *testing.T) {
	t.Parallel()

	for arch, want := range map[string]int{"x64": 1, "arm64": 2} {
		cfg, err := config.Parse([]byte(templated), config.Options{
			Vars:     map[string]string{"ARCH": arch, "VERSION": "1.2.3"},
			Template: true,
		})
		if err != nil {
			t.Fatalf("%s: %v", arch, err)
		}

		if got := len(cfg.Components); got != want {
			t.Errorf("%s: %d components, want %d", arch, got, want)
		}

		if !strings.EqualFold(string(cfg.Package.Platform), arch) || cfg.Product.Name != "App" || cfg.Product.Version != "1.2.3" {
			t.Errorf("%s: got platform %q, name %q, version %q", arch, cfg.Package.Platform, cfg.Product.Name, cfg.Product.Version)
		}

		if ds := config.Validate(cfg); ds.Err() != nil {
			t.Errorf("%s: %v", arch, ds)
		}
	}
}

func TestTemplateEnv(t *testing.T) {
	t.Parallel()

	env := func(name string) (string, bool) {
		if name == "NAME" {
			return `a "quoted": name`, true
		}

		return "", false
	}

	cfg, err := config.Parse([]byte(templated), config.Options{
		Vars:      map[string]string{"ARCH": "x64", "VERSION": "1.0.0"},
		LookupEnv: env,
		Template:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Product.Name != `a "quoted": name` {
		t.Errorf("name = %q", cfg.Product.Name)
	}
}

func TestTemplateErrors(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		doc  string
		opts config.Options
		want string
	}{
		"missing key": {
			doc:  templated,
			opts: config.Options{Vars: map[string]string{"VERSION": "1"}, Template: true},
			want: `template: line 8:15: executing "msi.yaml" at <.ARCH>: map has no entry for key "ARCH"`,
		},
		"var without default": {
			doc:  "schema: msi-builder/v1\nx: {{ var \"NOPE\" }}\n",
			opts: config.Options{Template: true},
			want: "variable NOPE is not set",
		},
		"syntax": {
			doc:  "schema: msi-builder/v1\n{{ if }}\n",
			opts: config.Options{Template: true},
			want: "template: line 2:",
		},
		"too large": {
			doc:  "schema: msi-builder/v1\n{{ range 20000000 }}x{{ end }}\n",
			opts: config.Options{Template: true},
			want: "the template writes more than",
		},
		"indent": {
			doc:  "x: {{ indent 2000000000 \"a\\nb\" }}\n",
			opts: config.Options{Template: true},
			want: "is not between 0 and 1024",
		},
		"quote": {
			doc:  "x: {{ var \"V\" | quote }}\n",
			opts: config.Options{Vars: map[string]string{"V": "\xff"}, Template: true},
			want: "not valid UTF-8",
		},
		"rendered yaml": {
			doc:  "schema: msi-builder/v1\n{{ \"a: [\" }}\n",
			opts: config.Options{Template: true},
			want: "lines of the rendered file",
		},
		"without template": {
			doc:  templated,
			opts: config.Options{Vars: map[string]string{"ARCH": "x64", "VERSION": "1"}},
			want: "pass --template",
		},
	} {
		_, err := config.Parse([]byte(test.doc), test.opts)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: got %v, want %q", name, err, test.want)
		}
	}
}

// TestTemplateHas checks has, indent, split and that ${NAME} passes through a template.
func TestTemplateHas(t *testing.T) {
	t.Parallel()

	doc := "a: {{ if has \"X\" }}set{{ else }}unset{{ end }}\nb: ${X}\nc: {{ indent 2 \"l1\\nl2\" }}\nd:{{ range split \";\" (var \"L\" \"\") }} {{ . }}{{ end }}\n"

	out, err := config.Render([]byte(doc), config.Options{Vars: map[string]string{"X": "1", "L": "en;de"}})
	if err != nil {
		t.Fatal(err)
	}

	if want := "a: set\nb: ${X}\nc: l1\n  l2\nd: en de\n"; string(out) != want {
		t.Errorf("got %q, want %q", out, want)
	}

	out, err = config.Render([]byte(doc), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(string(out), "a: unset\n") || !strings.HasSuffix(string(out), "d:\n") {
		t.Errorf("got %q", out)
	}
}
