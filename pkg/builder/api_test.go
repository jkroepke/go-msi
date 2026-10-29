package builder_test

import (
	"reflect"
	"testing"

	internal "github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
)

func TestBuildReturnsDiagnostics(t *testing.T) {
	t.Parallel()

	res, err := builder.Build(&config.Config{Schema: config.SchemaV1}, builder.Options{})
	if err == nil {
		t.Fatal("Build accepted a config without a product")
	}

	if res == nil || len(res.Diagnostics) == 0 {
		t.Fatalf("Build returned no diagnostics with %v", err)
	}
}

// TestResultFieldsMatch fails when internal/builder.Result gains a field that
// Build does not copy into the public Result.
func TestResultFieldsMatch(t *testing.T) {
	t.Parallel()

	public := fieldNames(reflect.TypeFor[builder.Result]())
	public["Database"] = true // deliberately internal

	for name := range fieldNames(reflect.TypeFor[internal.Result]()) {
		if !public[name] {
			t.Errorf("internal Result field %s is missing from the public Result", name)
		}
	}
}

func fieldNames(typ reflect.Type) map[string]bool {
	names := make(map[string]bool, typ.NumField())
	for field := range typ.Fields() {
		names[field.Name] = true
	}

	return names
}
