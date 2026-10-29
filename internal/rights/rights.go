// Package rights maps the access rights names of permission grants
// (docs/design/yaml-schema.md, "Grants") to access masks.
package rights

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
// service names are SERVICE_ALL_ACCESS, SERVICE_GENERIC_READ, SERVICE_START,
// SERVICE_STOP and SERVICE_PAUSE_CONTINUE.
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
		"read":          0x2008D,
		"start":         0x10,
		"stop":          0x20,
		"pauseContinue": 0x40,
	}
)

// unusable are the bits that are not rights of the object: GENERIC_*, which
// an explicit ACE on an existing object keeps unmapped, MAXIMUM_ALLOWED and
// ACCESS_SYSTEM_SECURITY.
const unusable = 0xF3000000

// Mask returns the access mask of rights for a file or folder, or for a
// service. A right is a name of its kind or a hexadecimal mask (0x...).
func Mask(rights []string, service bool) (uint32, error) {
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

			if value&unusable != 0 {
				return 0, fmt.Errorf("right %q has generic, MAXIMUM_ALLOWED or ACCESS_SYSTEM_SECURITY bits (0x%X), which grant nothing in an explicit ACE; "+
					"use the object's own rights", right, value&unusable)
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
