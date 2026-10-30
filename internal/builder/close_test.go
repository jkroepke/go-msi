package builder_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

func TestCloseApps(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe, "tray.exe": exe})

	comp := cfg.Components[0]
	comp.Files[0].Close = &config.Close{}
	timeout := config.Duration(1500 * time.Millisecond)
	cfg.Components = append(cfg.Components, &config.Component{
		ID: "Tray", Directory: comp.Directory,
		Files: []*config.File{{Source: "tray.exe", Close: &config.Close{Timeout: &timeout, Terminate: new(false)}}},
	})
	cfg.Features[0].Components = append(cfg.Features[0].Components, cfg.Components[1])

	res := mustBuild(t, cfg)
	database := res.Database

	dir := "[" + comp.Directory + "]"
	want := strings.Join([]string{
		"$" + comp.ID + "=2 OR $" + comp.ID + "=3\x1f1388|\x1f" + dir + "hello.exe",
		"$Tray=2 OR $Tray=3\x1f5dc|\x1f" + dir + "tray.exe",
	}, "\n")

	if got := cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "BuilderCloseApps" }); got != want {
		t.Errorf("BuilderCloseApps\n%q\nwant\n%q", got, want)
	}

	seq := make(map[string]int)
	conds := map[string]any{}

	for _, row := range rows(t, database, "InstallExecuteSequence") {
		if number, ok := row[2].(int); ok {
			seq[fmt.Sprint(row[0])] = number
		}

		conds[fmt.Sprint(row[0])] = row[1]
	}

	for _, action := range []struct {
		name, entry string
		typ         int
	}{
		{"BuilderCloseSchedule", "BuilderCloseSchedule", 1},
		{"BuilderCloseExec", "BuilderClose", 1 | 0x400 | 0x800}, // DLL, deferred, no impersonation
	} {
		r := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == action.name })
		if fmt.Sprint(r[1]) != strconv.Itoa(action.typ) || r[2] != "BuilderRun" || r[3] != action.entry {
			t.Errorf("%s row %v", action.name, r)
		}
	}

	if cond := conds["BuilderCloseSchedule"]; cond != nil && cond != "" {
		t.Errorf("BuilderCloseSchedule condition %v, want none", cond)
	}

	if conds["BuilderCloseExec"] != "BuilderCloseExec" {
		t.Errorf("BuilderCloseExec condition %v, want its own property", conds["BuilderCloseExec"])
	}

	if seq["BuilderCloseSchedule"] <= seq["CostFinalize"] || seq["BuilderCloseSchedule"] >= seq["InstallValidate"] {
		t.Errorf("BuilderCloseSchedule at %d is not between CostFinalize and InstallValidate: %v", seq["BuilderCloseSchedule"], seq)
	}

	if seq["BuilderCloseExec"] <= seq["InstallInitialize"] || seq["BuilderCloseExec"] >= seq["RemoveFiles"] {
		t.Errorf("BuilderCloseExec at %d is not between InstallInitialize and RemoveFiles: %v", seq["BuilderCloseExec"], seq)
	}

	// Only hello.exe terminates: one type-51 action, run by the engine
	// under the component condition, appends its path to the deferred
	// action's data, right before that action.
	var setters []string

	for _, row := range rows(t, database, "CustomAction") {
		if strings.HasPrefix(fmt.Sprint(row[0]), "BuilderCloseSet") {
			setters = append(setters, fmt.Sprint(row[0]))

			if fmt.Sprint(row[1]) != "51" || row[2] != "BuilderCloseExec" || row[3] != "[BuilderCloseExec]"+dir+"hello.exe|" {
				t.Errorf("setter row %v", row)
			}

			if want := "$" + comp.ID + "=2 OR $" + comp.ID + "=3"; conds[fmt.Sprint(row[0])] != want {
				t.Errorf("setter condition %v, want %q", conds[fmt.Sprint(row[0])], want)
			}

			if seq[fmt.Sprint(row[0])] <= seq["StopServices"] && seq["StopServices"] != 0 || seq[fmt.Sprint(row[0])] >= seq["BuilderCloseExec"] {
				t.Errorf("setter at %d, BuilderCloseExec at %d", seq[fmt.Sprint(row[0])], seq["BuilderCloseExec"])
			}
		}
	}

	if len(setters) != 1 {
		t.Errorf("setters %v, want one for hello.exe", setters)
	}

	if rowWhere(t, database, "Binary", func(r []any) bool { return r[0] == "BuilderRun" }) == nil {
		t.Error("the DLL is not in the Binary table")
	}
}

func TestCloseAppsOff(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	cfg.Components[0].Files[0].Close = &config.Close{Disabled: true}

	database := mustBuild(t, cfg).Database
	for _, row := range rows(t, database, "CustomAction") {
		if strings.HasPrefix(fmt.Sprint(row[0]), "BuilderClose") {
			t.Errorf("close: false wrote %v", row)
		}
	}
}

func TestCloseAppsReserved(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	cfg.Components[0].Files[0].Close = &config.Close{}
	cfg.Properties = config.Properties{}
	cfg.Properties["BuilderCloseApps"] = &config.Property{Value: "x"}

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "reserved for closing running programs") {
		t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
	}
}

func TestCloseAppsWithoutTerminate(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	cfg.Components[0].Files[0].Close = &config.Close{Terminate: new(false)}

	database := mustBuild(t, cfg).Database
	for _, row := range rows(t, database, "CustomAction") {
		if name := fmt.Sprint(row[0]); name != "BuilderCloseSchedule" && strings.HasPrefix(name, "BuilderClose") {
			t.Errorf("terminate: false wrote %v", row)
		}
	}
}
