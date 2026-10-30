// Package cadll holds the custom action DLL that msi-builder embeds into packages
// for `run` actions (docs/design/yaml-schema.md section 4.9). The DLL is
// built from src/builderrun.c by build.sh (go generate), once per platform,
// and committed; msi-builder itself stays free of cgo because it only copies the
// bytes into the Binary table. SHA256SUMS pins the committed builds and the
// test checks them, their PE machine, their exports and their imports.
package cadll

// The rebuild needs clang 18, lld and llvm-dlltool (CLANG and DLLTOOL name
// the binaries); build.sh refuses another clang, whose bytes would differ.
//go:generate sh ./build.sh

import (
	"embed"
	"fmt"
	"strings"
)

// Key is the Binary table key of the DLL and Entry the exported entry point
// of run actions.
const (
	Key   = "BuilderRun"
	Entry = "BuilderRun"

	// DataProperty is the property an immediate action reads its command
	// from; deferred actions read CustomActionData, which the engine takes
	// from a property named like the action.
	DataProperty = "BuilderRun"

	// Separator ends the working directory in the data. It cannot occur in
	// a Windows path, so a directory never needs escaping; what follows the
	// first separator is the command line.
	Separator = "|"
)

//go:embed builderrun-x86.dll builderrun-x64.dll builderrun-arm64.dll
var dlls embed.FS

// Binary returns the DLL for a Template platform token: Intel, x64 or Arm64.
func Binary(platform string) ([]byte, error) {
	var name string

	switch platform {
	case "Intel":
		name = "builderrun-x86.dll"
	case "x64":
		name = "builderrun-x64.dll"
	case "Arm64":
		name = "builderrun-arm64.dll"
	default:
		return nil, fmt.Errorf("cadll: no custom action DLL for platform %q", platform)
	}

	data, err := dlls.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("cadll: %w", err)
	}

	return data, nil
}

// Permission grants: the property the builder lists them in, the actions
// that apply them (named like their entry points where they have their own)
// and the kinds of object.
const (
	GrantProperty   = "BuilderGrants"
	GrantSchedule   = "BuilderGrantSchedule"
	GrantEntryPoint = "BuilderGrant"
	GrantExec       = "BuilderGrantExec"
	GrantRollback   = "BuilderGrantRollback"
	GrantCommit     = "BuilderGrantCommit"

	GrantFile    = 'f'
	GrantService = 's'

	// grantSeparator separates the fields of a BuilderGrants line: the
	// condition, the rights, the account and the object.
	grantSeparator = "\x1f"
)

// GrantEntry encodes one line of the BuilderGrants property: the condition
// under which it applies, the kind of object, the access mask, the ACE
// inheritance flags, and the account and object, which BuilderGrantSchedule
// formats. None of the texts may contain a line break or U+001F.
func GrantEntry(condition string, kind byte, mask, flags uint32, account, object string) string {
	return strings.Join([]string{condition, fmt.Sprintf("%c|%x|%x", kind, mask, flags), account, object}, grantSeparator)
}

// Closing running programs: the property the builder lists the files in,
// the immediate action that asks them to close, the entry point and the
// deferred action that ends what is left, and the flag that asks for it.
const (
	CloseProperty   = "BuilderCloseApps"
	CloseSchedule   = "BuilderCloseSchedule"
	CloseEntryPoint = "BuilderClose"
	CloseExec       = "BuilderCloseExec"

	CloseTerminate = 1
)

// CloseEntry encodes one line of the BuilderCloseApps property: the
// condition under which it applies, the time in milliseconds the programs
// get to close, the flags and the path of the program, which
// BuilderCloseSchedule formats. None of the texts may contain a line break
// or U+001F.
func CloseEntry(condition string, timeoutMS, flags uint32, path string) string {
	return strings.Join([]string{condition, fmt.Sprintf("%x|%x|", timeoutMS, flags), path}, grantSeparator)
}

// Data encodes the working directory and command line the DLL expects.
func Data(directory, command string) string {
	return directory + Separator + command
}
