package builder

import (
	"strconv"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// Tables MsiLockPermissionsEx can secure (spec section 6.2).
const (
	lockFile      = "File"
	lockRegistry  = "Registry"
	lockFolder    = "CreateFolder"
	lockService   = "ServiceInstall"
	lockTableName = "MsiLockPermissionsEx"
)

// ACE inheritance flags of a folder grant: subfolders and files (OICI).
const grantInherit = 0x3

// grantInfo is one grant of a permissions block on an object the component
// installs.
type grantInfo struct {
	cfg       *config.Grant
	component string
	condition string
	object    string
	kind      byte
	flags     uint32
}

// permissions writes one MsiLockPermissionsEx row per secured object. The
// engine applies the SDDL when it creates the object and fails with 1943 on
// a bad string, so config.CheckSDDL has already validated every one. The
// grants of the blocks become custom actions on the embedded DLL.
func (b *build) permissions() error {
	for i, comp := range b.cfg.Components {
		b.lockComponent(comp.Path(i), comp, b.components[comp.ID])
	}

	for _, file := range b.files {
		if file.cfg.Permissions != nil {
			b.lock(lockFile, file.Key, file.cfg.Permissions)
			b.grant(file.Component.cfg.ID, "[#"+file.Key+"]", cadll.GrantFile, 0, file.cfg.Permissions)
		}
	}

	b.grantActions()

	return nil
}

func (b *build) lockComponent(path string, comp *config.Component, info *componentInfo) {
	if comp.FolderPermissions != nil {
		if info.keyPath != nil && !comp.CreateFolder {
			b.errorf(path+".folderPermissions", "the component creates no folder row; set createFolder: true")
		} else {
			b.lock(lockFolder, comp.Directory, comp.FolderPermissions)
			b.grant(comp.ID, "["+comp.Directory+"]", cadll.GrantFile, grantInherit, comp.FolderPermissions)
		}
	}

	for _, reg := range comp.Registry {
		if reg.Permissions != nil {
			b.lock(lockRegistry, b.registryID(info, reg), reg.Permissions)
		}
	}

	for _, svc := range comp.Services {
		if svc.Permissions != nil {
			b.lock(lockService, b.componentServiceID("svi", comp, svc.Name), svc.Permissions)
			b.grant(comp.ID, svc.Name, cadll.GrantService, 0, svc.Permissions)
		}
	}
}

func (b *build) lock(table, object string, perm *config.Permissions) {
	if perm.SDDL == "" {
		return
	}

	b.table(lockTableName).Add(generatedID("lpe", table, object), object, table, perm.SDDL, nullable(perm.Condition))
}

// grant records the grants of a block for grantActions. flags are the ACE
// flags of a folder grant that inherits.
func (b *build) grant(component, object string, kind byte, flags uint32, perm *config.Permissions) {
	for _, g := range perm.Grant {
		info := &grantInfo{cfg: g, component: component, condition: perm.Condition, object: object, kind: kind}
		if flags != 0 && (g.Inherit == nil || *g.Inherit) {
			info.flags = flags
		}

		b.grants = append(b.grants, info)
	}
}

// grantActions turns every grant into a deferred BuilderGrant action that
// runs as the installer (not impersonated) when the component is installed
// or repaired, after the object exists and before StartServices, with a
// rollback that takes the rights away again; a type-51 action before each
// formats its CustomActionData. Nothing runs on removal: the objects the
// package created go with it.
func (b *build) grantActions() {
	if len(b.grants) == 0 || !b.useRunDLL() {
		return
	}

	anchor := actInstallFiles
	if b.hasRows("ServiceInstall") {
		anchor = actInstallServices
	}

	b.standardAt(seqInstallExecute, anchor)

	for _, grant := range b.grants {
		mask, err := config.GrantMask(grant.cfg.Rights, grant.kind == cadll.GrantService)
		if err != nil {
			continue // reported by validation
		}

		name := generatedID("Grant", string(grant.kind), grant.object, grant.cfg.Account,
			strconv.FormatUint(uint64(mask), 16), strconv.FormatUint(uint64(grant.flags), 16))
		if b.action(name) != nil {
			continue // the same grant twice
		}

		cond := "$" + grant.component + "=3"
		if grant.condition != "" {
			cond += " AND (" + grant.condition + ")"
		}

		rollback := name + "Rollback"
		b.addAction(&actionInfo{name: name + ".Data", typ: caSetProperty, source: name,
			target: cadll.GrantData(false, grant.kind, mask, grant.flags, grant.cfg.Account, grant.object)})
		b.addAction(&actionInfo{name: rollback + ".Data", typ: caSetProperty, source: rollback,
			target: cadll.GrantData(true, grant.kind, mask, grant.flags, grant.cfg.Account, grant.object)})
		b.addAction(&actionInfo{name: rollback, typ: caDLL | caRollback | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntry})
		b.addAction(&actionInfo{name: name, typ: caDLL | caDeferred | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntry})

		for _, action := range []string{name + ".Data", rollback + ".Data", rollback, name} {
			b.schedule(seqInstallExecute, action, cond, "", anchor, 0)
		}
	}
}
