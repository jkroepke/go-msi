package config_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/go-msi/pkg/config"
)

// base is a valid package; each case appends to or replaces parts of it.
const base = `schema: go-msi/v1
product:
  name: App
  manufacturer: Example
  version: 1.0.0
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
package:
  platform: x64
upgrade: {}
directories:
  - id: ProgramFiles64Folder
    children:
      - id: INSTALLDIR
        name: App
        configurable: true
features:
  - id: Main
    components: [App]
`

const appComponent = `components:
  - id: App
    directory: INSTALLDIR
    files:
      - source: app.exe
`

// pkg returns base with its package section replaced.
func pkg(flow string) string {
	return strings.Replace(base, "package:\n  platform: x64\n", "package: "+flow+"\n", 1)
}

func validate(t *testing.T, doc string) config.Diagnostics {
	t.Helper()

	cfg, err := config.Parse([]byte(doc), config.Options{})
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, doc)
	}

	return config.Validate(cfg)
}

func find(ds config.Diagnostics, sev config.Severity, path, msg string) bool {
	for _, d := range ds {
		if d.Severity == sev && d.Path == path && strings.Contains(d.Message, msg) {
			return true
		}
	}

	return false
}

func TestBaseIsValid(t *testing.T) {
	t.Parallel()

	if ds := validate(t, base+appComponent); len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %v", ds)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	const (
		e = config.SeverityError
		w = config.SeverityWarning
	)

	for name, tc := range map[string]struct {
		doc  string
		sev  config.Severity
		path string
		msg  string
	}{
		// MSISPEC-RESULTS finding 2 (SUM-03).
		"page count above 500": {
			pkg(`{platform: x64, installerVersion: "5.1"}`) + appComponent,
			e, "package.installerVersion", "1613",
		},
		"x64 below 2.0": {
			pkg(`{platform: x64, installerVersion: "1.5"}`) + appComponent,
			e, "package.installerVersion", "at least 2.0",
		},
		"arm64 below 5.0": {
			pkg(`{platform: arm64, installerVersion: "4.5"}`) + appComponent,
			e, "package.installerVersion", "at least 5.0",
		},
		// RUN-02: an unversioned executable is deleted by an upgrade.
		"unversioned exe": {
			base + appComponent + "      - source: helper.dll\n        version: none\n",
			e, "components[0].files[1].version", "RUN-02",
		},
		"unversioned exe acknowledged": {
			base + appComponent + "      - source: helper.dll\n        version: none\n        allowUnversionedExecutable: true\n",
			w, "components[0].files[1].version", "RUN-02",
		},
		"empty version is not none": {
			base + appComponent + "      - source: data.txt\n        version: \"1.x\"\n",
			e, "components[0].files[1].version", "not auto, none",
		},
		// RUN-03 and RUN-11.
		"four-field version": {
			strings.Replace(base, "version: 1.0.0", "version: 1.0.0.5", 1) + appComponent,
			w, "product.version", "RUN-03",
		},
		"nil upgrade code": {
			strings.Replace(base, "3f2504e0-4f89-41d3-9a0c-0305e82c3301", `"{00000000-0000-0000-0000-000000000000}"`, 1) + appComponent,
			e, "product.upgradeCode", "nil GUID",
		},
		"nil related upgrade code": {
			strings.Replace(base, "upgrade: {}", "upgrade: {related: [{upgradeCode: 00000000-0000-0000-0000-000000000000, maxVersion: 1.0.0}]}", 1) + appComponent,
			e, "upgrade.related[0].upgradeCode", "nil GUID",
		},
		"pinned product code": {
			strings.Replace(base, "  upgradeCode:", "  productCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3302\n  upgradeCode:", 1) + appComponent,
			w, "product.productCode", "1638",
		},
		// RUN-18: exec actions must say how an exit code is handled.
		"exec without return": {
			base + appComponent + `customActions:
  - id: Run
    exec: {command: cmd.exe}
    execute: deferred
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			e, "customActions[0].return", "RUN-18",
		},
		"deferred set property": {
			base + appComponent + `customActions:
  - id: Set
    setProperty: {name: A, value: b}
    execute: deferred
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			e, "customActions[0].execute", "cannot be deferred",
		},
		"no impersonation immediate": {
			base + appComponent + `customActions:
  - id: Set
    setProperty: {name: A, value: b}
    impersonate: false
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			e, "customActions[0].impersonate", "deferred",
		},
		"standard action name": {
			base + appComponent + `customActions:
  - id: InstallFiles
    error: no
    schedule: [{sequence: execute, after: CostFinalize}]
`,
			e, "customActions[0].id", "ICE93",
		},
		"unknown anchor": {
			base + appComponent + `customActions:
  - id: Fail
    error: no
    schedule: [{sequence: execute, after: Nowhere}]
`,
			e, "customActions[0].schedule[0]", "Nowhere",
		},
		// Finding 1 (DB-07) and DB-02.
		"32 columns": {
			base + appComponent + "tables:\n  - name: Wide\n    columns:\n" +
				strings.Repeat("      - {name: C, type: I2, key: true}\n", 32),
			e, "tables[0].columns", "DB-07",
		},
		"i2 minimum": {
			base + appComponent + `tables:
  - name: T
    columns: [{name: K, type: s72, key: true}, {name: V, type: I2}]
    rows: [[a, -32768]]
`,
			e, "tables[0].rows[0][1]", "outside",
		},
		"standard table": {
			base + appComponent + "tables:\n  - name: Property\n    columns: [{name: K, type: s72, key: true}]\n",
			e, "tables[0].name", "written by the builder",
		},
		// DB-11.
		"binary key too long": {
			base + appComponent + "binaries:\n  " + strings.Repeat("b", 56) + ": x.dll\n",
			e, "binaries." + strings.Repeat("b", 56), "DB-11",
		},
		// RUN-08: removal is never implied by scope.
		"environment prefix": {
			base + strings.Replace(appComponent, "    files:", "    environment: [{name: '*PATH', value: x}]\n    files:", 1),
			e, "components[0].environment[0].name", "plain name",
		},
		"PATH set": {
			base + strings.Replace(appComponent, "    files:", "    environment: [{name: PATH, value: x}]\n    files:", 1),
			w, "components[0].environment[0].action", "append",
		},
		// SUM-04.
		"utf-16 codepage": {
			pkg(`{platform: x64, codepage: 1200}`) + appComponent,
			e, "package.codepage", "1200",
		},
		// RUN-05.
		"service keeps registration": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, control: {deleteOnUninstall: false}}]\n    files:", 1),
			w, "components[0].services[0].control.deleteOnUninstall", "RUN-05",
		},
		"service on dll": {
			base + strings.Replace(strings.Replace(appComponent, "app.exe", "app.dll", 1), "    files:", "    services: [{name: svc}]\n    files:", 1),
			e, "components[0].services[0]", "not an .exe",
		},
		"literal password": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, password: hunter2}]\n    files:", 1),
			e, "components[0].services[0].password", "hidden property",
		},
		"recovery reset twice": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {resetAfterDays: 1, resetAfter: 24h, actions: [restart]}}]\n    files:", 1),
			e, "components[0].services[0].recovery.resetAfter", "not both",
		},
		"recovery reset fraction": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {resetAfter: 1500ms, actions: [restart]}}]\n    files:", 1),
			e, "components[0].services[0].recovery.resetAfter", "whole number of seconds",
		},
		"recovery step delay negative": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {actions: [{restart: -1s}]}}]\n    files:", 1),
			e, "components[0].services[0].recovery.actions[0].restart", "negative",
		},
		"recovery step delay too long": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {actions: [{restart: 600h}]}}]\n    files:", 1),
			e, "components[0].services[0].recovery.actions[0].restart", "too long",
		},
		"recovery reset days too long": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {resetAfterDays: 30000, actions: [restart]}}]\n    files:", 1),
			e, "components[0].services[0].recovery.resetAfterDays", "too long",
		},
		"recovery four steps": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {actions: [restart, restart, restart, restart]}}]\n    files:", 1),
			e, "components[0].services[0].recovery.actions", "one to three",
		},
		// RUN-26.
		"purge configurable folder": {
			base + strings.Replace(appComponent, "    files:", "    purge: true\n    files:", 1),
			e, "components[0].purge", "configurable",
		},
		"purge standard folder": {
			base + strings.Replace(appComponent, "directory: INSTALLDIR", "directory: ProgramFiles64Folder\n    purge: true", 1),
			e, "components[0].purge", "standard folder",
		},
		"purge below configurable folder": {
			strings.Replace(base, "        configurable: true\n", "        configurable: true\n        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1),
			e, "components[0].purge", "INSTALLDIR can be pointed",
		},
		"purge nested standard folder": {
			strings.Replace(base, "directories:\n", "directories:\n  - id: TARGETDIR\n    children: [{id: CommonAppDataFolder, name: ProgramData}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: CommonAppDataFolder\n    createFolder: true\n    purge: true", 1),
			e, "components[0].purge", "standard folder",
		},
		"purge below setDirectory": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1) +
				"customActions:\n  - id: MoveIt\n    setDirectory: {directory: INSTALLDIR, value: '[WindowsFolder]'}\n",
			e, "components[0].purge", "INSTALLDIR can be pointed",
		},
		"purge public folder id": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DATADIR, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DATADIR\n    purge: true", 1),
			e, "components[0].purge", "DATADIR can be pointed",
		},
		"purge below TARGETDIR": {
			strings.Replace(base, "directories:\n", "directories:\n  - id: TARGETDIR\n    children: [{id: DataDir, name: Data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    createFolder: true\n    purge: true", 1),
			e, "components[0].purge", "TARGETDIR",
		},
		"purge folder set by a property": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1) +
				"properties:\n  DataDir: 'C:\\'\n",
			e, "components[0].purge", "DataDir can be pointed",
		},
		"purge folder set by setProperty": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1) +
				"customActions:\n  - id: SetIt\n    setProperty: {name: DataDir, value: '[WindowsFolder]'}\n",
			e, "components[0].purge", "DataDir can be pointed",
		},
		"purge condition": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DATADIR, name: data}]\n", 1) + strings.Replace(appComponent, "    files:", "    purge: {condition: 'DELETE_DATA='}\n    files:", 1),
			e, "components[0].purge.condition", "",
		},
		// RUN-24.
		"neverOverwrite with early removal": {
			base + strings.Replace(appComponent, "    files:", "    neverOverwrite: true\n    files:", 1),
			w, "components[0].neverOverwrite", "RUN-24",
		},
		"neverOverwrite service with early removal": {
			base + strings.Replace(appComponent, "    files:", "    neverOverwrite: true\n    services: [{name: svc}]\n    files:", 1),
			w, "components[0].neverOverwrite", "own component",
		},
		"preserved service": {
			base + strings.Replace(appComponent, "    files:", "    preserve: true\n    services: [{name: svc}]\n    files:", 1),
			e, "components[0].preserve", "own component",
		},
		// Upgrade action properties (ICE61, section 8.10).
		"private upgrade property": {
			strings.Replace(base, "upgrade: {}", "upgrade: {property: oldFound}", 1) + appComponent,
			e, "upgrade.property", "upper-case",
		},
		"upgrade property in properties": {
			base + appComponent + "properties: {OLDERVERSIONBEINGUPGRADED: x}\n",
			e, "upgrade.property", "ICE61",
		},
		"removeOld window": {
			strings.Replace(base, "upgrade: {}", "upgrade: {removeOld: afterStartServices}", 1) + appComponent,
			e, "upgrade.removeOld", "afterInstallValidate",
		},
		// Components.
		"component without feature": {
			base + appComponent + "  - id: Other\n    directory: INSTALLDIR\n    createFolder: true\n",
			e, "components[1]", "ICE21",
		},
		"guid none not permanent": {
			base + strings.Replace(appComponent, "    files:", "    guid: none\n    files:", 1),
			e, "components[0].guid", "never removed",
		},
		"32-bit folder in x64": {
			strings.Replace(base, "ProgramFiles64Folder", "ProgramFilesFolder", 1) + appComponent,
			w, "components[0].directory", "ICE80",
		},
		"duplicate file key": {
			base + appComponent + "  - id: Two\n    directory: INSTALLDIR\n    files: [{source: other/app.exe, name: x.exe, id: app.exe}]\n",
			e, "components[1].files[0].id", "also used",
		},
		"per-user service": {
			pkg(`{platform: x64, scope: perUser}`) + strings.Replace(appComponent, "    files:", "    services: [{name: svc}]\n    files:", 1),
			e, "components[0].services[0]", "ICE105",
		},
		"dword value": {
			base + strings.Replace(appComponent, "    files:", "    registry: [{root: HKLM, key: Software\\X, name: N, type: dword, value: abc}]\n    files:", 1),
			e, "components[0].registry[0].value", "32-bit integer",
		},
		"multi string marker": {
			base + strings.Replace(appComponent, "    files:", "    registry: [{root: HKLM, key: Software\\X, name: N, value: 'a[~]b'}]\n    files:", 1),
			e, "components[0].registry[0].value", "multiString",
		},
		// Properties.
		"builder property": {
			base + appComponent + "properties: {ALLUSERS: \"1\"}\n",
			e, "properties.ALLUSERS", "set by the builder",
		},
		"secure private": {
			base + appComponent + "properties: {lower: {value: x, secure: true}}\n",
			e, "properties.lower.secure", "public",
		},
		// UI.
		"dialog without wizard": {
			base + appComponent + "ui: {dialogs: [{id: D, title: T, after: X, fields: [{property: P}]}]}\n",
			e, "ui.dialogs", "need a wizard",
		},
		"license without wizard": {
			base + appComponent + "ui: {license: LICENSE.rtf}\n",
			e, "ui.license", "needs a wizard",
		},
		"theme without wizard": {
			base + appComponent + "ui: {theme: {banner: banner.bmp}}\n",
			e, "ui.theme", "needs a wizard",
		},
		"bad theme colour": {
			base + appComponent + "ui: {wizard: minimal, theme: {colors: {accent: \"#12345\"}}}\n",
			e, "ui.theme.colors.accent", "#RRGGBB",
		},
	} {
		ds := validate(t, tc.doc)
		if !find(ds, tc.sev, tc.path, tc.msg) {
			t.Errorf("%s: want %s at %s containing %q, got:\n%v", name, tc.sev, tc.path, tc.msg, ds)
		}
	}
}

// TestVersionedFiles checks the "more than one versioned file" warning: auto
// counts as versioned only on a PE file, so data files never trigger it.
func TestVersionedFiles(t *testing.T) {
	t.Parallel()

	const warning = "more than one versioned file"

	for name, tc := range map[string]struct {
		files string
		want  bool
	}{
		"two data files":        {"[{source: a.txt}, {source: b.txt}]", false},
		"exe and data file":     {"[{source: app.exe}, {source: a.txt}]", false},
		"two executables":       {"[{source: app.exe}, {source: helper.dll}]", true},
		"explicit versions":     {"[{source: a.txt, version: 1.0.0.0}, {source: b.txt, version: 1.0.0.0}]", true},
		"explicit none on exes": {"[{source: app.exe}, {source: helper.dll, version: none, allowUnversionedExecutable: true}]", false},
	} {
		doc := base + strings.Replace(appComponent, "    files:\n      - source: app.exe\n", "    files: "+tc.files+"\n", 1)

		ds := validate(t, doc)
		if got := find(ds, config.SeverityWarning, "components[0].files", warning); got != tc.want {
			t.Errorf("%s: warning present = %v, want %v, got:\n%v", name, got, tc.want, ds)
		}
	}
}

func TestHelpers(t *testing.T) {
	t.Parallel()

	if g, err := config.NormalizeGUID("3f2504e0-4f89-41d3-9a0c-0305e82c3301"); err != nil || g != "{3F2504E0-4F89-41D3-9A0C-0305E82C3301}" {
		t.Errorf("NormalizeGUID = %q, %v", g, err)
	}

	for _, bad := range []string{"{3f2504e0-4f89-41d3-9a0c-0305e82c3301", "3f2504e04f8941d39a0c0305e82c3301", "*"} {
		if _, err := config.NormalizeGUID(bad); err == nil {
			t.Errorf("NormalizeGUID(%q) accepted", bad)
		}
	}

	for _, bad := range []string{"256.0.0", "1.256.0", "1.0.65536", "1", "1.0.0.0.0", "1..0", "1.-1.0", "a.b.c"} {
		if _, err := config.ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) accepted", bad)
		}
	}

	if pc, err := config.PageCount("4.5"); err != nil || pc != 405 {
		t.Errorf("PageCount(4.5) = %d, %v", pc, err)
	}

	for db, want := range map[int]int{1252: 1252, 932: 932, 65001: 1252, 0: 1252} {
		if got := config.SummaryCodepage(db); got != want {
			t.Errorf("SummaryCodepage(%d) = %d, want %d", db, got, want)
		}
	}
}

// TestRecoveryDelayOnNone covers a config built in Go, where a delay can be
// set on a step that is not a restart.
func TestRecoveryDelayOnNone(t *testing.T) {
	t.Parallel()

	doc := base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {actions: [none]}}]\n    files:", 1)

	cfg, err := config.Parse([]byte(doc), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	delay := config.Duration(time.Minute)
	cfg.Components[0].Services[0].Recovery.Actions[0].Restart = &delay

	field := "components[0].services[0].recovery.actions[0].restart"
	if ds := config.Validate(cfg); !find(ds, config.SeverityError, field, "only allowed on a restart") {
		t.Errorf("diagnostics %v, want an error at %s", ds, field)
	}
}

// The default cabinet name comes from the product name, which may not fit a
// stream name (FuzzBuild finding).
func TestDefaultCabinet(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{
		"App":    "App.cab",
		"My App": "My App.cab",
		"Grüße":  config.DefaultCabinet,
		"Привет": config.DefaultCabinet,
		"A#B":    config.DefaultCabinet,
		// 27 spaces take one stream name unit each.
		strings.Repeat("A ", 27): config.DefaultCabinet,
	} {
		doc := strings.Replace(base, "name: App\n  manufacturer", "name: '"+name+"'\n  manufacturer", 1)

		cfg, err := config.Parse([]byte(doc+appComponent), config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		if cfg.Package.Cabinet != want {
			t.Errorf("product %q: cabinet %q, want %q", name, cfg.Package.Cabinet, want)
		}

		if ds := config.Validate(cfg); find(ds, config.SeverityError, "package.cabinet", "") {
			t.Errorf("product %q: the default cabinet is invalid: %v", name, ds)
		}
	}
}

func TestCabinetName(t *testing.T) {
	t.Parallel()

	for cabinet, valid := range map[string]bool{
		"product.cab":                    true,
		strings.Repeat("a", 58) + ".cab": true,
		strings.Repeat("a", 59) + ".cab": false,
		// Every character outside 0-9A-Za-z._ takes a unit of its own.
		strings.Repeat("-", 31): true,
		strings.Repeat("-", 32): false,
		"daten-ü.cab":           false,
		"a/b.cab":               false,
		// Not a forbidden file-name character, but CFB rejects it.
		"a!b.cab": false,
		// A control character could name a stream go-msi writes itself.
		"\x05SummaryInformation": false,
		"a\tb.cab":               false,
		// Outside the BMP: two units each.
		strings.Repeat("\U0001F600", 15): true,
		strings.Repeat("\U0001F600", 16): false,
	} {
		ds := validate(t, pkg(`{platform: x64, cabinet: `+strconv.Quote(cabinet)+`}`)+appComponent)
		if got := !find(ds, config.SeverityError, "package.cabinet", ""); got != valid {
			t.Errorf("cabinet %q: valid = %v, want %v (%v)", cabinet, got, valid, ds)
		}
	}
}

func TestPreserve(t *testing.T) {
	t.Parallel()

	doc := base + strings.Replace(appComponent, "    files:", "    preserve: true\n    files:", 1)

	cfg, err := config.Parse([]byte(doc), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if ds := config.Validate(cfg); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}

	if c := cfg.Components[0]; c.Permanent || !c.NeverOverwrite {
		t.Errorf("preserve: permanent %v, neverOverwrite %v, want only neverOverwrite", c.Permanent, c.NeverOverwrite)
	}

	if got := cfg.Upgrade.RemoveOld; got != config.PreserveRemoveOld {
		t.Errorf("preserve: removeOld %q, want %q", got, config.PreserveRemoveOld)
	}

	early := strings.Replace(doc, "upgrade: {}", "upgrade: {removeOld: afterInstallInitialize}", 1)
	if ds := validate(t, early); !find(ds, config.SeverityError, "components[0].preserve", "RUN-24") {
		t.Errorf("preserve with an early removeOld: diagnostics %v, want an error", ds)
	}

	// RUN-24 neverperm-retrofit-early: permanent does not save an edit made
	// in a version without the flags.
	perm := strings.Replace(early, "    preserve: true\n", "    preserve: true\n    permanent: true\n", 1)
	if ds := validate(t, perm); !find(ds, config.SeverityError, "components[0].preserve", "RUN-24") {
		t.Errorf("preserve and permanent with an early removeOld: diagnostics %v, want an error", ds)
	}

	finalize := strings.Replace(doc, "upgrade: {}", "upgrade: {removeOld: afterInstallFinalize}", 1)
	if ds := validate(t, finalize); !find(ds, config.SeverityWarning, "components[0].preserve", "not") {
		t.Errorf("preserve with removeOld afterInstallFinalize: diagnostics %v, want a warning", ds)
	}

	off := strings.Replace(early, "upgrade: {removeOld: afterInstallInitialize}", "upgrade: false", 1)
	if ds := validate(t, off); len(ds) != 0 {
		t.Errorf("preserve without upgrades: unexpected diagnostics: %v", ds)
	}

	late := strings.Replace(base, "upgrade: {}", "upgrade: {removeOld: afterInstallExecute}", 1) +
		strings.Replace(appComponent, "    files:", "    neverOverwrite: true\n    files:", 1)
	if ds := validate(t, late); len(ds) != 0 {
		t.Errorf("neverOverwrite with a late removal: unexpected diagnostics: %v", ds)
	}
}

// TestPreserveInFeature checks that a preserved component defined inside a
// feature also moves the removal of the old product late.
func TestPreserveInFeature(t *testing.T) {
	t.Parallel()

	cfg := parseValid(t, short+`features:
  - id: Main
    components:
      - files: [{ source: app.exe }]
      - id: Config
        preserve: true
        files: [{ source: config.yaml }]
`)

	if got := cfg.Upgrade.RemoveOld; got != config.PreserveRemoveOld {
		t.Errorf("removeOld %q, want %q", got, config.PreserveRemoveOld)
	}

	for _, c := range cfg.Components {
		if c.ID == "Config" && !c.NeverOverwrite {
			t.Error("preserved component without neverOverwrite")
		}
	}
}
