package config_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/pkg/config"
	"go.yaml.in/yaml/v3"
)

func load(t *testing.T, path string, vars map[string]string) *config.Config {
	t.Helper()

	cfg, err := config.Load(path, config.Options{Vars: vars})
	if err != nil {
		t.Fatal(err)
	}

	return cfg
}

func TestExamplesValidate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		path string
		vars map[string]string
	}{
		{"../../examples/minimal/msi.yaml", nil},
		{"../../examples/windows_exporter/msi.yaml", map[string]string{"VERSION": "0.32.0"}},
		{"../../examples/windows_exporter/msi.yaml", map[string]string{"VERSION": "0.32.0", "ARCH": "arm64"}},
		{"../../examples/branded/msi.yaml", map[string]string{"VERSION": "1.5.0"}},
		{"../../examples/desktop/msi.yaml", map[string]string{"VERSION": "2.0.0"}},
	} {
		cfg := load(t, tc.path, tc.vars)

		diags := config.Validate(cfg)
		if err := diags.Err(); err != nil {
			t.Errorf("%s: %v", tc.path, err)
		}

		for _, w := range diags.Warnings() {
			t.Errorf("%s: unexpected %s", tc.path, w)
		}
	}
}

func TestWindowsExporterExample(t *testing.T) {
	t.Parallel()

	cfg := load(t, "../../examples/windows_exporter/msi.yaml", map[string]string{"VERSION": "0.32.0", "ARCH": "AMD64"})

	if cfg.Package.Platform != config.PlatformX64 {
		t.Errorf("platform = %q, want the exact token x64", cfg.Package.Platform)
	}

	if got := cfg.Features[0].Title; got != "windows_exporter 0.32.0" {
		t.Errorf("feature title = %q", got)
	}

	if got := cfg.Properties["LISTEN_PORT"].Value; got != "9182" {
		t.Errorf("LISTEN_PORT = %q", got)
	}

	if !cfg.Properties["SERVICE_PASSWORD"].Hidden {
		t.Error("SERVICE_PASSWORD is not hidden")
	}

	svc := cfg.Components[0].Services[0]
	if !*svc.Control.DeleteOnUninstall || svc.Recovery.RestartDelay != config.Duration(config.DefaultRestartDelay) {
		t.Errorf("service defaults: %+v", svc)
	}

	// waitForStart is set; wait keeps its default.
	if !*svc.Control.Wait || svc.Control.WaitForStart == nil || *svc.Control.WaitForStart {
		t.Errorf("service control: wait %v, waitForStart %v", *svc.Control.Wait, *svc.Control.WaitForStart)
	}

	if cfg.Product.ProductCode != config.Auto || cfg.Product.Description != "windows_exporter 0.32.0 installer" {
		t.Errorf("product defaults: %+v", cfg.Product)
	}

	if u := cfg.Upgrade; u.Property != config.DefaultUpgradeProperty || !*u.AllowSameVersion {
		t.Errorf("upgrade defaults: %+v", u)
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		yaml string
		want string
	}{
		"unknown key":      {"schema: msi-builder/v1\nproduct: {nickname: x}\n", "nickname"},
		"unknown nested":   {"schema: msi-builder/v1\nproperties: {A: {valeu: x}}\n", "valeu"},
		"schema":           {"schema: msi-builder/v2\n", "msi-builder/v1"},
		"former schema":    {"schema: go-msi/v1\n", "before the rename to msi-builder; write \"msi-builder/v1\""},
		"undefined var":    {"schema: msi-builder/v1\nproduct: {version: \"${NOPE}\"}\n", "undefined variable ${NOPE}"},
		"platform":         {"schema: msi-builder/v1\npackage: {platform: ia64}\n", "unknown platform"},
		"scope":            {"schema: msi-builder/v1\npackage: {scope: global}\n", "unknown scope"},
		"property list":    {"schema: msi-builder/v1\nproperties: {A: [1]}\n", "value or a mapping"},
		"not a mapping":    {"- a\n", "mapping"},
		"bad restartDelay": {"schema: msi-builder/v1\ncomponents: [{services: [{recovery: {restartDelay: soon}}]}]\n", "duration"},
		"bad step delay":   {"schema: msi-builder/v1\ncomponents: [{services: [{recovery: {actions: [{restart: soon}]}}]}]\n", "duration"},
		"bad step key":     {"schema: msi-builder/v1\ncomponents: [{services: [{recovery: {actions: [{reboot: 1m}]}}]}]\n", "{restart: <delay>}"},
		"null step delay":  {"schema: msi-builder/v1\ncomponents: [{services: [{recovery: {actions: [{restart: ~}]}}]}]\n", "needs a delay"},
	} {
		_, err := config.Parse([]byte(tc.yaml), config.Options{})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
}

func TestVariables(t *testing.T) {
	t.Parallel()

	doc := `schema: msi-builder/v1
vars: {NAME: fromFile, PORT: "1"}
product: {name: "${NAME}", version: "$${LITERAL}"}
package:
  platform: x64
  codepage: ${CP}
properties: {P: "${PORT}"}
`
	env := func(k string) (string, bool) {
		if k == "CP" {
			return "65001", true
		}

		if k == "NAME" {
			return "fromEnv", true
		}

		return "", false
	}

	cfg, err := config.Parse([]byte(doc), config.Options{Vars: map[string]string{"PORT": "9"}, LookupEnv: env})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Product.Name != "fromEnv" {
		t.Errorf("name = %q: the environment overrides the file", cfg.Product.Name)
	}

	if cfg.Product.Version != "${LITERAL}" {
		t.Errorf("version = %q: $${ is a literal", cfg.Product.Version)
	}

	if cfg.Package.Codepage != 65001 {
		t.Errorf("codepage = %d: a substituted integer decodes into an int field", cfg.Package.Codepage)
	}

	if cfg.Properties["P"].Value != "9" {
		t.Errorf("P = %q: --var overrides everything", cfg.Properties["P"].Value)
	}
}

func TestPlatformTokens(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]config.Platform{
		"x86": config.PlatformX86, "Intel": config.PlatformX86, "X64": config.PlatformX64,
		"amd64": config.PlatformX64, "ARM64": config.PlatformArm64, "aarch64": config.PlatformArm64,
	} {
		cfg, err := config.Parse([]byte("schema: msi-builder/v1\npackage: {platform: "+input+"}\n"), config.Options{})
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}

		if cfg.Package.Platform != want {
			t.Errorf("%s: got %q, want %q", input, cfg.Package.Platform, want)
		}
	}
}

func TestValidateGoAPI(t *testing.T) {
	t.Parallel()

	// A Config built in Go, without Load, must not panic on unset pointers.
	cfg := &config.Config{Schema: config.SchemaV1, Features: []*config.Feature{{ID: "F"}},
		Components: []*config.Component{{ID: "C", Directory: "D", Services: []*config.Service{{Name: "s"}}}}}

	if err := config.Validate(cfg).Err(); err == nil {
		t.Error("an incomplete config validated")
	}
}

// TestRecoveryActionsMarshal keeps every recovery step in the marshalled
// config, from which the builder derives the product code.
func TestRecoveryActionsMarshal(t *testing.T) {
	t.Parallel()

	doc := "schema: msi-builder/v1\ncomponents: [{services: [{recovery: {actions: [restart, none, {restart: 5m}]}}]}]\n"

	cfg, err := config.Parse([]byte(doc), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	out, err := yaml.Marshal(cfg.Components[0].Services[0].Recovery.Actions)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := string(out), "- restart\n- none\n- restart: 5m0s\n"; got != want {
		t.Errorf("marshalled actions %q, want %q", got, want)
	}
}

// FuzzVariable findings: a value that spells null emptied a string field,
// and a value that is not valid UTF-8 reached the config.
func TestVariableValues(t *testing.T) {
	t.Parallel()

	doc := "schema: msi-builder/v1\nproduct:\n  name: ${V}\n  manufacturer: '${V}'\npackage: {platform: x64}\n"

	for _, value := range []string{"null", "Null", "NULL", "~"} {
		cfg, err := config.Parse([]byte(doc), config.Options{Vars: map[string]string{"V": value}})
		if err != nil {
			t.Fatal(err)
		}

		if cfg.Product.Name != value || cfg.Product.Manufacturer != value {
			t.Errorf("${V} = %q gives name %q and manufacturer %q", value, cfg.Product.Name, cfg.Product.Manufacturer)
		}
	}

	if _, err := config.Parse([]byte(doc), config.Options{Vars: map[string]string{"V": "\xff"}}); err == nil {
		t.Error("a variable that is not valid UTF-8 was accepted")
	}
}
