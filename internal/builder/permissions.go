package builder

import (
	"strings"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/internal/rights"
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

// grantActions writes the grants into the BuilderGrants property, one line
// per grant with its condition (the component is installed or repaired, and
// the block's condition), and adds the actions on the embedded DLL that
// apply them (specification section 17.7): BuilderGrantSchedule evaluates the
// conditions and formats accounts and objects, then the deferred
// BuilderGrantExec grants as the installer (not impersonated), its rollback
// takes away what it added and the commit action drops the record of that.
// They run after the objects exist and before StartServices. Nothing runs
// on removal: the objects the package created go with it.
func (b *build) grantActions() {
	if len(b.grants) == 0 || !b.useRunDLL() {
		return
	}

	if _, ok := b.cfg.Properties[cadll.GrantProperty]; ok {
		b.errorf("properties."+cadll.GrantProperty, "the property %s is reserved for permission grants", cadll.GrantProperty)

		return
	}

	lines := make([]string, 0, len(b.grants))

	for _, grant := range b.grants {
		mask, err := rights.Mask(grant.cfg.Rights, grant.kind == cadll.GrantService)
		if err != nil {
			continue // reported by validation
		}

		cond := "$" + grant.component + "=3"
		if grant.condition != "" {
			cond += " AND (" + grant.condition + ")"
		}

		lines = append(lines, cadll.GrantEntry(cond, grant.kind, mask, grant.flags, grant.cfg.Account, grant.object))
	}

	b.properties[cadll.GrantProperty] = strings.Join(lines, "\n")

	anchor := actInstallFiles
	if b.hasRows("ServiceInstall") {
		anchor = actInstallServices
	}

	b.standardAt(seqInstallExecute, anchor)

	actions := []*actionInfo{
		{name: cadll.GrantSchedule, typ: caDLL, source: cadll.Key, target: cadll.GrantSchedule},
		{name: cadll.GrantRollback, typ: caDLL | caRollback | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
		{name: cadll.GrantExec, typ: caDLL | caDeferred | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
		{name: cadll.GrantCommit, typ: caDLL | caCommit | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
	}

	for _, action := range actions {
		b.addAction(action)
		b.schedule(seqInstallExecute, action.name, `NOT REMOVE~="ALL"`, "", anchor, 0)
	}
}
