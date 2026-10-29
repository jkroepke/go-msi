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
// BuilderGrantExec grants without impersonation (as LocalSystem in a per-machine install), its rollback
// takes away what it added and the commit action drops the record of that.
// They run after the objects exist and before StartServices. An account
// from a property is remembered for maintenance installs, and the removal
// of the product forgets it; the objects the package created go with it.
func (b *build) grantActions() {
	if len(b.grants) == 0 || !b.useRunDLL() {
		return
	}

	for _, name := range []string{cadll.GrantProperty, cadll.GrantExec, cadll.GrantRollback, cadll.GrantCommit} {
		if _, ok := b.cfg.Properties[name]; ok {
			b.errorf("properties."+name, "the property %s is reserved for permission grants", name)

			return
		}
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
	b.grantSchedule(anchor)
}

// grantSchedule adds the grant actions after anchor.
func (b *build) grantSchedule(anchor string) {
	actions := []*actionInfo{
		{name: cadll.GrantSchedule, typ: caDLL, source: cadll.Key, target: cadll.GrantSchedule},
		{name: cadll.GrantRollback, typ: caDLL | caRollback | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
		{name: cadll.GrantExec, typ: caDLL | caDeferred | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
		{name: cadll.GrantCommit, typ: caDLL | caCommit | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
	}

	// The schedule action runs on removal too, where it only hands the
	// exec action the forgetting of the remembered accounts; each deferred
	// action runs when it has data.
	for _, action := range actions {
		b.addAction(action)

		cond := action.name
		if action.name == cadll.GrantSchedule {
			cond = ""
		}

		b.schedule(seqInstallExecute, action.name, cond, "", anchor, 0)
	}
}
