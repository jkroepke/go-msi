package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// setProps returns custom actions that set a property, one per placement.
func setProps(placements ...string) string {
	var sb strings.Builder

	sb.WriteString("customActions:\n")

	for i, placement := range placements {
		fmt.Fprintf(&sb, "  - id: Set%d\n    setProperty: {name: P%d, value: x}\n    schedule: [{sequence: execute, %s}]\n", i, i, placement)
	}

	return sb.String()
}

// Validate numbers the sequences as far as the configuration decides them,
// so that what the build would reject fails validate at its placement.
func TestScheduleConflicts(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		doc, path, msg string
	}{
		// Two actions anchored to each other passed validate and
		// failed the build without a path.
		"cycle": {
			setProps("before: Set1", "before: Set0"),
			"customActions[0].schedule[0]", "Set0 → Set1 → Set0",
		},
		// An action that only hangs off the cycle is not blamed.
		"cycle behind an innocent action": {
			setProps("before: Set1", "before: Set2", "before: Set1"),
			"customActions[1].schedule[0]", "Set1 → Set2 → Set1",
		},
		"chain into a fixed number": {
			setProps("at: 1402", "after: InstallValidate", "after: Set1"),
			"customActions[2].schedule[0]", "Set0 already has",
		},
		"same number": {
			setProps("at: 1400", "at: 1500"),
			"customActions[0].schedule[0]", "InstallValidate",
		},
		"below one": {
			setProps("at: 1", "before: Set0"),
			"customActions[1].schedule[0]", "below 1",
		},
		"above max": {
			setProps("at: 32767", "after: Set0"),
			"customActions[1].schedule[0]", "above 32767",
		},
		"twice in a sequence": {
			"customActions:\n  - id: Twice\n    setProperty: {name: P, value: x}\n" +
				"    schedule: [{sequence: execute, after: CostFinalize}, {sequence: execute, before: InstallFinalize}]\n",
			"customActions[0].schedule[1]", "already scheduled in this sequence at customActions[0].schedule[0]",
		},
		"anchor in another sequence": {
			"customActions:\n  - id: UIOnly\n    setProperty: {name: P, value: x}\n    schedule: [{sequence: ui, after: CostFinalize}]\n" +
				"  - id: Next\n    setProperty: {name: Q, value: x}\n    schedule: [{sequence: execute, after: UIOnly}]\n",
			"customActions[1].schedule[0]", "not scheduled in the execute sequence",
		},
		// The data action of a run action takes the number before it.
		"run data action": {
			"customActions:\n  - id: Fixed\n    setProperty: {name: P, value: x}\n    schedule: [{sequence: execute, at: 4003}]\n" +
				"  - id: Run\n    run: {command: cmd.exe}\n    return: check\n    execute: deferred\n    schedule: [{sequence: execute, before: Fixed}]\n" +
				"  - id: Late\n    setProperty: {name: Q, value: x}\n    schedule: [{sequence: execute, after: InstallFiles}]\n",
			"customActions[1].schedule[0]", "Run.Data",
		},
	} {
		ds := validate(t, base+appComponent+test.doc)
		if !find(ds, config.SeverityError, test.path, test.msg) {
			t.Errorf("%s: no error at %s with %q: %v", name, test.path, test.msg, ds)
		}
	}
}

// Chains that end at an action whose number only the builder knows, and
// chains that fit, pass.
func TestScheduleAccepted(t *testing.T) {
	t.Parallel()

	for name, doc := range map[string]string{
		"chains":                     setProps("after: InstallValidate", "after: InstallValidate", "after: Set0", "before: InstallFinalize", "at: 1650"),
		"after a builder placement":  setProps("after: RemoveExistingProducts", "after: Set0", "at: 1402"),
		"in different sequences":     setProps("after: CostFinalize") + "  - id: Other\n    setProperty: {name: Q, value: x}\n    schedule: [{sequence: ui, after: CostFinalize}]\n",
		"standard without a default": setProps("after: ScheduleReboot"),
	} {
		ds := validate(t, base+appComponent+doc)
		if err := ds.Err(); err != nil {
			t.Errorf("%s: %v", name, ds)
		}
	}
}
