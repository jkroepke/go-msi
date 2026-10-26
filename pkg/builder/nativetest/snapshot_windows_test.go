//go:build windows

package nativetest_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"
)

//nolint:gochecknoglobals // lazy DLL procs, loaded on first use
var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	pRegEnumValueW = advapi32.NewProc("RegEnumValueW")
)

// snapshot is the machine state an installation may change, by entry: a
// directory, file, registry key, registry value or firewall rule, each with
// a description of its content.
type snapshot map[string]string

// watch is a place an installation writes to. A snapshot records its direct
// children; a child that the baseline does not have is recorded with
// everything below it, so the noise of the rest of the machine stays out.
type watch struct {
	// label names the place in the snapshot, such as {ProgramFiles} or a
	// registry path.
	label string
	// dir is a directory; when it is empty, key below root is a registry key.
	dir  string
	root syscall.Handle
	key  string
	// hashes records the size and SHA-256 of a file that is not one of the
	// inputs; otherwise only its presence is recorded.
	hashes bool
	// skip leaves out children whose names Windows Installer picks at random.
	skip *regexp.Regexp
}

// volatileValues are registry values that change with every installation.
var volatileValues = map[string]string{ //nolint:gochecknoglobals // lookup table
	"InstallDate": "<date>",
	// Windows Installer computes it from the file costs, and the service
	// examples install a test service built by the local Go toolchain.
	"EstimatedSize": "<size>",
}

// firewallVersion is the schema prefix of a firewall rule, which differs
// between Windows versions.
var firewallVersion = regexp.MustCompile(`^v\d+\.\d+\|`)

func watches() []watch {
	programData := os.Getenv("ProgramData")
	cachedPackage := regexp.MustCompile(`(?i)^[0-9a-f]+\.(msi|msp|mst)$`)

	return []watch{
		{label: "{ProgramFiles}", dir: os.Getenv("ProgramFiles"), hashes: true},
		{label: "{ProgramFiles(x86)}", dir: os.Getenv("ProgramFiles(x86)"), hashes: true},
		{label: "{ProgramData}", dir: programData, hashes: true},
		{label: "{CommonPrograms}", dir: filepath.Join(programData, `Microsoft\Windows\Start Menu\Programs`), hashes: true},
		{label: "{CommonDesktop}", dir: filepath.Join(os.Getenv("PUBLIC"), "Desktop"), hashes: true},
		{label: "{Programs}", dir: filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`), hashes: true},
		{label: "{Desktop}", dir: filepath.Join(os.Getenv("USERPROFILE"), "Desktop"), hashes: true},
		// The cached package gets a random name; the icons go to {ProductCode}.
		{label: "{Installer}", dir: filepath.Join(os.Getenv("SystemRoot"), "Installer"), skip: cachedPackage},
		{label: `HKLM\SOFTWARE`, root: syscall.HKEY_LOCAL_MACHINE, key: `SOFTWARE`},
		{label: `HKLM\SOFTWARE\WOW6432Node`, root: syscall.HKEY_LOCAL_MACHINE, key: `SOFTWARE\WOW6432Node`},
		{label: `HKLM\...\Uninstall`, root: syscall.HKEY_LOCAL_MACHINE, key: `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{label: `HKLM\...\WOW6432Node\...\Uninstall`, root: syscall.HKEY_LOCAL_MACHINE, key: `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{label: `HKLM\...\Services`, root: syscall.HKEY_LOCAL_MACHINE, key: `SYSTEM\CurrentControlSet\Services`},
		{label: `HKLM\...\EventLog\Application`, root: syscall.HKEY_LOCAL_MACHINE, key: `SYSTEM\CurrentControlSet\Services\EventLog\Application`},
		{label: `HKCU\Software`, root: syscall.HKEY_CURRENT_USER, key: `Software`},
	}
}

// pathLabels replaces the machine-specific paths in names and values: the
// folder the package was installed from and the known folders.
func pathLabels(packageDir string) func(string) string {
	type path struct{ label, path string }

	paths := []path{
		{"{PackageDir}", packageDir},
		{"{PackageDir}", longPath(packageDir)},
		{"{ProgramFiles}", os.Getenv("ProgramFiles")},
		{"{ProgramFiles(x86)}", os.Getenv("ProgramFiles(x86)")},
		{"{CommonProgramFiles}", os.Getenv("CommonProgramFiles")},
		{"{ProgramData}", os.Getenv("ProgramData")},
		{"{PUBLIC}", os.Getenv("PUBLIC")},
		{"{APPDATA}", os.Getenv("APPDATA")},
		{"{LOCALAPPDATA}", os.Getenv("LOCALAPPDATA")},
		{"{USERPROFILE}", os.Getenv("USERPROFILE")},
		{"{SystemRoot}", os.Getenv("SystemRoot")},
	}

	// The longest path first, so that C:\Program Files (x86) is not taken
	// for C:\Program Files.
	slices.SortStableFunc(paths, func(a, b path) int { return len(b.path) - len(a.path) })

	type replacement struct {
		pattern *regexp.Regexp
		label   string
	}

	var replacements []replacement

	for _, p := range paths {
		if p.path != "" {
			replacements = append(replacements, replacement{regexp.MustCompile(`(?i)` + regexp.QuoteMeta(filepath.Clean(p.path))), p.label})
		}
	}

	return func(s string) string {
		for _, r := range replacements {
			s = r.pattern.ReplaceAllLiteralString(s, r.label)
		}

		return s
	}
}

// longPath expands the 8.3 names in path, such as RUNNER~1 in the temporary
// folder of a GitHub runner.
func longPath(path string) string {
	short, err := syscall.UTF16FromString(path)
	if err != nil {
		return path
	}

	var buf [syscall.MAX_LONG_PATH]uint16

	size, err := syscall.GetLongPathName(&short[0], &buf[0], syscall.MAX_LONG_PATH)
	if err != nil || size > syscall.MAX_LONG_PATH {
		return path
	}

	return syscall.UTF16ToString(buf[:size])
}

// takeSnapshot records every watch. Without a baseline it records only the
// direct children; with one it also records the subtree of every child the
// baseline does not have.
func takeSnapshot(t *testing.T, base snapshot, label func(string) string, inputs map[string][]byte) snapshot {
	t.Helper()

	snap := snapshot{}

	for _, w := range watches() {
		if w.dir != "" {
			snapDir(t, snap, base, w, label, inputs)
		} else {
			snapKey(t, snap, base, w, label)
		}
	}

	snapFirewall(t, snap, label)

	return snap
}

func snapDir(t *testing.T, snap, base snapshot, place watch, label func(string) string, inputs map[string][]byte) {
	t.Helper()

	entries, err := os.ReadDir(place.dir)
	if err != nil {
		return // The folder does not exist on this machine.
	}

	for _, entry := range entries {
		if place.skip != nil && place.skip.MatchString(entry.Name()) {
			continue
		}

		name := place.label + `\` + entry.Name()
		kind := "file "

		if entry.IsDir() {
			kind = "dir "
		}

		if base == nil || hasEntry(base, kind+name) {
			snap[kind+name] = ""

			continue
		}

		root := filepath.Join(place.dir, entry.Name())

		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			rel, _ := filepath.Rel(place.dir, path)
			if entry.IsDir() {
				snap["dir "+place.label+`\`+rel] = ""

				return nil
			}

			snap["file "+place.label+`\`+rel] = fileContent(t, path, place.hashes, label, inputs)

			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
}

func hasEntry(snap snapshot, name string) bool {
	_, ok := snap[name]

	return ok
}

// fileContent describes a file: the input it equals, the target of a
// shortcut, or its size and hash.
func fileContent(t *testing.T, path string, hashes bool, label func(string) string, inputs map[string][]byte) string {
	t.Helper()

	if strings.EqualFold(filepath.Ext(path), ".lnk") {
		// A shortcut holds machine IDs and times; only what it points to
		// is stable.
		return "shortcut " + label(lnkTarget(t, path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	for _, name := range slices.Sorted(maps.Keys(inputs)) {
		if bytes.Equal(data, inputs[name]) {
			return "input " + name
		}
	}

	if !hashes {
		return ""
	}

	sum := sha256.Sum256(data)

	return fmt.Sprintf("%d bytes sha256:%s", len(data), hex.EncodeToString(sum[:8]))
}

func snapKey(t *testing.T, snap, base snapshot, place watch, label func(string) string) {
	t.Helper()

	key, err := openKey(place.root, place.key)
	if err != nil {
		return // The key does not exist on this machine.
	}
	defer syscall.RegCloseKey(key) //nolint:errcheck // read-only handle

	for _, child := range subkeys(t, key) {
		name := "key " + place.label + `\` + child
		if base == nil || hasEntry(base, name) {
			snap[name] = ""

			continue
		}

		snapTree(t, snap, place.root, place.key+`\`+child, place.label+`\`+child, label)
	}
}

// snapTree records a key, its values and its subkeys.
func snapTree(t *testing.T, snap snapshot, root syscall.Handle, path, name string, label func(string) string) {
	t.Helper()

	key, err := openKey(root, path)
	if err != nil {
		snap["key "+name] = "<" + err.Error() + ">"

		return
	}
	defer syscall.RegCloseKey(key) //nolint:errcheck // read-only handle

	snap["key "+name] = ""

	for value, content := range values(t, key) {
		if masked, ok := volatileValues[value]; ok {
			content = masked
		}

		snap["value "+name+` [`+value+`]`] = label(content)
	}

	for _, child := range subkeys(t, key) {
		snapTree(t, snap, root, path+`\`+child, name+`\`+child, label)
	}
}

// snapFirewall records the firewall rules by their names; the value names
// that netsh picks are random.
func snapFirewall(t *testing.T, snap snapshot, label func(string) string) {
	t.Helper()

	key, err := openKey(syscall.HKEY_LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\SharedAccess\Parameters\FirewallPolicy\FirewallRules`)
	if err != nil {
		return
	}
	defer syscall.RegCloseKey(key) //nolint:errcheck // read-only handle

	rules := map[string][]string{}

	for _, rule := range values(t, key) {
		rule = strings.TrimSuffix(strings.TrimPrefix(rule, `"`), `"`)

		rule = label(firewallVersion.ReplaceAllString(rule, ""))
		name := "?"

		for field := range strings.SplitSeq(rule, "|") {
			if after, ok := strings.CutPrefix(field, "Name="); ok {
				name = after
			}
		}

		rules[name] = append(rules[name], rule)
	}

	for name, list := range rules {
		slices.Sort(list)
		snap["firewall "+name] = strings.Join(list, " AND ")
	}
}

func openKey(root syscall.Handle, path string) (syscall.Handle, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var key syscall.Handle

	err = syscall.RegOpenKeyEx(root, name, 0, syscall.KEY_READ, &key)

	return key, err
}

type keyInfo struct {
	subkeys, maxSubkey, values, maxValueName, maxValue uint32
}

func queryKey(t *testing.T, key syscall.Handle) keyInfo {
	t.Helper()

	var info keyInfo
	if err := syscall.RegQueryInfoKey(key, nil, nil, nil, &info.subkeys, &info.maxSubkey, nil,
		&info.values, &info.maxValueName, &info.maxValue, nil, nil); err != nil {
		t.Fatalf("RegQueryInfoKey: %v", err)
	}

	return info
}

// errMoreData and errNoMoreItems end or restart an enumeration whose key
// changed after RegQueryInfoKey.
const (
	errMoreData    = syscall.Errno(234)
	errNoMoreItems = syscall.Errno(259)
)

func subkeys(t *testing.T, key syscall.Handle) []string {
	t.Helper()

	info := queryKey(t, key)
	names := make([]string, 0, info.subkeys)
	capacity := info.maxSubkey + 1

	for index := uint32(0); ; {
		size := capacity
		buf := make([]uint16, size)

		err := syscall.RegEnumKeyEx(key, index, &buf[0], &size, nil, nil, nil, nil)

		switch {
		case errors.Is(err, errNoMoreItems):
			return names
		case errors.Is(err, errMoreData):
			// A longer name appeared since the query; retry with room for it.
			capacity *= 2

			continue
		case err != nil:
			t.Fatalf("RegEnumKeyEx: %v", err)
		}

		names = append(names, syscall.UTF16ToString(buf[:size]))
		index++
	}
}

// values returns the values of a key by name, formatted with their types.
// The default value is named "".
func values(t *testing.T, key syscall.Handle) map[string]string {
	t.Helper()

	info := queryKey(t, key)
	out := make(map[string]string, info.values)
	nameCap, dataCap := info.maxValueName+1, info.maxValue+1

	for index := uint32(0); ; {
		nameLen, dataLen := nameCap, dataCap
		name := make([]uint16, nameLen)
		data := make([]byte, dataLen)

		var typ uint32

		ret, _, _ := pRegEnumValueW.Call(uintptr(key), uintptr(index),
			uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&nameLen)), 0,
			uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&dataLen)))

		switch err := syscall.Errno(ret); {
		case err == errNoMoreItems:
			return out
		case err == errMoreData:
			// The value grew since the query; retry with room for it.
			nameCap, dataCap = 2*nameCap, max(2*dataCap, dataLen)

			continue
		case err != 0:
			t.Fatalf("RegEnumValueW: %v", err)
		}

		out[syscall.UTF16ToString(name[:nameLen])] = formatValue(typ, data[:dataLen])
		index++
	}
}

func formatValue(typ uint32, data []byte) string {
	utf := func() []uint16 {
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(data[2*i:])
		}

		return units
	}

	// Strings are quoted without escapes, so that paths stay readable and
	// pathLabels finds them.
	quote := func(s string) string { return `"` + s + `"` }

	switch typ {
	case syscall.REG_SZ:
		return quote(syscall.UTF16ToString(utf()))
	case syscall.REG_EXPAND_SZ:
		return "expand " + quote(syscall.UTF16ToString(utf()))
	case syscall.REG_MULTI_SZ:
		var parts []string

		for part := range strings.SplitSeq(strings.TrimRight(string(utf16.Decode(utf())), "\x00"), "\x00") {
			parts = append(parts, quote(part))
		}

		return "multi [" + strings.Join(parts, ", ") + "]"
	case syscall.REG_DWORD:
		if len(data) == 4 {
			return fmt.Sprintf("dword %d", binary.LittleEndian.Uint32(data))
		}
	case syscall.REG_QWORD:
		if len(data) == 8 {
			return fmt.Sprintf("qword %d", binary.LittleEndian.Uint64(data))
		}
	}

	return fmt.Sprintf("type %d %s", typ, hex.EncodeToString(data))
}

// changes lists what differs between two snapshots, sorted by entry: "+"
// for an entry that is new, "-" for one that is gone and "~" for one whose
// content changed.
func changes(before, after snapshot) []string {
	var lines []string

	describe := func(sign, entry, content string) string {
		if content == "" {
			return sign + " " + entry
		}

		return sign + " " + entry + " = " + content
	}

	for _, entry := range slices.Sorted(maps.Keys(after)) {
		old, ok := before[entry]

		switch {
		case !ok:
			lines = append(lines, describe("+", entry, after[entry]))
		case old != after[entry]:
			lines = append(lines, describe("~", entry, old+" -> "+after[entry]))
		}
	}

	for _, entry := range slices.Sorted(maps.Keys(before)) {
		if _, ok := after[entry]; !ok {
			lines = append(lines, describe("-", entry, before[entry]))
		}
	}

	slices.SortStableFunc(lines, func(a, b string) int { return strings.Compare(a[2:], b[2:]) })

	return lines
}

// saveSnapshot writes a snapshot that differs from testdata into
// GO_MSI_SNAPSHOT_DIR, which CI uploads, so that it can be committed
// without an elevated Windows machine at hand.
func saveSnapshot(t *testing.T, name, report string) {
	t.Helper()

	dir := os.Getenv("GO_MSI_SNAPSHOT_DIR")
	if dir == "" {
		t.Logf("new snapshot of %s (set GO_MSI_SNAPSHOT_DIR to save it as a file):\n%s", name, report)

		return
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, name+".txt")
	if err := os.WriteFile(file, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Logf("new snapshot of %s saved as %s", name, file)
}

// snapshotReport is the snapshot file of one example.
func snapshotReport(name string, install, uninstall []string) string {
	var out strings.Builder

	fmt.Fprintf(&out, "# What msiexec /i and then /x of examples/%s change on the machine.\n", name)
	out.WriteString("# Written by TestMsiexecInstallsExamples with GO_MSI_UPDATE_SNAPSHOTS=1.\n")
	out.WriteString("\n[install]\n")

	for _, line := range install {
		out.WriteString(line + "\n")
	}

	out.WriteString("\n[uninstall]\n")

	for _, line := range uninstall {
		out.WriteString(line + "\n")
	}

	return out.String()
}
