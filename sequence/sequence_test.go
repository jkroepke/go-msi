package sequence //nolint:testpackage // the tests reach unexported encoders

import (
	"fmt"
	"strings"
	"testing"
)

func names(seq []Numbered) string {
	var sb strings.Builder

	for _, n := range seq {
		fmt.Fprintf(&sb, "%s=%d ", n.Name, n.Sequence)
	}

	return sb.String()
}

// The examples of docs/spec/MSISPEC.md section 11.7.
func TestWiXExamples(t *testing.T) {
	t.Parallel()

	seq, err := Number([]Action{
		{Name: "InstallFinalize", Sequence: 6600},
		{Name: "InstallValidate", Sequence: 1400},
		{Name: "A", Before: "InstallFinalize"},
		{Name: "B", After: "InstallFinalize"},
		{Name: "REP", After: "InstallValidate"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := names(seq); got != "InstallValidate=1400 REP=1401 A=6599 InstallFinalize=6600 B=6601 " {
		t.Error(got)
	}

	seq, err = Number([]Action{
		{Name: "WriteRegistryValues", Sequence: 5000},
		{Name: "PrepA", Before: "WriteRegistryValues"},
		{Name: "PrepA2", After: "PrepA"},
		{Name: "PrepB", Before: "WriteRegistryValues"},
		{Name: "PostA", After: "WriteRegistryValues"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := names(seq); got != "PrepA=4997 PrepA2=4998 PrepB=4999 WriteRegistryValues=5000 PostA=5001 " {
		t.Error(got)
	}

	seq, err = Number([]Action{
		{Name: "StopServices", Sequence: 1900, Condition: "VersionNT"},
		{Name: "X1", Before: "X2"},
		{Name: "X2", Before: "StopServices"},
		{Name: "BindImage", Sequence: 4300},
		{Name: "P1", After: "BindImage"}, {Name: "P2", After: "BindImage"}, {Name: "P3", After: "BindImage"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := names(seq); got != "X1=1898 X2=1899 StopServices=1900 BindImage=4300 P1=4301 P2=4302 P3=4303 " {
		t.Error(got)
	}

	if Find(seq, "StopServices") != 1900 || seq[2].Condition != "VersionNT" || Find(seq, "nope") != 0 {
		t.Error("Find or condition lost")
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()

	for name, actions := range map[string][]Action{
		"missing anchor":  {{Name: "A", Before: "Nope"}},
		"cycle":           {{Name: "A", Before: "B"}, {Name: "B", Before: "A"}},
		"both":            {{Name: "A", Sequence: 5, Before: "B"}, {Name: "B", Sequence: 10}},
		"neither":         {{Name: "A"}},
		"duplicate":       {{Name: "A", Sequence: 5}, {Name: "A", Sequence: 10}},
		"same number":     {{Name: "A", Sequence: 5}, {Name: "B", Sequence: 5}},
		"collision":       {{Name: "A", Sequence: 100}, {Name: "B", Sequence: 101}, {Name: "C", After: "A"}},
		"below one":       {{Name: "A", Sequence: 1}, {Name: "B", Before: "A"}},
		"above max":       {{Name: "A", Sequence: 32767}, {Name: "B", After: "A"}},
		"bad number":      {{Name: "A", Sequence: -5}},
		"termination":     {{Name: "A", Sequence: -1}, {Name: "B", After: "A"}},
		"zero and above":  {{Name: "A", Sequence: 40000}},
		"unnamed":         {{Sequence: 1}},
		"chain collision": {{Name: "A", Sequence: 100}, {Name: "B", Sequence: 103}, {Name: "C", After: "A"}, {Name: "D", After: "C"}, {Name: "E", After: "D"}},
	} {
		if _, err := Number(actions); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestTerminationActionsAreFine(t *testing.T) {
	t.Parallel()

	seq, err := Number([]Action{{Name: "ExitDialog", Sequence: -1}, {Name: "FatalError", Sequence: -3}, {Name: "A", Sequence: 10}})
	if err != nil {
		t.Fatal(err)
	}

	if got := names(seq); got != "FatalError=-3 ExitDialog=-1 A=10 " {
		t.Error(got)
	}
}
