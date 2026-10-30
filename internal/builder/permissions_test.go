package builder_test

import (
	"fmt"
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

	// The engine builds the data right before the deferred actions: two
	// complementary resets and a reset of the count, then two type-51
	// actions per grant with its condition, in authoring order, then the
	// data of each deferred action, set in every install, so that none
	// comes from the command line.
	data, count := "[BuilderGrantData]\n", "[BuilderGrantCount]#"
	want := []struct{ property, cond, target string }{
		{"BuilderGrantData", "NOT Installed", "i[ProductCode]"},
		{"BuilderGrantData", "Installed", "m[ProductCode]"},
		{"BuilderGrantCount", "", ":"},
		{"BuilderGrantData", installed + " AND (DATA=1)", data + "f|1301bf|3|NS|" + dir + "|NS"},
		{"BuilderGrantCount", installed + " AND (DATA=1)", count},
		{"BuilderGrantData", installed + " AND (DATA=1)", data + "f|120089|0|.\\svc|" + dir + "|.\\svc"},
		{"BuilderGrantCount", installed + " AND (DATA=1)", count},
		{"BuilderGrantData", installed, data + "s|30|0|BU|HelloSvc|BU"},
		{"BuilderGrantCount", installed, count},
		{"BuilderGrantData", installed, data + "f|1300a9|0|[\\[]SVC_ACCOUNT[\\]]|[#hello.exe]|[SVC_ACCOUNT]"},
		{"BuilderGrantCount", installed, count},
		{"BuilderGrantExec", "", "+[BuilderGrantCount][BuilderGrantData]"},
		{"BuilderGrantRollback", "", "-[BuilderGrantCount][BuilderGrantData]"},
		{"BuilderGrantCommit", "", "c[BuilderGrantCount][BuilderGrantData]"},
		{"BuilderGrantCommit", `REMOVE~="ALL"`, "x[ProductCode]"},
		{"BuilderRun", `NOT REMOVE~="ALL"`, "BuilderGrant"}, // BuilderGrantRollback
		{"BuilderRun", `NOT REMOVE~="ALL"`, "BuilderGrant"}, // BuilderGrantExec
		{"BuilderRun", "", "BuilderGrant"},                  // BuilderGrantCommit
	}

	// Nothing, above all no action that runs as the user, comes between
	// the first reset and the deferred actions.
	first := seq["BuilderGrantReset"]
	byNumber := map[int]string{}

	for name, number := range seq {
		byNumber[number] = name
	}

	for i, action := range want {
		name := byNumber[first+i]
		r := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == name })

		if r[2] != action.property || r[3] != action.target || strings.TrimSuffix(fmt.Sprint(conds[name]), "<nil>") != action.cond {
			t.Errorf("action %d at %d: %v condition %v, want %s=%q if %q", i, first+i, r, conds[name], action.property, action.target, action.cond)
		}
	}

	if first <= seq["InstallServices"] || first+len(want) > seq["StartServices"] {
		t.Errorf("grant actions %d..%d not between InstallServices and StartServices: %v", first, first+len(want)-1, seq)
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

// TestPermissionGrantRoom builds the most grants that fit after
// InstallServices, which MsiConfigureServices at 5850 halves, and after
// InstallFiles without services, where DuplicateFiles (4210) leaves more.
func TestPermissionGrantRoom(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	for _, tc := range []struct {
		grants           int
		service, delayed bool
		want             string
	}{
		{44, true, false, ""},
		{45, true, false, "at most 44 fit"},
		{19, true, true, ""},
		{20, true, true, "at most 19 fit"},
		{100, false, false, ""},
	} {
		t.Run(fmt.Sprintf("%d grants service %v delayed %v", tc.grants, tc.service, tc.delayed), func(t *testing.T) {
			t.Parallel()

			cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
			grants := make([]*config.Grant, tc.grants)

			for i := range grants {
				grants[i] = &config.Grant{Account: "NS", Rights: config.StringList{"read"}}
			}

			cfg.Components[0].Files[0].Permissions = &config.Permissions{Grant: grants}
			if tc.service {
				cfg.Components[0].Services = []*config.Service{{Name: "HelloSvc", Start: "auto", DelayedAutoStart: tc.delayed}}
			}

			_, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
			if (tc.want == "" && err != nil) || (tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want))) {
				t.Errorf("err %v, want %q", err, tc.want)
			}
		})
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
