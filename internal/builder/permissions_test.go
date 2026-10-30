package builder_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

func TestPermissions(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})

	comp := cfg.Components[0]
	comp.Files[0].Permissions = &config.Permissions{SDDL: "D:PAI(A;;FA;;;BA)(A;;FRFX;;;BU)", Condition: "LOCKDOWN=1"}
	comp.CreateFolder = true
	comp.FolderPermissions = &config.Permissions{SDDL: "D:(A;OICI;FA;;;<BUILTIN\\Administrators>)"}
	comp.Registry = append(comp.Registry, &config.RegistryValue{
		Root: "HKLM", Key: `Software\Hello`, Name: "Setting", Type: "string", Value: config.StringList{"x"},
		Permissions: &config.Permissions{SDDL: "D:(A;;KA;;;BA)"},
	})

	res := mustBuild(t, cfg)
	locks := rows(t, res.Database, "MsiLockPermissionsEx")

	if len(locks) != 3 {
		t.Fatalf("%d rows: %v", len(locks), locks)
	}

	tables := make(map[string]bool)

	for _, row := range locks {
		tables[fmt.Sprint(row[2])] = true

		switch row[2] {
		case "File":
			if row[1] != "hello.exe" || row[4] != "LOCKDOWN=1" {
				t.Errorf("file row %v", row)
			}
		case "CreateFolder":
			if row[1] != comp.Directory || row[4] != nil {
				t.Errorf("folder row %v", row)
			}
		}

		// The object exists in the named table (ICE104).
		if cell(t, res.Database, fmt.Sprint(row[2]), res.Database.Table(fmt.Sprint(row[2])).Columns[0].Name,
			func(r []any) bool { return r[0] == row[1] }) == nil {
			t.Errorf("%v names a missing object", row)
		}
	}

	if len(tables) != 3 {
		t.Errorf("tables %v", tables)
	}
}

func TestPermissionsErrors(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	t.Run("bad sddl", func(t *testing.T) {
		t.Parallel()

		cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
		cfg.Components[0].Files[0].Permissions = &config.Permissions{SDDL: "D:(A;;FA;;;[PERMSID])"}

		res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
		if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "1943") {
			t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
		}
	})

	t.Run("folder without row", func(t *testing.T) {
		t.Parallel()

		cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
		cfg.Components[0].FolderPermissions = &config.Permissions{SDDL: "D:(A;;FA;;;BA)"}

		res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
		if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "createFolder") {
			t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
		}
	})

	t.Run("both tables", func(t *testing.T) {
		t.Parallel()

		cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
		cfg.Components[0].Files[0].Permissions = &config.Permissions{SDDL: "D:(A;;FA;;;BA)"}
		cfg.Tables = append(cfg.Tables, &config.Table{Name: "LockPermissions", Columns: []config.Column{
			{Name: "LockObject", Type: "s72", Key: true}, {Name: "Table", Type: "s32", Key: true}, {Name: "Domain", Type: "S255", Key: true},
			{Name: "User", Type: "s255", Key: true}, {Name: "Permission", Type: "I4"},
		}, Rows: [][]any{{"hello.exe", "File", "BUILTIN", "Users", 268435456}}})

		res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
		if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "ICE104") {
			t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
		}
	})
}

func TestPermissionGrants(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	noInherit := false

	comp := cfg.Components[0]
	comp.Files[0].Permissions = &config.Permissions{
		Grant: []*config.Grant{{Account: "[SVC_ACCOUNT]", Rights: config.StringList{"readExecute", "0x10000"}}},
	}
	comp.CreateFolder = true
	comp.FolderPermissions = &config.Permissions{
		SDDL:      "D:(A;OICI;FA;;;BA)",
		Condition: "DATA=1",
		Grant: []*config.Grant{
			{Account: "NS", Rights: config.StringList{"modify"}},
			{Account: `.\svc`, Rights: config.StringList{"read"}, Inherit: &noInherit},
		},
	}
	comp.Services = []*config.Service{{Name: "HelloSvc", Permissions: &config.Permissions{
		Grant: []*config.Grant{{Account: "BU", Rights: config.StringList{"start", "stop"}}},
	}}}

	res := mustBuild(t, cfg)
	database := res.Database

	if !strings.Contains(fmt.Sprint(res.Diagnostics), "can be pointed at any folder") {
		t.Errorf("no warning for an inheriting grant on %s: %s", comp.Directory, res.Diagnostics)
	}

	if locks := rows(t, database, "MsiLockPermissionsEx"); len(locks) != 1 {
		t.Errorf("MsiLockPermissionsEx rows %v, want the folder SDDL only", locks)
	}

	installed := "$" + comp.ID + "=3"
	dir := "[" + comp.Directory + "]"
	seq := make(map[string]int)

	conds := map[string]any{}

	for _, row := range rows(t, database, "InstallExecuteSequence") {
		if number, ok := row[2].(int); ok {
			seq[fmt.Sprint(row[0])] = number
		}

		conds[fmt.Sprint(row[0])] = row[1]
	}

	// The engine builds the data: two complementary resets, then one
	// type-51 action per grant with its condition, in authoring order.
	data := "[BuilderGrantData]\n"
	want := []struct{ cond, target string }{
		{"NOT Installed", "i[ProductCode]"},
		{"Installed", "m[ProductCode]"},
		{installed + " AND (DATA=1)", data + "f|1301bf|3|NS|" + dir + "|NS"},
		{installed + " AND (DATA=1)", data + "f|120089|0|.\\svc|" + dir + "|.\\svc"},
		{installed, data + "s|30|0|BU|HelloSvc|BU"},
		{installed, data + "f|1300a9|0|[\\[]SVC_ACCOUNT[\\]]|[#hello.exe]|[SVC_ACCOUNT]"},
	}

	var setters []string

	for _, r := range rows(t, database, "CustomAction") {
		if r[2] == "BuilderGrantData" {
			setters = append(setters, fmt.Sprint(r[0]))
		}
	}

	slices.SortFunc(setters, func(x, y string) int { return seq[x] - seq[y] })

	if len(setters) != len(want) {
		t.Fatalf("BuilderGrantData setters %v, want %d", setters, len(want))
	}

	for i, name := range setters {
		r := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == name })
		if fmt.Sprint(r[1]) != "51" || r[3] != want[i].target || fmt.Sprint(conds[name]) != want[i].cond {
			t.Errorf("setter %d: %v condition %v, want %q if %q", i, r, conds[name], want[i].target, want[i].cond)
		}
	}

	// Then the data of the deferred actions, set in every install, so that
	// none comes from the command line, all before InstallValidate.
	prevSetter := setters[len(setters)-1]

	for _, final := range []struct{ name, property, target, cond string }{
		{"BuilderGrantSetExec", "BuilderGrantExec", "+[BuilderGrantData]", ""},
		{"BuilderGrantSetRollback", "BuilderGrantRollback", "-[BuilderGrantData]", ""},
		{"BuilderGrantSetCommit", "BuilderGrantCommit", "c[BuilderGrantData]", ""},
		{"BuilderGrantForget", "BuilderGrantCommit", "x[ProductCode]", `REMOVE~="ALL"`},
	} {
		r := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == final.name })
		if fmt.Sprint(r[1]) != "51" || r[2] != final.property || r[3] != final.target {
			t.Errorf("%s row %v", final.name, r)
		}

		if cond := conds[final.name]; (cond == nil && final.cond != "") || (cond != nil && fmt.Sprint(cond) != final.cond) {
			t.Errorf("%s condition %v, want %q", final.name, cond, final.cond)
		}

		if seq[final.name] <= seq[prevSetter] {
			t.Errorf("%s at %d is not after %s at %d", final.name, seq[final.name], prevSetter, seq[prevSetter])
		}

		prevSetter = final.name
	}

	if seq[prevSetter] >= seq["InstallValidate"] || seq[setters[0]] <= seq["CostFinalize"] {
		t.Errorf("grant data setters %d..%d not between CostFinalize %d and InstallValidate %d",
			seq[setters[0]], seq[prevSetter], seq["CostFinalize"], seq["InstallValidate"])
	}

	if _, ok := seq["BuilderGrantSchedule"]; ok {
		t.Error("BuilderGrantSchedule is scheduled; the engine builds the data")
	}

	for name, want := range map[string]string{
		"BuilderGrantRollback": `NOT REMOVE~="ALL"`, "BuilderGrantExec": `NOT REMOVE~="ALL"`,
	} {
		if conds[name] != want {
			t.Errorf("%s condition %v, want %q", name, conds[name], want)
		}
	}

	if cond := conds["BuilderGrantCommit"]; cond != nil && cond != "" {
		t.Errorf("BuilderGrantCommit condition %v, want none", cond)
	}

	prev := "InstallServices"

	for _, action := range []struct {
		name, entry string
		typ         int
	}{
		{"BuilderGrantRollback", "BuilderGrant", 1 | 0x500 | 0x800}, // DLL, rollback, no impersonation
		{"BuilderGrantExec", "BuilderGrant", 1 | 0x400 | 0x800},     // deferred
		{"BuilderGrantCommit", "BuilderGrant", 1 | 0x600 | 0x800},   // commit
	} {
		r := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == action.name })
		if fmt.Sprint(r[1]) != strconv.Itoa(action.typ) || r[2] != "BuilderRun" || r[3] != action.entry {
			t.Errorf("%s row %v", action.name, r)
		}

		if seq[action.name] <= seq[prev] {
			t.Errorf("%s at %d is not after %s at %d", action.name, seq[action.name], prev, seq[prev])
		}

		prev = action.name
	}

	if seq[prev] >= seq["StartServices"] {
		t.Errorf("%s at %d is not before StartServices at %d", prev, seq[prev], seq["StartServices"])
	}
}

func TestPermissionGrantErrors(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	yes := true

	for name, tc := range map[string]struct {
		set  func(*config.Component)
		want string
	}{
		"registry": {func(c *config.Component) {
			c.Registry = append(c.Registry, &config.RegistryValue{Root: "HKLM", Key: `Software\Hello`, Name: "x", Type: "string", Value: config.StringList{"x"},
				Permissions: &config.Permissions{Grant: []*config.Grant{{Account: "NS", Rights: config.StringList{"read"}}}}})
		}, "registry key with sddl"},
		"unknown right": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Account: "NS", Rights: config.StringList{"start"}}}}
		}, `unknown right "start"`},
		"no rights": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Account: "NS"}}}
		}, "rights is empty"},
		"empty account": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Rights: config.StringList{"read"}}}}
		}, "account is empty"},
		"inherit on a file": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Account: "NS", Rights: config.StringList{"read"}, Inherit: &yes}}}
		}, "folders only"},
		"generic mask": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Account: "NS", Rights: config.StringList{"0x10000000"}}}}
		}, "generic"},
		"pipe in account": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Account: "a|b", Rights: config.StringList{"read"}}}}
		}, "cannot contain |"},
		"line break in condition": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Condition: "A=1\nOR B=1", Grant: []*config.Grant{{Account: "NS", Rights: config.StringList{"read"}}}}
		}, "line breaks"},
		"zero mask": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Account: "NS", Rights: config.StringList{"0x0"}}}}
		}, "non-zero"},
		"neither sddl nor grant": {func(c *config.Component) {
			c.Files[0].Permissions = &config.Permissions{}
		}, "sddl is empty"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
			tc.set(cfg.Components[0])

			res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
			if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), tc.want) {
				t.Errorf("err %v, diagnostics %s, want %q", err, res.Diagnostics, tc.want)
			}
		})
	}

	t.Run("reserved property", func(t *testing.T) {
		t.Parallel()

		cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
		cfg.Components[0].Files[0].Permissions = &config.Permissions{Grant: []*config.Grant{{Account: "NS", Rights: config.StringList{"read"}}}}
		cfg.Properties = config.Properties{"BuilderGrantExec": {Value: "x"}}

		res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
		if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "reserved for permission grants") {
			t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
		}
	})
}
