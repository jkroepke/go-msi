package builder_test

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jkroepke/go-msi/pkg/builder"
	"github.com/jkroepke/go-msi/pkg/config"
)

// variantConfig installs a.txt from two conditional components in two
// directory ids for one folder, like an x64 and an x86 variant.
func variantConfig(bitnessB, guidA, guidB string) *config.Config {
	cfg := goConfig()
	cfg.FS = fstest.MapFS{"x64/a.txt": {Data: []byte("64")}, "x86/a.txt": {Data: []byte("32")}}
	cfg.Directories[0].Children = append(cfg.Directories[0].Children, &config.Directory{ID: "INSTALLDIR32", Name: "Hello"})
	cfg.Components = []*config.Component{
		{ID: "A64", Directory: "INSTALLDIR", Condition: "VersionNT64", GUID: guidA, Files: []*config.File{{Source: "x64/a.txt"}}},
		{ID: "A32", Directory: "INSTALLDIR32", Condition: "NOT VersionNT64", Bitness: bitnessB, GUID: guidB, Files: []*config.File{{Source: "x86/a.txt", ID: "a32"}}},
	}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("A64"), config.ComponentRef("A32")}

	return cfg
}

func TestBuildExclusiveVariants(t *testing.T) {
	t.Parallel()

	const pinned = "3f2504e0-4f89-41d3-9a0c-0305e82c3401"

	for _, tc := range []struct {
		name, bitnessB, guidA, guidB string
		wantErr                      string
	}{
		{"automatic GUIDs, other bitness", "32", config.Auto, config.Auto, ""},
		{"automatic GUIDs, same bitness", config.Auto, config.Auto, config.Auto, "same ComponentId"},
		{"one pinned GUID", config.Auto, pinned, config.Auto, ""},
		{"same pinned GUID", config.Auto, pinned, pinned, "same ComponentId"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := builder.Build(variantConfig(tc.bitnessB, tc.guidA, tc.guidB), builder.Options{})

			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("build: %v\n%s", err, res.Diagnostics)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			case tc.wantErr == "" && !strings.Contains(fmt.Sprint(res.Diagnostics), "ICE30"):
				t.Errorf("no ICE30 warning in\n%s", res.Diagnostics)
			}
		})
	}
}

func TestBuildDirectoryKeyPathsShareComponentID(t *testing.T) {
	t.Parallel()

	// Two components without files or registry values both have INSTALLDIR as
	// key path, so their automatic GUIDs are equal.
	cfg := goConfig()
	cfg.Components = []*config.Component{
		{ID: "Path", Directory: "INSTALLDIR", Environment: []*config.Environment{{Name: "PATH", Value: "[INSTALLDIR]", Action: "append"}}},
		{ID: "Home", Directory: "INSTALLDIR", Environment: []*config.Environment{{Name: "HELLO_HOME", Value: "[INSTALLDIR]"}}},
	}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("Path"), config.ComponentRef("Home")}

	res, err := builder.Build(cfg, builder.Options{})
	if err == nil {
		t.Fatal("two components with one ComponentId build")
	}

	if got := fmt.Sprint(res.Diagnostics); !strings.Contains(got, "components[1].guid") || !strings.Contains(got, "same ComponentId") {
		t.Errorf("diagnostics do not explain the error: %s", got)
	}

	cfg.Components[1].GUID = "3f2504e0-4f89-41d3-9a0c-0305e82c3409"
	mustBuild(t, cfg)
}
