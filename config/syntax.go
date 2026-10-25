package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	guidPattern       = regexp.MustCompile(`^\{?[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}?$`)
	identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)
	columnTypePattern = regexp.MustCompile(`^([sSlL])(\d+)$|^([iI])([24])$|^([vV])0$`)
)

// NilGUID is the all-zero GUID in the form NormalizeGUID returns.
const NilGUID = "{00000000-0000-0000-0000-000000000000}"

// NormalizeGUID returns s braced and upper case, or an error if s is not a GUID.
func NormalizeGUID(s string) (string, error) {
	if !guidPattern.MatchString(s) || strings.HasPrefix(s, "{") != strings.HasSuffix(s, "}") {
		return "", fmt.Errorf("%q is not a GUID", s)
	}

	return "{" + strings.ToUpper(strings.Trim(s, "{}")) + "}", nil
}

// Version is a ProductVersion: three fields and an optional fourth.
type Version struct {
	Major, Minor, Build, Revision int
	Fields                        int
}

// ParseVersion parses major.minor.build[.revision] within the limits of
// ProductVersion (255.255.65535, revision 65535).
func ParseVersion(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return Version{}, fmt.Errorf("version %q must have two to four fields", s)
	}

	limits := []int{255, 255, 65535, 65535}
	v := Version{Fields: len(parts)}
	dst := []*int{&v.Major, &v.Minor, &v.Build, &v.Revision}

	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || (len(p) > 1 && p[0] == '+') {
			return Version{}, fmt.Errorf("version %q: field %d is not a number", s, i+1)
		}

		if n > limits[i] {
			return Version{}, fmt.Errorf("version %q: field %d exceeds %d", s, i+1, limits[i])
		}

		*dst[i] = n
	}

	return v, nil
}

// PageCount converts installerVersion ("5.0", "4.5") into the Summary
// Information Page Count (major × 100 + minor).
func PageCount(installerVersion string) (int, error) {
	major, minor, ok := strings.Cut(installerVersion, ".")
	if !ok {
		return 0, fmt.Errorf("installer version %q must be major.minor", installerVersion)
	}

	ma, err1 := strconv.Atoi(major)
	mi, err2 := strconv.Atoi(minor)

	if err1 != nil || err2 != nil || ma < 0 || mi < 0 || mi > 99 {
		return 0, fmt.Errorf("installer version %q must be major.minor", installerVersion)
	}

	return ma*100 + mi, nil
}

// MinPageCount is the lowest Page Count go-msi writes for a platform. x64
// installed from 150 up in SUM-03, but 200 is kept until ICE80 has been run.
func MinPageCount(p Platform) int {
	switch p {
	case PlatformX64:
		return 200
	case PlatformArm64:
		return 500
	default:
		return 100
	}
}

// MaxPageCount is the highest Page Count Windows Installer 5.0 accepts
// (SUM-03: 501 and above fail with 1613 on every platform).
const MaxPageCount = 500

// IsPublicProperty reports whether name has no lower-case letter.
func IsPublicProperty(name string) bool {
	return strings.ToUpper(name) == name
}

// IsIdentifier reports whether s is a valid Windows Installer identifier.
func IsIdentifier(s string) bool {
	return identifierPattern.MatchString(s)
}

// ansiCodepages may be stored in the Summary Information stream (SUM-04).
var ansiCodepages = map[int]bool{
	874: true, 932: true, 936: true, 949: true, 950: true,
	1250: true, 1251: true, 1252: true, 1253: true, 1254: true,
	1255: true, 1256: true, 1257: true, 1258: true,
}

// SummaryCodepage returns the code page written to the Summary Information
// stream for a database code page: the same page when it is ANSI, else 1252.
// Summary code page 65001 or 1200 makes msiexec fail with 1620 (SUM-04).
func SummaryCodepage(databaseCodepage int) int {
	if ansiCodepages[databaseCodepage] {
		return databaseCodepage
	}

	return 1252
}

const invalidLongNameChars = `\/:*?"<>|`

// validLongName checks a long file or directory name.
func validLongName(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, invalidLongNameChars) &&
		strings.TrimRight(s, ". ") == s
}

// validShortName checks an 8.3 name (specification section 11.3).
func validShortName(s string) bool {
	if strings.ContainsAny(s, `\/:*?"<>|+,;=[] `) {
		return false
	}

	base, ext, hasExt := strings.Cut(s, ".")
	if hasExt && (ext == "" || len(ext) > 3 || strings.Contains(ext, ".")) {
		return false
	}

	for _, r := range s {
		if r < 0x21 || r > 0x7E {
			return false
		}
	}

	return base != "" && len(base) <= 8
}

// validName checks "long" or "short|long".
func validName(s string) bool {
	short, long, ok := strings.Cut(s, "|")
	if !ok {
		return validLongName(s)
	}

	return validShortName(short) && validLongName(long)
}
