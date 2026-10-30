package builder

import (
	"slices"
	"sort"
	"strconv"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/internal/rights"
	"github.com/jkroepke/msi-builder/internal/sequence"
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
		if !info.createFolder {
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

// grantActions adds the actions on the embedded DLL that apply the grants
// (specification section 17.7): the deferred BuilderGrantExec grants
// without impersonation (as LocalSystem in a per-machine install), its
// rollback takes away what it added and the commit action drops the record
// of that. They run after the objects exist and before StartServices. An
// account from a property is remembered for maintenance installs, and the
// removal of the product forgets it; the objects the package created go
// with it. Their data comes from type-51 actions the engine runs, never
// from an action that runs as the installing user, who could change it.
func (b *build) grantActions() {
	if len(b.grants) == 0 || !b.useRunDLL() {
		return
	}

	for _, name := range []string{cadll.GrantData, cadll.GrantCount, cadll.GrantExec, cadll.GrantRollback, cadll.GrantCommit} {
		if _, ok := b.cfg.Properties[name]; ok {
			b.errorf("properties."+name, "the property %s is reserved for permission grants", name)

			return
		}
	}

	anchor := actInstallFiles
	if b.hasRows("ServiceInstall") {
		anchor = actInstallServices
	}

	b.grantAnchor = anchor
	b.standardAt(seqInstallExecute, anchor)
	b.grantSchedule(anchor)
	b.grantData()
}

// grantData builds the data of the deferred grant actions in BuilderGrantData
// with type-51 actions right before them, so that no action that runs as
// the user comes between: two resets, one of which runs, and a reset of
// BuilderGrantCount, so that nothing set before (on the command line or by
// an action) survives into the data, then two actions per grant,
// conditioned on its component being installed or repaired and the block's
// condition, one that appends the grant with its account and object
// formatted by the engine and one that counts it, then the data of each
// deferred action. The count lets the DLL find a line break that a
// formatted value added. The head line ends with a marker in a per-user
// install, whose accounts are remembered in that user's hive. A
// removal only hands the commit action the forgetting of the remembered
// accounts, with that head line, so a failed removal keeps them.
func (b *build) grantData() {
	data := "[" + cadll.GrantCount + "][" + cadll.GrantData + "]"

	type setter struct{ name, property, cond, target string }

	setters := []setter{
		{"BuilderGrantReset", cadll.GrantData, condNotInstalled, string(cadll.GrantInstall) + "[ProductCode]"},
		{"BuilderGrantResetMaintenance", cadll.GrantData, "Installed", string(cadll.GrantMaintenance) + "[ProductCode]"},
		{"BuilderGrantResetCount", cadll.GrantCount, "", ":"},
		// A per-user install remembers the accounts in the hive of the
		// engine's UserSID, which the DLL reads itself.
		{"BuilderGrantUser", cadll.GrantData, condPerUser, "[" + cadll.GrantData + "]" + string(cadll.GrantUser)},
	}

	for i, grant := range b.grants {
		mask, err := rights.Mask(grant.cfg.Rights, grant.kind == cadll.GrantService)
		if err != nil {
			continue // reported by validation
		}

		cond := "$" + grant.component + "=3"
		if grant.condition != "" {
			cond += " AND (" + grant.condition + ")"
		}

		account := grant.cfg.Account
		entry := cadll.GrantEntry(grant.kind, mask, grant.flags, escapeFormatted(account), grant.object, account)
		id := generatedID("", strconv.Itoa(i), grant.object, account)
		b.grantSetters = append(b.grantSetters, "BuilderGrantSet"+id, "BuilderGrantCount"+id)
		setters = append(setters,
			setter{"BuilderGrantSet" + id, cadll.GrantData, cond, "[" + cadll.GrantData + "]\n" + entry},
			setter{"BuilderGrantCount" + id, cadll.GrantCount, cond, "[" + cadll.GrantCount + "]#"},
		)
	}

	setters = append(setters,
		setter{"BuilderGrantSetExec", cadll.GrantExec, "", "+" + data},
		setter{"BuilderGrantSetRollback", cadll.GrantRollback, "", "-" + data},
		setter{"BuilderGrantSetCommit", cadll.GrantCommit, "", "c" + data},
		setter{"BuilderGrantForget", cadll.GrantCommit, condRemoveAll, string(cadll.GrantForget) + "[" + cadll.GrantData + "]"},
	)

	for _, s := range setters {
		b.addAction(&actionInfo{name: s.name, typ: caSetProperty, source: s.property, target: s.target})
		b.schedule(seqInstallExecute, s.name, s.cond, cadll.GrantRollback, "", 0)
	}
}

// checkGrantRoom reports grants whose actions do not fit between their
// anchor and the next numbered action, such as MsiConfigureServices (5850)
// after InstallServices, with the number that fits, before numbering fails
// with a less helpful error. It numbers the sequence with fewer grants
// to find how many fit; other actions around the anchor take numbers there too.
func (b *build) checkGrantRoom() {
	actions := b.schedules[seqInstallExecute]
	if len(b.grantSetters) == 0 {
		return
	}

	if _, err := sequence.Number(actions); err == nil {
		return
	}

	// Fewer grants never need more numbers, so the most that fit is found
	// by bisection.
	fits := func(count int) bool {
		dropped := make(map[string]bool, len(b.grantSetters))
		for _, name := range b.grantSetters[2*count:] {
			dropped[name] = true
		}

		_, err := sequence.Number(slices.DeleteFunc(slices.Clone(actions), func(a sequence.Action) bool { return dropped[a.Name] }))

		return err == nil
	}

	if !fits(0) {
		return // the numbering error is not the grants'
	}

	most := sort.Search(len(b.grantSetters)/2, func(count int) bool { return !fits(count + 1) })
	b.errorf("components", "%d permission grants do not fit: each takes two actions after %s, which the package's other "+
		"actions there share; at most %d fit in this package", len(b.grants), b.grantAnchor, most)
}

// grantSchedule adds the deferred grant actions after anchor. They do not
// run on removal; their data is set in every install, so none can come from
// elsewhere.
func (b *build) grantSchedule(anchor string) {
	actions := []*actionInfo{
		{name: cadll.GrantRollback, typ: caDLL | caRollback | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
		{name: cadll.GrantExec, typ: caDLL | caDeferred | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
		{name: cadll.GrantCommit, typ: caDLL | caCommit | caNoImpersonate, source: cadll.Key, target: cadll.GrantEntryPoint},
	}

	for _, action := range actions {
		b.addAction(action)

		cond := "NOT " + condRemoveAll
		if action.name == cadll.GrantCommit {
			cond = "" // forgets the remembered accounts on removal
		}

		b.schedule(seqInstallExecute, action.name, cond, "", anchor, 0)
	}
}
