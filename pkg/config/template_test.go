package config_test

import (
	"os"
	"path/filepath"
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
			want: `template: line 8:15: at <.ARCH>: variable ARCH is not set; pass --var ARCH=VALUE or add it to vars`,
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
		"nil vars": {
			doc:  "x: {{ .X }}\n",
			opts: config.Options{Template: true},
			want: "variable X is not set",
		},
		"empty separator": {
			doc:  "x: {{ split \"\" \"abc\" }}\n",
			opts: config.Options{Template: true},
			want: "the separator is empty",
		},
		"substitution line": {
			doc:  "schema: msi-builder/v1\n{{- \"\" }}\nproduct:\n  name: ${NOPE}\n",
			opts: config.Options{Template: true},
			want: "undefined variable ${NOPE} (lines of the rendered file",
		},
		"decodes without template": {
			doc:  "schema: msi-builder/v1\nproduct:\n  name: {{ .ARCH }}\n",
			opts: config.Options{},
			want: "pass --template",
		},
		"template in vars": {
			doc:  "vars:\n  A: {{ .B }}\nx: 1\n",
			opts: config.Options{Template: true},
			want: "must not use template syntax",
		},
		"vars cycle": {
			doc:  "x: {{ .A }}\nvars:\n  A: ${B}\n  B: ${A}\n",
			opts: config.Options{Template: true},
			want: "line 4: vars.B: ${A} -> ${B} -> ${A} is a cycle",
		},
		"two defaults": {
			doc:  "x: {{ var \"X\" \"a\" \"b\" }}\n",
			opts: config.Options{Template: true},
			want: "at most one default",
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

// TestTemplateQuote checks that quote delivers values as given, also ones
// that look like ${NAME} references or HTML.
func TestTemplateQuote(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"${Y}", "$${Y}", "a<b&c>", `"q" \ x`, "line\nbreak", ""} {
		doc := strings.Replace(base, "manufacturer: Example", `manufacturer: {{ var "V" | quote }}`, 1) + appComponent

		cfg, err := config.Parse([]byte(doc), config.Options{Vars: map[string]string{"V": value}, Template: true})
		if err != nil {
			t.Errorf("%q: %v", value, err)

			continue
		}

		if cfg.Product.Manufacturer != value {
			t.Errorf("%q arrives as %q", value, cfg.Product.Manufacturer)
		}
	}

	out, err := config.Render([]byte(`{{ quote "a<b" }}`), config.Options{})
	if err != nil || string(out) != `"a<b"` {
		t.Errorf("got %q, %v", out, err)
	}
}

// TestTemplateLoad checks that Load names the file in a template error.
func TestTemplateLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("x: {{ .X }}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path, config.Options{Template: true})
	if err == nil || !strings.HasPrefix(err.Error(), path+": template: line 1:6: at <.X>:") {
		t.Errorf("got %v", err)
	}
}

// TestTemplateHints checks which errors get a hint.
func TestTemplateHints(t *testing.T) {
	t.Parallel()

	// A {{ in a plain file that parses does not cause an undefined ${V}.
	doc := strings.Replace(base, "manufacturer: Example", `manufacturer: "{{x}} ${V}"`, 1) + appComponent

	_, err := config.Parse([]byte(doc), config.Options{})
	if err == nil || strings.Contains(err.Error(), "--template") {
		t.Errorf("plain file: got %v", err)
	}

	// An error without a line gets no note about rendered lines.
	_, err = config.Parse([]byte("schema: x\n"), config.Options{Template: true})
	if err == nil || strings.Contains(err.Error(), "rendered file") {
		t.Errorf("schema: got %v", err)
	}
}

// TestTemplateVars checks that templates find variables as ${NAME} does:
// --var, then the environment, then the file's vars.
func TestTemplateVars(t *testing.T) {
	t.Parallel()

	doc := `# comment
vars:
  ARCH: x64
  NAME: Acme ${ARCH}

  EMPTY: ""
x: {{ .ARCH }} {{ var "NAME" }} {{ has "EMPTY" }} {{ has "NOPE" }} ${NAME}
`
	env := func(name string) (string, bool) {
		if name == "NAME" {
			return "env", true
		}

		return "", false
	}

	for _, test := range []struct {
		opts config.Options
		want string
	}{
		{config.Options{}, "x: x64 Acme x64 true false ${NAME}\n"},
		{config.Options{Vars: map[string]string{"ARCH": "arm64"}}, "x: arm64 Acme arm64 true false ${NAME}\n"},
		{config.Options{LookupEnv: env}, "x: x64 env true false ${NAME}\n"},
	} {
		out, err := config.Render([]byte(doc), test.opts)
		if err != nil {
			t.Fatal(err)
		}

		if got := string(out[strings.Index(string(out), "x: "):]); got != test.want {
			t.Errorf("got %q, want %q", got, test.want)
		}
	}
}

// TestTemplateDecodeError checks that an unknown key, whose line is that of
// the re-encoded document, gets no note about rendered lines.
func TestTemplateDecodeError(t *testing.T) {
	t.Parallel()

	_, err := config.Parse([]byte(base+appComponent+"\n\n# c\nbogus: {{ 1 }}\n"), config.Options{Template: true})
	if err == nil || !strings.Contains(err.Error(), "bogus") || strings.Contains(err.Error(), "rendered file") {
		t.Errorf("got %v", err)
	}
}
