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

	res := mustBuild(t, cfg)
	database := res.Database

	if locks := rows(t, database, "MsiLockPermissionsEx"); len(locks) != 1 {
		t.Errorf("MsiLockPermissionsEx rows %v, want the folder SDDL only", locks)
	}

	want := map[string]string{
		"+f|1300a9|0|[SVC_ACCOUNT]|[#hello.exe]":     "$" + comp.ID + "=3",
		"+f|1301bf|3|NS|[" + comp.Directory + "]":    "$" + comp.ID + "=3 AND (DATA=1)",
		`+f|120089|0|.\svc|[` + comp.Directory + "]": "$" + comp.ID + "=3 AND (DATA=1)",
	}

	seq := make(map[string]int)

	for _, row := range rows(t, database, "InstallExecuteSequence") {
		if number, ok := row[2].(int); ok {
			seq[fmt.Sprint(row[0])] = number
		}
	}

	found := 0

	for _, row := range rows(t, database, "CustomAction") {
		data := fmt.Sprint(row[3])

		cond, ok := want[data]
		if !ok || fmt.Sprint(row[1]) != "51" {
			continue
		}

		found++
		deferred := fmt.Sprint(row[2])
		rollback := deferred + "Rollback"

		if got := cell(t, database, "CustomAction", "Target", func(r []any) bool { return r[0] == rollback+".Data" }); got != "-"+data[1:] {
			t.Errorf("%s data %v, want %s", rollback, got, "-"+data[1:])
		}

		for _, action := range []struct {
			name string
			typ  int
		}{{rollback, 1 | 0x500 | 0x800}, {deferred, 1 | 0x400 | 0x800}} { // DLL, rollback or deferred, no impersonation
			r := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == action.name })
			if fmt.Sprint(r[1]) != strconv.Itoa(action.typ) || r[2] != "BuilderRun" || r[3] != "BuilderGrant" {
				t.Errorf("%s row %v", action.name, r)
			}
		}

		order := []string{deferred + ".Data", rollback + ".Data", rollback, deferred}
		prev := ""

		for _, name := range order {
			if got := cell(t, database, "InstallExecuteSequence", "Condition", func(r []any) bool { return r[0] == name }); got != cond {
				t.Errorf("%s condition %v, want %s", name, got, cond)
			}

			if prev != "" && seq[name] <= seq[prev] {
				t.Errorf("%s at %d is not after %s at %d", name, seq[name], prev, seq[prev])
			}

			prev = name
		}

		if seq[deferred+".Data"] <= seq["InstallFiles"] || seq[deferred] >= seq["InstallFinalize"] {
			t.Errorf("%s runs at %d..%d, InstallFiles at %d", deferred, seq[deferred+".Data"], seq[deferred], seq["InstallFiles"])
		}
	}

	if found != len(want) {
		t.Errorf("found %d of %d grants", found, len(want))
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
}
