package config

import (
	"fmt"
	"strings"

	"github.com/jkroepke/msi-builder/internal/rights"
)

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
	if len(perm.Grant) > 0 && strings.ContainsFunc(perm.Condition, func(r rune) bool { return r < 0x20 }) {
		v.errorf(field+".condition", "the condition of a block with grants cannot contain line breaks or control characters")
	}

	if len(perm.Grant) > 0 && kind == permRegistry {
		v.errorf(field+".grant", "grant is supported on files, folders and services; secure a registry key with sddl")

		return
	}

	for i, grant := range perm.Grant {
		v.grant(fmt.Sprintf("%s.grant[%d]", field, i), grant, kind)
	}
}

// grant validates one grant on an object of the kind.
func (v *validator) grant(path string, grant *Grant, kind string) {
	if grant == nil {
		v.errorf(path, "a grant needs account and rights")

		return
	}

	switch {
	case strings.TrimSpace(grant.Account) == "":
		v.errorf(path+".account", "account is empty")
	case strings.ContainsFunc(grant.Account, func(r rune) bool { return r == '|' || r < 0x20 }):
		v.errorf(path+".account", "an account name cannot contain | or control characters")
	}

	if _, err := rights.Mask(grant.Rights, kind == permService); err != nil {
		v.errorf(path+".rights", "%v", err)
	}

	if grant.Inherit != nil && kind != permFolder {
		v.errorf(path+".inherit", "inherit applies to folders only")
	}
}
