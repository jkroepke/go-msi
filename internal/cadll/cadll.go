// Package cadll holds the custom action DLL that go-msi embeds into packages
// for `run` actions (docs/design/yaml-schema.md section 4.9). The DLL is
// built from src/gomsirun.c by build.sh (go generate), once per platform,
// and committed; go-msi itself stays free of cgo because it only copies the
// bytes into the Binary table. SHA256SUMS pins the committed builds and the
// test checks them, their PE machine, their exports and their imports.
package cadll

// The rebuild needs clang 18, lld and llvm-dlltool (CLANG and DLLTOOL name
// the binaries); build.sh refuses another clang, whose bytes would differ.
//go:generate sh ./build.sh

import (
	"embed"
	"fmt"
)

// Key is the Binary table key of the DLL and Entry its exported entry point.
const (
	Key   = "GoMsiRun"
	Entry = "GoMsiRun"

	// DataProperty is the property an immediate action reads its command
	// from; deferred actions read CustomActionData, which the engine takes
	// from a property named like the action.
	DataProperty = "GoMsiRun"

	// Separator ends the working directory in the data. It cannot occur in
	// a Windows path, so a directory never needs escaping; what follows the
	// first separator is the command line.
	Separator = "|"
)

//go:embed gomsirun-x86.dll gomsirun-x64.dll gomsirun-arm64.dll
var dlls embed.FS

// Binary returns the DLL for a Template platform token: Intel, x64 or Arm64.
func Binary(platform string) ([]byte, error) {
	var name string

	switch platform {
	case "Intel":
		name = "gomsirun-x86.dll"
	case "x64":
		name = "gomsirun-x64.dll"
	case "Arm64":
		name = "gomsirun-arm64.dll"
	default:
		return nil, fmt.Errorf("cadll: no custom action DLL for platform %q", platform)
	}

	data, err := dlls.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("cadll: %w", err)
	}

	return data, nil
}

// Data encodes the working directory and command line the DLL expects.
func Data(directory, command string) string {
	return directory + Separator + command
}
