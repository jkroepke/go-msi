package builder_test

import (
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// A numbering error is a diagnostic at the placement it concerns, also when
// only the build knows the number it runs into: InstallFiles comes with the
// File table, which validate does not decide.
func TestSequenceErrorDiagnostic(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	for name, tc := range map[string]struct {
		first, late config.Placement
		path, want  string
	}{
		"collision found by the build": {
			config.Placement{Sequence: "execute", After: "CostFinalize"}, config.Placement{Sequence: "execute", At: 4000},
			"customActions[1].schedule[0]", "InstallExecuteSequence: actions Late and InstallFiles both have number 4000",
		},
		"cycle found by validate": {
			config.Placement{Sequence: "execute", Before: "Late"}, config.Placement{Sequence: "execute", Before: "First"},
			"customActions[0].schedule[0]", "First → Late → First",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
			cfg.CustomActions = append(cfg.CustomActions,
				&config.CustomAction{ID: "First", SetProperty: &config.SetProperty{Name: "FIRST", Value: "1"}, Schedule: []config.Placement{tc.first}},
				&config.CustomAction{ID: "Late", SetProperty: &config.SetProperty{Name: "LATE", Value: "1"}, Schedule: []config.Placement{tc.late}},
			)

			res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
			if err == nil {
				t.Fatal("built")
			}

			for _, d := range res.Diagnostics {
				if d.Severity == config.SeverityError && d.Path == tc.path && strings.Contains(d.Message, tc.want) {
					return
				}
			}

			t.Errorf("no error at %s with %q: %v", tc.path, tc.want, res.Diagnostics)
		})
	}
}
