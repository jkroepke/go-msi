package config

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SDDL checks for MsiLockPermissionsEx.SDDLText (spec section 6.2, SEC-01):
// the engine does not expand [Property] references and fails with 1943 on
// anything it cannot parse, so the syntax is checked at build time after
// each <account> form, which the engine resolves, is replaced by a
// placeholder SID.

var (
	numericSID = regexp.MustCompile(`^S-1-\d+(-\d+)+$`)
	envRef     = regexp.MustCompile(`\[%[A-Za-z_][A-Za-z0-9_]*\]`)
	accountRef = regexp.MustCompile(`<[^<>]*>`)
	hexRights  = regexp.MustCompile(`^(0[xX][0-9A-Fa-f]+|\d+)$`)
	guid       = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)
)

// sidAliases are the two-letter SID strings of the SDDL grammar.
var sidAliases = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"AA": true, "AC": true, "AN": true, "AO": true, "AP": true, "AS": true, "AU": true, "BA": true, "BG": true, "BO": true, "BU": true,
	"CA": true, "CD": true, "CG": true, "CN": true, "CO": true, "CY": true, "DA": true, "DC": true, "DD": true, "DG": true, "DU": true,
	"EA": true, "ED": true, "EK": true, "ER": true, "ES": true, "HA": true, "HI": true, "IS": true, "IU": true, "KA": true, "LA": true,
	"LG": true, "LS": true, "LU": true, "LW": true, "ME": true, "MP": true, "MS": true, "MU": true, "NO": true, "NS": true, "NU": true,
	"OW": true, "PA": true, "PO": true, "PS": true, "PU": true, "RA": true, "RC": true, "RD": true, "RE": true, "RM": true, "RO": true,
	"RS": true, "RU": true, "SA": true, "SI": true, "SO": true, "SS": true, "SU": true, "SY": true, "UD": true, "WD": true, "WR": true,
}

var aceTypes = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"A": true, "D": true, "OA": true, "OD": true, "AU": true, "AL": true, "OU": true, "OL": true, "ML": true,
	"XA": true, "XD": true, "RA": true, "SP": true, "XU": true, "ZA": true, "TL": true, "FL": true,
}

var aceFlags = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"CI": true, "OI": true, "NP": true, "IO": true, "ID": true, "SA": true, "FA": true, "TP": true, "CR": true,
}

var aceRights = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"GA": true, "GR": true, "GW": true, "GX": true, "RC": true, "SD": true, "WD": true, "WO": true, "RP": true, "WP": true, "CC": true,
	"DC": true, "LC": true, "SW": true, "LO": true, "DT": true, "CR": true, "FA": true, "FR": true, "FW": true, "FX": true, "KA": true,
	"KR": true, "KW": true, "KX": true, "NR": true, "NW": true, "NX": true,
}

var aclFlags = []string{"NO_ACCESS_CONTROL", "AR", "AI", "P"} //nolint:gochecknoglobals // lookup table

const (
	placeholderSID = "BA"
	minACEFields   = 6
	maxACEFields   = 7
	// maxSubAuthorities is SID_MAX_SUB_AUTHORITIES.
	maxSubAuthorities = 15
	denyTypes         = "D OD XD"
)

// CheckSDDL validates an SDDL string the way the engine will see it. It
// returns warnings about ACEs that can lock the installer itself out
// (SEC-01: a deny ACE for a broad group leaves the file behind on uninstall)
// and an error for anything the engine would reject with 1943.
func CheckSDDL(sddl string) ([]string, error) {
	if sddl == "" {
		return nil, errors.New("sddl is empty")
	}

	plain, err := resolveAccounts(sddl)
	if err != nil {
		return nil, err
	}

	parts, err := splitSDDL(plain)
	if err != nil {
		return nil, err
	}

	var warnings []string

	for _, part := range parts {
		if part.kind == 'O' || part.kind == 'G' {
			if !validSID(part.body) {
				return nil, fmt.Errorf("%c: %q is not a SID", part.kind, part.body)
			}

			continue
		}

		aclWarnings, err := checkACL(part.kind, part.body)
		if err != nil {
			return nil, err
		}

		warnings = append(warnings, aclWarnings...)
	}

	return warnings, nil
}

// resolveAccounts replaces the <DOMAIN\account> forms, which the engine
// resolves, by a placeholder SID. Inside them only [%ENV] references are
// allowed; everything else in brackets stays literal and fails at install
// time.
func resolveAccounts(sddl string) (string, error) {
	plain := accountRef.ReplaceAllStringFunc(sddl, func(account string) string {
		inner := envRef.ReplaceAllString(account[1:len(account)-1], "x")
		if strings.ContainsAny(inner, "[]") || strings.TrimSpace(inner) == "" {
			return "<bad>"
		}

		return placeholderSID
	})

	if strings.Contains(plain, "<bad>") {
		return "", errors.New("an <account> form is empty or holds a [Property] reference; only [%ENV] references are expanded there")
	}

	if strings.ContainsAny(plain, "[]<>") {
		return "", errors.New("[Property] references are not expanded in SDDL (error 1943); use an <account> form")
	}

	return plain, nil
}

type sddlPart struct {
	body string
	kind byte
}

// splitSDDL cuts the string at the O:, G:, D: and S: markers.
// Each may appear once: Windows refuses "D:(…)D:(…)".
func splitSDDL(sddl string) ([]sddlPart, error) {
	var parts []sddlPart

	for sddl != "" {
		if len(sddl) < 2 || sddl[1] != ':' || !strings.ContainsRune("OGDS", rune(sddl[0])) {
			return nil, fmt.Errorf("expected O:, G:, D: or S: at %q", sddl)
		}

		for _, seen := range parts {
			if seen.kind == sddl[0] {
				return nil, fmt.Errorf("%c: appears twice", sddl[0])
			}
		}

		part := sddlPart{kind: sddl[0]}
		sddl = sddl[2:]

		end := nextMarker(sddl, part.kind)
		part.body = sddl[:end]
		sddl = sddl[end:]

		if part.kind != 'D' && part.kind != 'S' && part.body == "" {
			return nil, fmt.Errorf("%c: has no SID", part.kind)
		}

		parts = append(parts, part)
	}

	return parts, nil
}

// nextMarker finds the start of the next component. In an ACL a marker only
// counts after a closing parenthesis, since ACE rights can spell "SD".
func nextMarker(sddl string, kind byte) int {
	acl := kind == 'D' || kind == 'S'

	for i := 0; i+1 < len(sddl); i++ {
		if sddl[i+1] != ':' || !strings.ContainsRune("OGDS", rune(sddl[i])) {
			continue
		}

		if !acl || i == 0 || sddl[i-1] == ')' {
			return i
		}
	}

	return len(sddl)
}

// validSID accepts an alias or S-1-authority-sub..., with a 48-bit
// authority and 1 to 15 32-bit sub-authorities (SID_MAX_SUB_AUTHORITIES).
func validSID(sid string) bool {
	if sidAliases[sid] {
		return true
	}

	if !numericSID.MatchString(sid) {
		return false
	}

	parts := strings.Split(sid, "-")[2:]
	if len(parts)-1 > maxSubAuthorities {
		return false
	}

	if _, err := strconv.ParseUint(parts[0], 10, 48); err != nil {
		return false
	}

	for _, sub := range parts[1:] {
		if _, err := strconv.ParseUint(sub, 10, 32); err != nil {
			return false
		}
	}

	return true
}

// checkACL validates the flags and ACEs of a DACL or SACL.
func checkACL(kind byte, body string) ([]string, error) {
	flags, aces, _ := strings.Cut(body, "(")
	if aces != "" {
		aces = "(" + aces
	}

	if err := checkACLFlags(flags); err != nil {
		return nil, fmt.Errorf("%c: %w", kind, err)
	}

	var warnings []string

	for aces != "" {
		if aces[0] != '(' {
			return nil, fmt.Errorf("%c: expected an ACE at %q", kind, aces)
		}

		end := strings.IndexByte(aces, ')')
		if end < 0 {
			return nil, fmt.Errorf("%c: unterminated ACE %q", kind, aces)
		}

		warning, err := checkACE(aces[1:end])
		if err != nil {
			return nil, fmt.Errorf("%c: ACE %q: %w", kind, aces[:end+1], err)
		}

		if warning != "" {
			warnings = append(warnings, warning)
		}

		aces = aces[end+1:]
	}

	return warnings, nil
}

func checkACLFlags(flags string) error {
	for flags != "" {
		matched := false

		for _, flag := range aclFlags {
			if strings.HasPrefix(flags, flag) {
				flags = flags[len(flag):]
				matched = true

				break
			}
		}

		if !matched {
			return fmt.Errorf("unknown ACL flags %q", flags)
		}
	}

	return nil
}

// checkACE validates one ace_type;ace_flags;rights;object_guid;inherit_object_guid;account_sid[;resource] entry.
func checkACE(ace string) (string, error) {
	fields := strings.Split(ace, ";")
	if len(fields) < minACEFields || len(fields) > maxACEFields {
		return "", fmt.Errorf("%d fields, want %d", len(fields), minACEFields)
	}

	if !aceTypes[fields[0]] {
		return "", fmt.Errorf("unknown ACE type %q", fields[0])
	}

	if err := pairs(fields[1], aceFlags); err != nil {
		return "", fmt.Errorf("flags: %w", err)
	}

	if !hexRights.MatchString(fields[2]) {
		if err := pairs(fields[2], aceRights); err != nil {
			return "", fmt.Errorf("rights: %w", err)
		}
	}

	if err := checkACETail(fields); err != nil {
		return "", err
	}

	if strings.Contains(denyTypes, fields[0]) {
		return fmt.Sprintf("the deny ACE for %s can lock the installer out of the object; SEC-01 saw uninstall leave a file behind", fields[5]), nil
	}

	return "", nil
}

// checkACETail checks the object GUIDs and the account SID, and refuses a
// seventh field: Windows refuses it on access ACEs (A, D, ...).
func checkACETail(fields []string) error {
	for _, field := range fields[3:5] {
		if field != "" && !guid.MatchString(field) {
			return fmt.Errorf("%q is not a GUID", field)
		}
	}

	if !validSID(fields[5]) {
		return fmt.Errorf("%q is not a SID", fields[5])
	}

	if len(fields) == maxACEFields {
		// A resource attribute or a callback condition: both are
		// parenthesised values this parser does not read.
		return fmt.Errorf("the field %q after the SID (a resource attribute or condition) is not supported", fields[6])
	}

	return nil
}

// pairs checks that s is a sequence of two-letter codes from the set.
func pairs(codes string, set map[string]bool) error {
	if len(codes)%2 != 0 {
		return fmt.Errorf("%q is not a list of two-letter codes", codes)
	}

	for i := 0; i < len(codes); i += 2 {
		if !set[codes[i:i+2]] {
			return fmt.Errorf("unknown code %q", codes[i:i+2])
		}
	}

	return nil
}
