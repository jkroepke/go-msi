package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Access rights of the grant names. The file names are the simple rights of
// a file or folder (full, modify, read and execute, read, write); the
// service names combine the service access rights.
var (
	fileRights = map[string]uint32{ //nolint:gochecknoglobals // lookup table
		"full":        0x1F01FF,
		"modify":      0x1301BF,
		"readExecute": 0x1200A9,
		"read":        0x120089,
		"write":       0x100116,
	}
	serviceRights = map[string]uint32{ //nolint:gochecknoglobals // lookup table
		"full":          0xF01FF,
		"read":          0x2018D,
		"start":         0x10,
		"stop":          0x20,
		"pauseContinue": 0x40,
	}
)

// GrantMask returns the access mask of rights for a file or folder, or for a
// service. A right is a name of its kind or a hexadecimal mask (0x...).
func GrantMask(rights []string, service bool) (uint32, error) {
	names := fileRights
	if service {
		names = serviceRights
	}

	var mask uint32

	for _, right := range rights {
		if hex, ok := strings.CutPrefix(strings.ToLower(right), "0x"); ok {
			value, err := strconv.ParseUint(hex, 16, 32)
			if err != nil || value == 0 {
				return 0, fmt.Errorf("right %q is not a non-zero 32-bit hexadecimal mask", right)
			}

			mask |= uint32(value)

			continue
		}

		value, ok := names[right]
		if !ok {
			return 0, fmt.Errorf("unknown right %q; use %s or a hexadecimal mask (0x...)", right, strings.Join(slices.Sorted(maps.Keys(names)), ", "))
		}

		mask |= value
	}

	if mask == 0 {
		return 0, errors.New("rights is empty")
	}

	return mask, nil
}

// Kinds of objects a permissions block secures.
const (
	permFile     = "file"
	permFolder   = "folder"
	permRegistry = "registry"
	permService  = "service"
)

// grants validates the grant list of a permissions block on an object of
// the kind.
func (v *validator) grants(field string, perm *Permissions, kind string) {
	if len(perm.Grant) > 0 && kind == permRegistry {
		v.errorf(field+".grant", "grant is supported on files, folders and services; secure a registry key with sddl")

		return
	}

	for i, grant := range perm.Grant {
		path := fmt.Sprintf("%s.grant[%d]", field, i)
		if grant == nil {
			v.errorf(path, "a grant needs account and rights")

			continue
		}

		if strings.TrimSpace(grant.Account) == "" {
			v.errorf(path+".account", "account is empty")
		} else if strings.Contains(grant.Account, "|") {
			v.errorf(path+".account", "an account name cannot contain |")
		}

		if _, err := GrantMask(grant.Rights, kind == permService); err != nil {
			v.errorf(path+".rights", "%v", err)
		}

		if grant.Inherit != nil && kind != permFolder {
			v.errorf(path+".inherit", "inherit applies to folders only")
		}
	}
}
