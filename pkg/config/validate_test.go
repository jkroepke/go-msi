package config_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// base is a valid package; each case appends to or replaces parts of it.
const base = `schema: msi-builder/v1
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

// TestValidateAccepted checks values that must not produce a diagnostic.
func TestValidateAccepted(t *testing.T) {
	t.Parallel()

	firewall := func(addresses string) string {
		return base + appComponent + "firewall: [{name: a, component: App, program: x, remoteAddresses: '" + addresses + "'}]\n"
	}
	environment := func(entries string) string {
		return base + strings.Replace(appComponent, "    files:", "    environment: ["+entries+"]\n    files:", 1)
	}

	for name, doc := range map[string]string{
		"remote any":          firewall("ANY"),
		"remote star":         firewall("*"),
		"remote keyword list": firewall("localsubnet,10.0.0.1"),
		"remote keywords":     firewall("LocalSubnet,dns"),
		"remote property":     firewall("[REMOTE_ADDR]"),
		"environment scopes":  environment("{name: FOO, value: a}, {name: FOO, value: b, scope: user}"),
		"environment actions": environment("{name: FOO, value: a}, {name: FOO, value: b, action: append}"),
		"property search": base + appComponent +
			"properties: {P: {value: x, search: {registry: {root: HKLM, key: Software\\X, name: V, bitness: 32, type: directory}}}}\n",
		"remove registry": base + strings.Replace(appComponent, "    files:",
			"    removeRegistry: [{root: HKCU, key: Software\\Classes\\old}, {root: HKMU, key: Software\\X, name: Legacy}]\n    files:", 1),
	} {
		if ds := validate(t, doc); len(ds) != 0 {
			t.Errorf("%s: unexpected diagnostics: %v", name, ds)
		}
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	const (
		sevError   = config.SeverityError
		sevWarning = config.SeverityWarning
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
			sevError, "package.installerVersion", "1613",
		},
		"x64 below 2.0": {
			pkg(`{platform: x64, installerVersion: "1.5"}`) + appComponent,
			sevError, "package.installerVersion", "at least 2.0",
		},
		"arm64 below 5.0": {
			pkg(`{platform: arm64, installerVersion: "4.5"}`) + appComponent,
			sevError, "package.installerVersion", "at least 5.0",
		},
		// FILE-01: an unversioned executable is deleted by an upgrade.
		"unversioned exe": {
			base + appComponent + "      - source: helper.dll\n        version: none\n",
			sevError, "components[0].files[1].version", "FILE-01",
		},
		"unversioned exe acknowledged": {
			base + appComponent + "      - source: helper.dll\n        version: none\n        allowUnversionedExecutable: true\n",
			sevWarning, "components[0].files[1].version", "FILE-01",
		},
		"empty version is not none": {
			base + appComponent + "      - source: data.txt\n        version: \"1.x\"\n",
			sevError, "components[0].files[1].version", "not auto, none",
		},
		// UPG-01 and UPG-03.
		"four-field version": {
			strings.Replace(base, "version: 1.0.0", "version: 1.0.0.5", 1) + appComponent,
			sevWarning, "product.version", "UPG-01",
		},
		"nil upgrade code": {
			strings.Replace(base, "3f2504e0-4f89-41d3-9a0c-0305e82c3301", `"{00000000-0000-0000-0000-000000000000}"`, 1) + appComponent,
			sevError, "product.upgradeCode", "nil GUID",
		},
		"nil related upgrade code": {
			strings.Replace(base, "upgrade: {}", "upgrade: {related: [{upgradeCode: 00000000-0000-0000-0000-000000000000, maxVersion: 1.0.0}]}", 1) + appComponent,
			sevError, "upgrade.related[0].upgradeCode", "nil GUID",
		},
		"pinned product code": {
			strings.Replace(base, "  upgradeCode:", "  productCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3302\n  upgradeCode:", 1) + appComponent,
			sevWarning, "product.productCode", "1638",
		},
		// CA-02: exec actions must say how an exit code is handled.
		"exec without return": {
			base + appComponent + `customActions:
  - id: Run
    exec: {command: cmd.exe}
    execute: deferred
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[0].return", "CA-02",
		},
		// CA-03: run actions carry the same return rule and a few of their own.
		"run without return": {
			base + appComponent + `customActions:
  - id: Run
    run: {command: cmd.exe}
    execute: deferred
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[0].return", "CA-02",
		},
		"run async": {
			base + appComponent + `customActions:
  - id: Run
    run: {command: cmd.exe}
    execute: deferred
    return: asyncNoWait
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[0].return", "asyncNoWait",
		},
		"run unknown directory": {
			base + appComponent + `customActions:
  - id: Run
    run: {command: cmd.exe, directory: NOWHERE}
    execute: deferred
    return: check
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[0].run.directory", "not defined",
		},
		"run id too long for .Data": {
			base + appComponent + `customActions:
  - id: Run67890123456789012345678901234567890123456789012345678901234567890
    run: {command: cmd.exe}
    execute: deferred
    return: check
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[0].id", ".Data",
		},
		"run data id clash": {
			base + appComponent + `customActions:
  - id: Run
    run: {command: cmd.exe}
    execute: deferred
    return: check
    schedule: [{sequence: execute, after: InstallFiles}]
  - id: Run.Data
    setProperty: {name: A, value: b}
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[1].id", "data action",
		},
		"deferred set property": {
			base + appComponent + `customActions:
  - id: Set
    setProperty: {name: A, value: b}
    execute: deferred
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[0].execute", "cannot be deferred",
		},
		"no impersonation immediate": {
			base + appComponent + `customActions:
  - id: Set
    setProperty: {name: A, value: b}
    impersonate: false
    schedule: [{sequence: execute, after: InstallFiles}]
`,
			sevError, "customActions[0].impersonate", "deferred",
		},
		"standard action name": {
			base + appComponent + `customActions:
  - id: InstallFiles
    error: no
    schedule: [{sequence: execute, after: CostFinalize}]
`,
			sevError, "customActions[0].id", "ICE93",
		},
		"unknown anchor": {
			base + appComponent + `customActions:
  - id: Fail
    error: no
    schedule: [{sequence: execute, after: Nowhere}]
`,
			sevError, "customActions[0].schedule[0]", "Nowhere",
		},
		// Finding 1 (MSIDB-07) and MSIDB-02.
		"32 columns": {
			base + appComponent + "tables:\n  - name: Wide\n    columns:\n" +
				strings.Repeat("      - {name: C, type: I2, key: true}\n", 32),
			sevError, "tables[0].columns", "MSIDB-07",
		},
		"i2 minimum": {
			base + appComponent + `tables:
  - name: T
    columns: [{name: K, type: s72, key: true}, {name: V, type: I2}]
    rows: [[a, -32768]]
`,
			sevError, "tables[0].rows[0][1]", "outside",
		},
		"standard table": {
			base + appComponent + "tables:\n  - name: Property\n    columns: [{name: K, type: s72, key: true}]\n",
			sevError, "tables[0].name", "written by the builder",
		},
		// MSIDB-11.
		"binary key too long": {
			base + appComponent + "binaries:\n  " + strings.Repeat("b", 56) + ": x.dll\n",
			sevError, "binaries." + strings.Repeat("b", 56), "MSIDB-11",
		},
		// ENVVAR-01: removal is never implied by scope.
		"environment prefix": {
			base + strings.Replace(appComponent, "    files:", "    environment: [{name: '*PATH', value: x}]\n    files:", 1),
			sevError, "components[0].environment[0].name", "plain name",
		},
		"environment twice": {
			base + strings.Replace(appComponent, "    files:", "    environment: [{name: FOO, value: a}, {name: FOO, value: b}]\n    files:", 1),
			sevError, "components[0].environment[1]", "FOO with action set and scope machine is set twice",
		},
		"firewall description quote": {
			base + appComponent + "firewall: [{name: a, component: App, program: x, description: 'say \"hi\"'}]\n",
			sevError, "firewall[0].description", "double quote",
		},
		"firewall any with addresses": {
			base + appComponent + "firewall: [{name: a, component: App, program: x, remoteAddresses: 'ANY,10.0.0.1'}]\n",
			sevError, "firewall[0].remoteAddresses", "ANY cannot be combined",
		},
		"firewall spaced addresses": {
			base + appComponent + "firewall: [{name: a, component: App, program: x, remoteAddresses: '10.0.0.1, 10.0.0.2'}]\n",
			sevError, "firewall[0].remoteAddresses", "must not contain spaces",
		},
		"firewall quoted address": {
			base + appComponent + "firewall: [{name: a, component: App, program: x, remoteAddresses: '\"10.0.0.1\"'}]\n",
			sevError, "firewall[0].remoteAddresses", "double quotes",
		},
		"firewall empty address": {
			base + appComponent + "firewall: [{name: a, component: App, program: x, remoteAddresses: 'any,'}]\n",
			sevError, "firewall[0].remoteAddresses", "empty entry",
		},
		"firewall star with addresses": {
			base + appComponent + "firewall: [{name: a, component: App, program: x, remoteAddresses: '10.0.0.1,*'}]\n",
			sevError, "firewall[0].remoteAddresses", "* cannot be combined",
		},
		"environment twice in another case": {
			base + strings.Replace(appComponent, "    files:", "    environment: [{name: FOO, value: a}, {name: foo, value: b}]\n    files:", 1),
			sevError, "components[0].environment[1]", "foo with action set and scope machine is set twice",
		},
		"PATH set": {
			base + strings.Replace(appComponent, "    files:", "    environment: [{name: PATH, value: x}]\n    files:", 1),
			sevWarning, "components[0].environment[0].action", "append",
		},
		"lzx window too small": {
			pkg(`{platform: x64, compression: "lzx:14"}`) + appComponent,
			sevError, "package.compression", `"lzx:14" is not one of`,
		},
		// SUM-04.
		"utf-16 codepage": {
			pkg(`{platform: x64, codepage: 1200}`) + appComponent,
			sevError, "package.codepage", "1200",
		},
		// SVC-01.
		"service keeps registration": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, control: {deleteOnUninstall: false}}]\n    files:", 1),
			sevWarning, "components[0].services[0].control.deleteOnUninstall", "SVC-01",
		},
		"service on dll": {
			base + strings.Replace(strings.Replace(appComponent, "app.exe", "app.dll", 1), "    files:", "    services: [{name: svc}]\n    files:", 1),
			sevError, "components[0].services[0]", "not an .exe",
		},
		"literal password": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, password: hunter2}]\n    files:", 1),
			sevError, "components[0].services[0].password", "hidden property",
		},
		"recovery reset twice": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {resetAfterDays: 1, resetAfter: 24h, actions: [restart]}}]\n    files:", 1),
			sevError, "components[0].services[0].recovery.resetAfter", "not both",
		},
		"recovery reset fraction": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {resetAfter: 1500ms, actions: [restart]}}]\n    files:", 1),
			sevError, "components[0].services[0].recovery.resetAfter", "whole number of seconds",
		},
		"recovery step delay negative": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {actions: [{restart: -1s}]}}]\n    files:", 1),
			sevError, "components[0].services[0].recovery.actions[0].restart", "negative",
		},
		"recovery step delay too long": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {actions: [{restart: 600h}]}}]\n    files:", 1),
			sevError, "components[0].services[0].recovery.actions[0].restart", "too long",
		},
		"recovery reset days too long": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {resetAfterDays: 30000, actions: [restart]}}]\n    files:", 1),
			sevError, "components[0].services[0].recovery.resetAfterDays", "too long",
		},
		"recovery four steps": {
			base + strings.Replace(appComponent, "    files:", "    services: [{name: svc, recovery: {actions: [restart, restart, restart, restart]}}]\n    files:", 1),
			sevError, "components[0].services[0].recovery.actions", "one to three",
		},
		"dialog field type": {
			base + appComponent + "ui:\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: radio}]}\n",
			sevError, "ui.dialogs[0].fields[0].type", "",
		},
		"dialog field value on text": {
			base + appComponent + "ui:\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, value: x}]}\n",
			sevError, "ui.dialogs[0].fields[0].value", "only a checkbox",
		},
		"choice one option": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a]}]}\n",
			sevError, "ui.dialogs[0].fields[0].options", "at least two",
		},
		"choice duplicate": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, a]}]}\n",
			sevError, "ui.dialogs[0].fields[0].options[1].value", "offered twice",
		},
		"choice brackets": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, '[b]']}]}\n",
			sevError, "ui.dialogs[0].fields[0].options[1].value", "formatted",
		},
		"choice long value": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx]}]}\n",
			sevError, "ui.dialogs[0].fields[0].options[1].value", "at most 64",
		},
		"choice long label": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, {value: b, label: yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy}]}]}\n",
			sevError, "ui.dialogs[0].fields[0].options[1].label", "at most 64",
		},
		"choice empty value": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, {label: b}]}]}\n",
			sevError, "ui.dialogs[0].fields[0].options[1].value", "required",
		},
		"choice default not an option": {
			base + appComponent + "properties: {OPT: c}\nui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, b]}]}\n",
			sevError, "ui.dialogs[0].fields[0].property", "none of the options",
		},
		"choice on a searched property": {
			base + appComponent + "properties: {OPT: {search: {registry: {root: HKLM, key: X}}}}\nui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, b]}]}\n",
			sevError, "ui.dialogs[0].fields[0].property", "ICE34",
		},
		"options on text": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, options: [a, b]}]}\n",
			sevError, "ui.dialogs[0].fields[0].options", "only a choice",
		},
		"choice shares a property": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: choice, options: [a, b]}, {property: OPT}]}\n",
			sevError, "ui.dialogs[0].fields[0].property", "another field",
		},
		"choice on RMOPTION": {
			base + appComponent + "ui:\n  wizard: minimal\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: RMOPTION, type: choice, options: [a, b]}]}\n",
			sevError, "ui.dialogs[0].fields[0].property", "wizard",
		},
		"checkbox with a default": {
			base + appComponent + "properties: {OPT: '0'}\nui:\n  dialogs:\n    - {id: ExtraDlg, title: x, after: WelcomeDlg, fields: [{property: OPT, type: checkbox}]}\n",
			sevWarning, "ui.dialogs[0].fields[0].property", "starts checked",
		},
		"schedule when": {
			base + appComponent + "customActions:\n  - id: Set\n    setProperty: {name: A, value: b}\n    schedule: [{sequence: execute, after: CostFinalize, when: always}]\n",
			sevError, "customActions[0].schedule[0].when", "",
		},
		"schedule when upgrade without upgrades": {
			strings.Replace(base, "upgrade: {}", "upgrade: false", 1) + appComponent +
				"customActions:\n  - id: Set\n    setProperty: {name: A, value: b}\n    schedule: [{sequence: execute, after: CostFinalize, when: upgrade}]\n",
			sevError, "customActions[0].schedule[0].when", "upgrade section",
		},
		"schedule when uninstall in ui": {
			base + appComponent + "customActions:\n  - id: Set\n    setProperty: {name: A, value: b}\n    schedule: [{sequence: ui, after: CostFinalize, when: uninstall}]\n",
			sevError, "customActions[0].schedule[0].when", "InstallValidate",
		},
		"schedule when in admin": {
			base + appComponent + "customActions:\n  - id: Set\n    setProperty: {name: A, value: b}\n    schedule: [{sequence: adminExecute, after: CostFinalize, when: install}]\n",
			sevError, "customActions[0].schedule[0].when", "administrative",
		},
		// SEARCH-03.
		"remember with a component of the same id": {
			strings.Replace(base, "        configurable: true\n", "        configurable: true\ninstallDir: {id: INSTALLDIR, remember: true}\n", 1) +
				strings.Replace(appComponent, "id: App", "id: RememberInstallDir", 1),
			sevError, "components[0].id", "installDir.remember adds",
		},
		"remember a private install dir": {
			strings.Replace(base, "      - id: INSTALLDIR\n        name: App\n        configurable: true\n", "      - id: AppDir\n        name: App\n", 1) +
				"installDir: {id: AppDir, remember: true}\n" + strings.Replace(appComponent, "INSTALLDIR", "AppDir", 1),
			sevError, "installDir.remember", "configurable directory with an upper-case id",
		},
		"remember with an action of the same id": {
			base + "installDir: {id: INSTALLDIR, remember: true}\n" + appComponent +
				"customActions:\n  - id: RestoreInstallDir\n    setProperty: {name: A, value: b}\n    schedule: [{sequence: execute, after: CostFinalize}]\n",
			sevError, "customActions[0].id", "installDir.remember adds",
		},
		// FILE-03.
		"purge configurable folder": {
			base + strings.Replace(appComponent, "    files:", "    purge: true\n    files:", 1),
			sevError, "components[0].purge", "configurable",
		},
		"purge standard folder": {
			base + strings.Replace(appComponent, "directory: INSTALLDIR", "directory: ProgramFiles64Folder\n    purge: true", 1),
			sevError, "components[0].purge", "standard folder",
		},
		"purge below configurable folder": {
			strings.Replace(base, "        configurable: true\n", "        configurable: true\n        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1),
			sevError, "components[0].purge", "INSTALLDIR can be pointed",
		},
		"purge nested standard folder": {
			strings.Replace(base, "directories:\n", "directories:\n  - id: TARGETDIR\n    children: [{id: CommonAppDataFolder, name: ProgramData}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: CommonAppDataFolder\n    createFolder: true\n    purge: true", 1),
			sevError, "components[0].purge", "standard folder",
		},
		"purge below nested standard folder": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: CommonAppDataFolder, name: x, children: [{id: DataDir, name: data}]}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1),
			sevError, "components[0].purge", "CommonAppDataFolder can be pointed",
		},
		"purge percent in name": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: '%TEMP%'}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1),
			sevError, "components[0].purge", "has a %",
		},
		"purge tilde in name": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: 'PROGRA~1'}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1),
			sevError, "components[0].purge", "8.3 alias",
		},
		"short|long source base name": {
			base + strings.Replace(appComponent, "source: app.exe", "source: 'APP.EXE|app.exe'", 1),
			sevError, "components[0].files[0].source", "short names are generated",
		},
		"purge below a standard folder set by setDirectory": {
			base + "components:\n  - id: Data\n    directory: CommonAppDataFolder/Example\n    createFolder: true\n    purge: true\n" +
				"customActions:\n  - id: SetIt\n    setDirectory: {directory: CommonAppDataFolder, value: '[DATAROOT]'}\n" +
				"    schedule: [{sequence: execute, after: CostFinalize}]\n",
			sevError, "components[0].purge", "CommonAppDataFolder can be pointed",
		},
		"short-name package with a long file name": {
			pkg("{platform: x64, longFileNames: false}") + strings.Replace(appComponent, "source: app.exe", "{source: app.exe, name: Application.exe}", 1),
			sevError, "components[0].files[0].name", "valid 8.3 name",
		},
		"short-name package with a long source name": {
			pkg("{platform: x64, longFileNames: false}") + strings.Replace(appComponent, "source: app.exe", "source: Application.exe", 1),
			sevError, "components[0].files[0].source", "valid 8.3 name",
		},
		"short|long file name": {
			base + strings.Replace(appComponent, "source: app.exe", "{source: app.exe, name: 'APP.EXE|Application.exe'}", 1),
			sevError, "components[0].files[0].name", "short names are generated",
		},
		"purge above a permanent component": {
			base + appComponent + "  - id: Data\n    directory: CommonAppDataFolder/Example/Hello\n    createFolder: true\n    purge: true\n" +
				"  - id: Keep\n    directory: CommonAppDataFolder/Example/Hello/keep\n    createFolder: true\n    permanent: true\n",
			sevError, "components[1].purge", "permanent component Keep",
		},
		"purge permanent": {
			base + strings.Replace(appComponent, "    files:", "    permanent: true\n    purge: true\n    files:", 1),
			sevError, "components[0].purge", "permanent",
		},
		"purge below setDirectory": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1) +
				"customActions:\n  - id: MoveIt\n    setDirectory: {directory: INSTALLDIR, value: '[WindowsFolder]'}\n",
			sevError, "components[0].purge", "INSTALLDIR can be pointed",
		},
		"purge public folder id": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DATADIR, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DATADIR\n    purge: true", 1),
			sevError, "components[0].purge", "DATADIR can be pointed",
		},
		"purge below TARGETDIR": {
			strings.Replace(base, "directories:\n", "directories:\n  - id: TARGETDIR\n    children: [{id: DataDir, name: Data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    createFolder: true\n    purge: true", 1),
			sevError, "components[0].purge", "TARGETDIR",
		},
		"purge folder set by a property": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1) +
				"properties:\n  DataDir: 'C:\\'\n",
			sevError, "components[0].purge", "DataDir can be pointed",
		},
		"purge folder set by setProperty": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DataDir, name: data}]\n", 1) +
				strings.Replace(appComponent, "directory: INSTALLDIR", "directory: DataDir\n    purge: true", 1) +
				"customActions:\n  - id: SetIt\n    setProperty: {name: DataDir, value: '[WindowsFolder]'}\n",
			sevError, "components[0].purge", "DataDir can be pointed",
		},
		"purge condition": {
			strings.Replace(base, "        configurable: true\n", "        children: [{id: DATADIR, name: data}]\n", 1) + strings.Replace(appComponent, "    files:", "    purge: {condition: 'DELETE_DATA='}\n    files:", 1),
			sevError, "components[0].purge.condition", "",
		},
		// UPG-05.
		"neverOverwrite with early removal": {
			base + strings.Replace(appComponent, "    files:", "    neverOverwrite: true\n    files:", 1),
			sevWarning, "components[0].neverOverwrite", "UPG-05",
		},
		"neverOverwrite service with early removal": {
			base + strings.Replace(appComponent, "    files:", "    neverOverwrite: true\n    services: [{name: svc}]\n    files:", 1),
			sevWarning, "components[0].neverOverwrite", "own component",
		},
		"preserved service": {
			base + strings.Replace(appComponent, "    files:", "    preserve: true\n    services: [{name: svc}]\n    files:", 1),
			sevError, "components[0].preserve", "own component",
		},
		// Upgrade action properties (ICE61, section 8.10).
		"private upgrade property": {
			strings.Replace(base, "upgrade: {}", "upgrade: {property: oldFound}", 1) + appComponent,
			sevError, "upgrade.property", "upper-case",
		},
		"upgrade property in properties": {
			base + appComponent + "properties: {OLDERVERSIONBEINGUPGRADED: x}\n",
			sevError, "upgrade.property", "ICE61",
		},
		"removeOld window": {
			strings.Replace(base, "upgrade: {}", "upgrade: {removeOld: afterStartServices}", 1) + appComponent,
			sevError, "upgrade.removeOld", "afterInstallValidate",
		},
		// Components.
		"component without feature": {
			base + appComponent + "  - id: Other\n    directory: INSTALLDIR\n    createFolder: true\n",
			sevError, "components[1]", "ICE21",
		},
		"guid none not permanent": {
			base + strings.Replace(appComponent, "    files:", "    guid: none\n    files:", 1),
			sevError, "components[0].guid", "never removed",
		},
		"32-bit folder in x64": {
			strings.Replace(base, "ProgramFiles64Folder", "ProgramFilesFolder", 1) + appComponent,
			sevWarning, "components[0].directory", "ICE80",
		},
		"duplicate generated file key": {
			base + appComponent + "  - id: Two\n    directory: INSTALLDIR\n    files: [{source: other/a-b.exe}, {source: other/a_b.exe}]\n",
			sevError, "components[1].files[1].id", "also used",
		},
		"duplicate file key": {
			base + appComponent + "  - id: Two\n    directory: INSTALLDIR\n    files: [{source: other/app.exe, name: x.exe, id: app.exe}]\n",
			sevError, "components[1].files[0].id", "also used",
		},
		"per-user service": {
			pkg(`{platform: x64, scope: perUser}`) + strings.Replace(appComponent, "    files:", "    services: [{name: svc}]\n    files:", 1),
			sevError, "components[0].services[0]", "ICE105",
		},
		"dword value": {
			base + strings.Replace(appComponent, "    files:", "    registry: [{root: HKLM, key: Software\\X, name: N, type: dword, value: abc}]\n    files:", 1),
			sevError, "components[0].registry[0].value", "32-bit integer",
		},
		"multi string marker": {
			base + strings.Replace(appComponent, "    files:", "    registry: [{root: HKLM, key: Software\\X, name: N, value: 'a[~]b'}]\n    files:", 1),
			sevError, "components[0].registry[0].value", "multiString",
		},
		"remove registry name dash": {
			base + strings.Replace(appComponent, "    files:", "    removeRegistry: [{root: HKLM, key: Software\\X, name: '-'}]\n    files:", 1),
			sevError, "components[0].removeRegistry[0].name", "leave name out",
		},
		"remove registry twice": {
			base + strings.Replace(appComponent, "    files:", "    removeRegistry: [{root: HKLM, key: Software\\X}, {root: HKLM, key: software\\x}]\n    files:", 1),
			sevError, "components[0].removeRegistry[1]", "already removed",
		},
		"remove registry twice with ids": {
			base + strings.Replace(appComponent, "    files:", "    removeRegistry: [{id: A, root: HKLM, key: Software\\X}, {root: HKLM, key: Software\\X}]\n    files:", 1),
			sevError, "components[0].removeRegistry[1]", "already removed",
		},
		"remove registry twice id last": {
			base + strings.Replace(appComponent, "    files:", "    removeRegistry: [{root: HKLM, key: Software\\X}, {id: B, root: HKLM, key: Software\\X}]\n    files:", 1),
			sevError, "components[0].removeRegistry[1]", "already removed",
		},
		"remove registry bad id": {
			base + strings.Replace(appComponent, "    files:", "    removeRegistry: [{id: 'a b', root: HKLM, key: Software\\X}]\n    files:", 1),
			sevError, "components[0].removeRegistry[0].id", "not an identifier",
		},
		"remove registry id twice": {
			base + strings.Replace(appComponent, "    files:", "    removeRegistry: [{id: A, root: HKLM, key: Software\\X}]\n    files:", 1) +
				"  - id: Other\n    directory: INSTALLDIR\n    removeRegistry: [{id: A, root: HKLM, key: Software\\Y}]\n",
			sevError, "components[1].removeRegistry[0].id", "defined twice",
		},
		"remove registry per user": {
			pkg(`{platform: x64, scope: perUser}`) + strings.Replace(appComponent, "    files:", "    removeRegistry: [{root: HKLM, key: Software\\X}]\n    files:", 1),
			sevError, "components[0].removeRegistry[0].root", "ICE105",
		},
		"remove registry key slash": {
			base + strings.Replace(appComponent, "    files:", "    removeRegistry: [{root: HKLM, key: Software\\X\\}]\n    files:", 1),
			sevError, "components[0].removeRegistry[0].key", "backslash",
		},
		"search private property": {
			base + appComponent + "properties: {lower: {search: {registry: {root: HKLM, key: Software\\X}}}}\n",
			sevError, "properties.lower.search", "public",
		},
		"search not secure": {
			base + appComponent + "properties: {P: {secure: false, search: {registry: {root: HKLM, key: Software\\X}}}}\n",
			sevError, "properties.P.secure", "must be secure",
		},
		"search without registry": {
			base + appComponent + "properties: {P: {search: {}}}\n",
			sevError, "properties.P.search.registry", "required",
		},
		"search hkmu": {
			base + appComponent + "properties: {P: {search: {registry: {root: HKMU, key: Software\\X}}}}\n",
			sevError, "properties.P.search.registry.root", "not one of",
		},
		"search type": {
			base + appComponent + "properties: {P: {search: {registry: {root: HKLM, key: Software\\X, type: file}}}}\n",
			sevError, "properties.P.search.registry.type", "not one of",
		},
		"search 64 in x86": {
			pkg(`{platform: x86}`) + appComponent + "properties: {P: {search: {registry: {root: HKLM, key: Software\\X, bitness: 64}}}}\n",
			sevError, "properties.P.search.registry.bitness", "32-bit package",
		},
		"search found taken": {
			base + appComponent + "properties: {P: {search: {registry: {root: HKLM, key: Software\\X}}}, P_FOUND: x}\n",
			sevError, "properties.P.search", "P_FOUND",
		},
		// Properties.
		"builder property": {
			base + appComponent + "properties: {ALLUSERS: \"1\"}\n",
			sevError, "properties.ALLUSERS", "set by the builder",
		},
		"secure private": {
			base + appComponent + "properties: {lower: {value: x, secure: true}}\n",
			sevError, "properties.lower.secure", "public",
		},
		// UI.
		"dialog without wizard": {
			base + appComponent + "ui: {dialogs: [{id: D, title: T, after: X, fields: [{property: P}]}]}\n",
			sevError, "ui.dialogs", "need a wizard",
		},
		"license without wizard": {
			base + appComponent + "ui: {license: LICENSE.rtf}\n",
			sevError, "ui.license", "needs a wizard",
		},
		"theme without wizard": {
			base + appComponent + "ui: {theme: {banner: banner.bmp}}\n",
			sevError, "ui.theme", "needs a wizard",
		},
		"bad theme colour": {
			base + appComponent + "ui: {wizard: minimal, theme: {colors: {accent: \"#12345\"}}}\n",
			sevError, "ui.theme.colors.accent", "#RRGGBB",
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

// TestGeneratedFileKey checks the file key of a file whose name is no
// identifier.
func TestGeneratedFileKey(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{
		"app.exe":               "app.exe",
		"fleetspeak-client.exe": "fleetspeak_client.exe",
		"7zip.dll":              "_7zip.dll",
		"my app (x64).exe":      "my_app__x64_.exe",
		".config":               "_.config",
	} {
		if got := config.FileKey(&config.File{Source: "bin/" + name}); got != want {
			t.Errorf("FileKey(%q) = %q, want %q", name, got, want)
		}
	}

	if got := config.FileKey(&config.File{Source: "a-b.exe", ID: "Main"}); got != "Main" {
		t.Errorf("FileKey with id = %q, want Main", got)
	}

	// Long names are cut to at most 60 characters with a hash suffix,
	// whether or not they were identifiers.
	for _, name := range []string{strings.Repeat("a-", 35) + ".exe", strings.Repeat("a", 80) + ".exe"} {
		if got := config.FileKey(&config.File{Source: name}); len(got) > 60 || !config.IsIdentifier(got) {
			t.Errorf("FileKey(%q) = %q", name, got)
		}
	}

	doc := base + strings.Replace(appComponent, "app.exe", "my-app.exe", 1)
	if ds := validate(t, doc); len(ds) != 0 {
		t.Errorf("unexpected diagnostics for my-app.exe: %v", ds)
	}

	// launch.file and a companion version may name the file by its long name.
	doc = base + strings.Replace(appComponent, "      - source: app.exe\n",
		"      - source: my-app.exe\n      - { source: data.bin, version: my-app.exe }\n", 1) +
		"ui: {wizard: minimal, launch: {file: my-app.exe}}\n"
	if err := validate(t, doc).Err(); err != nil {
		t.Errorf("references by long name: %v", err)
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
		"App": "App.cab",
		// ICE03 wants an identifier.
		"My App":                "MyApp.cab",
		"Acme-Tool":             "AcmeTool.cab",
		"Notepad++":             "Notepad.cab",
		"7Zip":                  config.DefaultCabinet,
		"   ":                   config.DefaultCabinet,
		"Grüße":                 config.DefaultCabinet,
		"Привет":                config.DefaultCabinet,
		"A#B":                   "AB.cab",
		strings.Repeat("A", 59): config.DefaultCabinet,
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
		// A control character could name a stream msi-builder writes itself.
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

func TestCabinetIdentifierWarns(t *testing.T) {
	t.Parallel()

	for cabinet, warns := range map[string]bool{"my app.cab": true, "my-app.cab": true, "my_app.cab": false} {
		ds := validate(t, pkg(`{platform: x64, cabinet: "`+cabinet+`"}`)+appComponent)
		if got := find(ds, config.SeverityWarning, "package.cabinet", "ICE03"); got != warns {
			t.Errorf("cabinet %q: ICE03 warning = %v, want %v (%v)", cabinet, got, warns, ds)
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
	if ds := validate(t, early); !find(ds, config.SeverityError, "components[0].preserve", "UPG-05") {
		t.Errorf("preserve with an early removeOld: diagnostics %v, want an error", ds)
	}

	// UPG-05 neverperm-retrofit-early: permanent does not save an edit made
	// in a version without the flags.
	perm := strings.Replace(early, "    preserve: true\n", "    preserve: true\n    permanent: true\n", 1)
	if ds := validate(t, perm); !find(ds, config.SeverityError, "components[0].preserve", "UPG-05") {
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
