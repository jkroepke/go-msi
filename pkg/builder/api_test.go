package builder_test

import (
	"testing"

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
