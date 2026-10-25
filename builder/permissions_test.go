package builder_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/builder"
	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/internal/testpe"
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
