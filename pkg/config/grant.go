package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jkroepke/msi-builder/internal/formatted"
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
	case formatted.Nested(grant.Account):
		// The builder keeps the properties of an account out of
		// SecureCustomProperties, which it cannot do for a property whose
		// name comes from another one (specification section 17.7).
		v.errorf(path+".account", "an account cannot name its property through another property ([A[B]] or [[B]])")
	}

	if _, err := rights.Mask(grant.Rights, kind == permService); err != nil {
		v.errorf(path+".rights", "%v", err)
	}

	if grant.Inherit != nil && kind != permFolder {
		v.errorf(path+".inherit", "inherit applies to folders only")
	}
}

// movableGrants warns about an inheriting folder grant on a folder that a
// property, an action or a dialog can point elsewhere at install time
// (INSTALLDIR=C:\Windows): the grant would reach the whole tree there, and
// a folder that existed before keeps the ACE after removal.
func (v *validator) movableGrants(cfg *Config, known *refs, compPath string, comp *Component) {
	perm := comp.FolderPermissions
	if perm == nil || known.dirs[comp.Directory] == nil {
		return
	}

	inherits := slices.ContainsFunc(perm.Grant, func(g *Grant) bool { return g != nil && (g.Inherit == nil || *g.Inherit) })
	if movable := movableAncestor(cfg, known, comp.Directory); inherits && movable != "" {
		v.warnf(compPath+".folderPermissions.grant", "%s can be pointed at any folder at install time (a public id, configurable, or set "+
			"by an action, a property or a dialog field), and an inheriting grant then reaches every file below that folder and stays "+
			"after removal; grant on a folder with private ids below a fixed standard folder, or set inherit: false", movable)
	}
}
