package formatted_test

import (
	"testing"

	"github.com/jkroepke/msi-builder/internal/formatted"
)

func TestNested(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]bool{
		"[SVC_ACCOUNT]":          false,
		`.\[USER]`:               false,
		"[#File] [%ENV] [$Comp]": false,
		`[\[]x[\]]`:              false,
		`C:\[Logs`:               false,
		"[A][B]":                 false,
		"[[WHICH]]":              true,
		"[A[B]]":                 true,
		"[[%ACCT]]":              true,
		"[[#File]]":              true,
		`[A[\T]]`:                true,
	} {
		if got := formatted.Nested(text); got != want {
			t.Errorf("Nested(%q) = %v, want %v", text, got, want)
		}
	}
}
