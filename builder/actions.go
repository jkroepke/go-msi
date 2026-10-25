package builder

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jkroepke/go-msi/condition"
	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/sequence"
)

// Custom action type words (section 9.4).
const (
	caDLL          = 1
	caError        = 19
	caExec         = 34
	caSetDirectory = 35
	caExecProperty = 50
	caSetProperty  = 51

	caContinue       = 0x40
	caAsyncWait      = 0x80
	caAsyncNoWait    = 0xC0
	caFirstSequence  = 0x100
	caOncePerProcess = 0x200
	caClientRepeat   = 0x300
	caDeferred       = 0x400
	caRollback       = 0x100 | caDeferred
	caCommit         = 0x200 | caDeferred
	caNoImpersonate  = 0x800
	caHideTarget     = 0x2000

	// maxTargetLength is the width of CustomAction.Target. The engine reads
	// longer values, but ICE03 reports them.
	maxTargetLength = 255
)

// Sequence table names.
const (
	seqInstallExecute = "InstallExecuteSequence"
	seqInstallUI      = "InstallUISequence"
	seqAdminExecute   = "AdminExecuteSequence"
	seqAdminUI        = "AdminUISequence"
	seqAdvtExecute    = "AdvtExecuteSequence"
)

var sequenceTables = []string{seqInstallExecute, seqInstallUI, seqAdminExecute, seqAdminUI, seqAdvtExecute}

// standardAction is a standard action's default number per table (section
// 9.2) and its condition.
type standardAction struct {
	condition string
	ies       int
	ius       int
	aes       int
	aus       int
	advt      int
}

func (s standardAction) number(table string) int {
	switch table {
	case seqInstallExecute:
		return s.ies
	case seqInstallUI:
		return s.ius
	case seqAdminExecute:
		return s.aes
	case seqAdminUI:
		return s.aus
	default:
		return s.advt
	}
}

// standardActions holds the default numbers of every standard action this
// builder may schedule.
var standardActions = map[string]standardAction{
	"FindRelatedProducts":      {ies: 25, ius: 25},
	"AppSearch":                {ies: 50, ius: 50},
	"LaunchConditions":         {ies: 100, ius: 100, aes: 100, aus: 100},
	"CCPSearch":                {ies: 500, ius: 500, condition: "NOT Installed"},
	"RMCCPSearch":              {ies: 600, ius: 600, condition: "NOT Installed"},
	"ValidateProductID":        {ies: 700, ius: 700},
	"CostInitialize":           {ies: 800, ius: 800, aes: 800, aus: 800, advt: 800},
	"FileCost":                 {ies: 900, ius: 900, aes: 900, aus: 900},
	"IsolateComponents":        {ies: 950, ius: 950},
	"CostFinalize":             {ies: 1000, ius: 1000, aes: 1000, aus: 1000, advt: 1000},
	"SetODBCFolders":           {ies: 1100},
	"MigrateFeatureStates":     {ies: 1200, ius: 1200},
	"ExecuteAction":            {ius: 1300, aus: 1300},
	"InstallValidate":          {ies: 1400, aes: 1400, advt: 1400},
	"RemoveExistingProducts":   {ies: 1401},
	"InstallInitialize":        {ies: 1500, aes: 1500, advt: 1500},
	"AllocateRegistrySpace":    {ies: 1550, condition: "NOT Installed"},
	"ProcessComponents":        {ies: 1600},
	"UnpublishComponents":      {ies: 1700},
	"MsiUnpublishAssemblies":   {ies: 1750},
	"UnpublishFeatures":        {ies: 1800},
	"StopServices":             {ies: 1900, condition: "VersionNT"},
	"DeleteServices":           {ies: 2000, condition: "VersionNT"},
	"UnregisterComPlus":        {ies: 2100},
	"SelfUnregModules":         {ies: 2200},
	"UnregisterTypeLibraries":  {ies: 2300},
	"RemoveODBC":               {ies: 2400},
	"UnregisterFonts":          {ies: 2500},
	"RemoveRegistryValues":     {ies: 2600},
	"UnregisterClassInfo":      {ies: 2700},
	"UnregisterExtensionInfo":  {ies: 2800},
	"UnregisterProgIdInfo":     {ies: 2900},
	"UnregisterMIMEInfo":       {ies: 3000},
	"RemoveIniValues":          {ies: 3100},
	"RemoveShortcuts":          {ies: 3200},
	"RemoveEnvironmentStrings": {ies: 3300},
	"RemoveDuplicateFiles":     {ies: 3400},
	"RemoveFiles":              {ies: 3500},
	"RemoveFolders":            {ies: 3600},
	"CreateFolders":            {ies: 3700},
	"MoveFiles":                {ies: 3800},
	"InstallAdminPackage":      {aes: 3900},
	"InstallFiles":             {ies: 4000, aes: 4000},
	"PatchFiles":               {ies: 4090, aes: 4090},
	"DuplicateFiles":           {ies: 4210},
	"BindImage":                {ies: 4300},
	"CreateShortcuts":          {ies: 4500, advt: 4500},
	"RegisterClassInfo":        {ies: 4600, advt: 4600},
	"RegisterExtensionInfo":    {ies: 4700, advt: 4700},
	"RegisterProgIdInfo":       {ies: 4800, advt: 4800},
	"RegisterMIMEInfo":         {ies: 4900, advt: 4900},
	"WriteRegistryValues":      {ies: 5000},
	"WriteIniValues":           {ies: 5100},
	"WriteEnvironmentStrings":  {ies: 5200},
	"RegisterFonts":            {ies: 5300},
	"InstallODBC":              {ies: 5400},
	"RegisterTypeLibraries":    {ies: 5500},
	"SelfRegModules":           {ies: 5600},
	"RegisterComPlus":          {ies: 5700},
	"InstallServices":          {ies: 5800, condition: "VersionNT"},
	"MsiConfigureServices":     {ies: 5850, condition: "VersionNT>=600"},
	"StartServices":            {ies: 5900, condition: "VersionNT"},
	"RegisterUser":             {ies: 6000},
	"RegisterProduct":          {ies: 6100},
	"PublishComponents":        {ies: 6200, advt: 6200},
	"MsiPublishAssemblies":     {ies: 6250, advt: 6250},
	"PublishFeatures":          {ies: 6300, advt: 6300},
	"PublishProduct":           {ies: 6400, advt: 6400},
	"InstallExecute":           {ies: 6500, condition: "NOT Installed"},
	"InstallExecuteAgain":      {ies: 6550, condition: "NOT Installed"},
	"InstallFinalize":          {ies: 6600, aes: 6600, advt: 6600},
}

// alwaysActions are the standard actions every product gets, per table
// (sections 9.2 and 11.7).
var alwaysActions = map[string][]string{
	seqInstallExecute: {
		"ValidateProductID", "CostInitialize", "FileCost", "CostFinalize", "InstallValidate",
		"InstallInitialize", "ProcessComponents", "UnpublishFeatures", "RegisterUser",
		"RegisterProduct", "PublishFeatures", "PublishProduct", "InstallFinalize",
	},
	seqInstallUI: {"ValidateProductID", "CostInitialize", "FileCost", "CostFinalize", "ExecuteAction"},
	seqAdminExecute: {
		"CostInitialize", "FileCost", "CostFinalize", "InstallValidate", "InstallInitialize", "InstallAdminPackage", "InstallFiles", "InstallFinalize",
	},
	seqAdminUI:     {"CostInitialize", "FileCost", "CostFinalize", "ExecuteAction"},
	seqAdvtExecute: {"CostInitialize", "CostFinalize", "InstallValidate", "InstallInitialize", "PublishFeatures", "PublishProduct", "InstallFinalize"},
}

// tableActions maps a populated table to the InstallExecuteSequence actions
// it needs (section 9.3).
var tableActions = map[string][]string{
	"Component":        {"CreateFolders", "RemoveFolders"},
	"File":             {"InstallFiles", "RemoveFiles"},
	"RemoveFile":       {"RemoveFiles", "RemoveFolders"},
	"Registry":         {"WriteRegistryValues", "RemoveRegistryValues"},
	"RemoveRegistry":   {"WriteRegistryValues", "RemoveRegistryValues"},
	"Environment":      {"WriteEnvironmentStrings", "RemoveEnvironmentStrings"},
	"ServiceInstall":   {"InstallServices", "StartServices", "StopServices", "DeleteServices"},
	"ServiceControl":   {"InstallServices", "StartServices", "StopServices", "DeleteServices"},
	"MsiServiceConfig": {"MsiConfigureServices"},
}

// addAction registers a custom action. The CustomAction row is written by
// sequences, once every contributor has run.
func (b *build) addAction(a *actionInfo) {
	for _, old := range b.actions {
		if old.name == a.name {
			b.errorf("customActions", "custom action %s is defined twice", a.name)

			return
		}
	}

	if s, ok := a.target.(string); ok && len(s) > maxTargetLength {
		b.warnf("customActions", "%s: Target is %d characters, CustomAction.Target is declared with %d (ICE03); the engine still runs it",
			a.name, len(s), maxTargetLength)
	}

	b.actions = append(b.actions, a)
}

func (b *build) action(name string) *actionInfo {
	for _, a := range b.actions {
		if a.name == name {
			return a
		}
	}

	return nil
}

// schedule places an action into a sequence table. Exactly one of before,
// after or at is set.
func (b *build) schedule(table, name, cond, before, after string, at int) {
	b.schedules[table] = append(b.schedules[table], sequence.Action{
		Name: name, Condition: cond, Sequence: at, Before: before, After: after,
	})
}

func (b *build) scheduled(table, name string) bool {
	for _, a := range b.schedules[table] {
		if a.Name == name {
			return true
		}
	}

	return false
}

// standardAt adds a standard action at its default number for the table
// unless it is already scheduled there. It panics on an action without a
// default in that table, which is a builder bug.
func (b *build) standardAt(table, name string) {
	if b.scheduled(table, name) {
		return
	}

	s, ok := standardActions[name]
	if !ok || s.number(table) == 0 {
		panic(fmt.Sprintf("builder: %s has no default number in %s", name, table))
	}

	b.schedule(table, name, s.condition, "", "", s.number(table))
}

// customActions turns the customActions section into CustomAction rows and
// placements.
func (b *build) customActions() error {
	for i, ca := range b.cfg.CustomActions {
		p := fmt.Sprintf("customActions[%d]", i)
		a := &actionInfo{name: ca.ID}

		switch {
		case ca.SetProperty != nil:
			a.typ, a.source, a.target = caSetProperty, ca.SetProperty.Name, ca.SetProperty.Value
		case ca.SetDirectory != nil:
			a.typ, a.source, a.target = caSetDirectory, ca.SetDirectory.Directory, ca.SetDirectory.Value
		case ca.Error != "":
			a.typ, a.target = caError, ca.Error
		case ca.Exec != nil:
			a.typ, a.source, a.target = caExec, ca.Exec.Directory, ca.Exec.Command

			if config.IsStandardFolder(ca.Exec.Directory) {
				b.ensureStandardDir(ca.Exec.Directory)
			}
		case ca.ExecProperty != nil:
			a.typ, a.source, a.target = caExecProperty, ca.ExecProperty.Property, nullable(ca.ExecProperty.Arguments)
		case ca.DLL != nil:
			a.typ, a.source, a.target = caDLL, ca.DLL.Binary, ca.DLL.Entry
		}

		switch ca.Execute {
		case "deferred":
			a.typ |= caDeferred
		case "rollback":
			a.typ |= caRollback
		case "commit":
			a.typ |= caCommit
		}

		if ca.Impersonate != nil && !*ca.Impersonate {
			a.typ |= caNoImpersonate
		}

		switch ca.Return {
		case "ignore":
			a.typ |= caContinue
		case "asyncWait":
			a.typ |= caAsyncWait
		case "asyncNoWait":
			a.typ |= caAsyncNoWait
		}

		switch ca.Once {
		case "firstSequence":
			a.typ |= caFirstSequence
		case "oncePerProcess":
			a.typ |= caOncePerProcess
		case "clientRepeat":
			a.typ |= caClientRepeat
		}

		if ca.HideTarget {
			a.typ |= caHideTarget
		}

		b.addAction(a)

		for j, pl := range ca.Schedule {
			pp := fmt.Sprintf("%s.schedule[%d]", p, j)

			table, ok := config.SequenceTable(pl.Sequence)
			if !ok {
				b.errorf(pp+".sequence", "unknown sequence %q", pl.Sequence)

				continue
			}

			if err := condition.Check(pl.Condition); err != nil {
				b.errorf(pp+".condition", "%v", err)

				continue
			}

			b.schedule(table, ca.ID, pl.Condition, pl.Before, pl.After, pl.At)
		}
	}

	return nil
}

// firewall writes the netsh actions of the firewall helper (section 17.3,
// design section 4.11): on install and repair every rule is deleted by name
// before any is added, with a rollback that deletes again; on removal the
// rule is deleted, with a rollback that adds it back.
func (b *build) firewall() error {
	if len(b.cfg.Firewall) == 0 {
		return nil
	}

	sysDir := b.systemDir()
	netsh := fmt.Sprintf(`"[%s]netsh.exe" advfirewall firewall `, sysDir)

	type rule struct {
		id, add, del, install, remove string
	}

	rules := make([]rule, 0, len(b.cfg.Firewall))

	for _, fw := range b.cfg.Firewall {
		id := generatedID("", "Firewall", fw.Name, fw.Protocol, fw.Port, fw.Program)[:8]
		install := "$" + fw.Component + "=3"
		remove := "$" + fw.Component + "=2"

		remoteIP := fw.RemoteAddresses
		if remoteIP == "" {
			remoteIP = "any"
		} else if refs := propertyRefs(remoteIP); len(refs) > 0 {
			// A property may be empty at run time; fall back to "any" then.
			prop := "FwIP" + id
			b.addAction(&actionInfo{name: "SetFwRemoteIPDefault" + id, typ: caSetProperty, source: prop, target: "any"})
			b.addAction(&actionInfo{name: "SetFwRemoteIP" + id, typ: caSetProperty, source: prop, target: remoteIP})
			b.schedule(seqInstallExecute, "SetFwRemoteIPDefault"+id, "", "", "CostFinalize", 0)
			b.schedule(seqInstallExecute, "SetFwRemoteIP"+id, strings.Join(refs, " AND "), "", "SetFwRemoteIPDefault"+id, 0)

			remoteIP = "[" + prop + "]"
		}

		var parts []string

		parts = append(parts, `name="`+fw.Name+`"`, "dir=in", "action=allow", "protocol="+fw.Protocol)

		if fw.Port != "" {
			parts = append(parts, "localport="+fw.Port)
		}

		if fw.Program != "" {
			parts = append(parts, `program="`+fw.Program+`"`)
		}

		// Defaults (remoteip=any, profile=any, enable=yes) are left out to
		// keep the Target within 255 characters.
		if remoteIP != "any" {
			parts = append(parts, "remoteip="+remoteIP)
		}

		if profiles := firewallProfiles(fw.Profiles); profiles != "any" {
			parts = append(parts, "profile="+profiles)
		}

		if fw.Description != "" {
			parts = append(parts, `description="`+fw.Description+`"`)
		}

		rules = append(rules, rule{
			id:      id,
			add:     netsh + "add rule " + strings.Join(parts, " "),
			del:     netsh + `delete rule name="` + fw.Name + `" dir=in`,
			install: install,
			remove:  remove,
		})
	}

	deferred := caExec | caDeferred | caNoImpersonate
	rollback := caExec | caRollback | caNoImpersonate

	// Removal: rollback (add back) precedes its deferred action (delete).
	for _, r := range rules {
		b.addAction(&actionInfo{name: "FwRemoveRollback" + r.id, typ: rollback | caContinue, source: sysDir, target: r.add})
		b.addAction(&actionInfo{name: "FwRemove" + r.id, typ: deferred | caContinue, source: sysDir, target: r.del})
	}

	// Install: delete every rule, then add each with a rollback that deletes.
	for _, r := range rules {
		b.addAction(&actionInfo{name: "FwClear" + r.id, typ: deferred | caContinue, source: sysDir, target: r.del})
	}

	for _, r := range rules {
		b.addAction(&actionInfo{name: "FwAddRollback" + r.id, typ: rollback | caContinue, source: sysDir, target: r.del})
		b.addAction(&actionInfo{name: "FwAdd" + r.id, typ: deferred, source: sysDir, target: r.add})
	}

	// After InstallFiles, so that the program exists; the chain keeps the
	// definition order.
	anchor := "InstallFiles"
	b.standardAt(seqInstallExecute, anchor)

	for _, r := range rules {
		b.schedule(seqInstallExecute, "FwRemoveRollback"+r.id, r.remove, "", anchor, 0)
		b.schedule(seqInstallExecute, "FwRemove"+r.id, r.remove, "", anchor, 0)
	}

	for _, r := range rules {
		b.schedule(seqInstallExecute, "FwClear"+r.id, r.install, "", anchor, 0)
	}

	for _, r := range rules {
		b.schedule(seqInstallExecute, "FwAddRollback"+r.id, r.install, "", anchor, 0)
		b.schedule(seqInstallExecute, "FwAdd"+r.id, r.install, "", anchor, 0)
	}

	return nil
}

func firewallProfiles(list config.StringList) string {
	if slices.Contains(list, "all") {
		return "any"
	}

	return strings.Join(list, ",")
}

// sequences adds the standard actions each table needs, numbers every table
// and writes the five sequence tables and the CustomAction table.
func (b *build) sequences() error {
	for _, table := range sequenceTables {
		for _, name := range alwaysActions[table] {
			b.standardAt(table, name)
		}
	}

	if b.hasRows("LaunchCondition") {
		b.standardAt(seqInstallExecute, "LaunchConditions")
		b.standardAt(seqInstallUI, "LaunchConditions")
	}

	for tbl, actions := range tableActions {
		if !b.hasRows(tbl) {
			continue
		}

		for _, name := range actions {
			b.standardAt(seqInstallExecute, name)
		}
	}

	if b.hasRows("Component") {
		b.standardAt(seqInstallExecute, "AllocateRegistrySpace")
	}

	// Every scheduled name must be a standard action or a custom action.
	for _, table := range sequenceTables {
		for _, a := range b.schedules[table] {
			if _, std := standardActions[a.Name]; std || b.action(a.Name) != nil {
				continue
			}

			if config.IsStandardAction(a.Name) {
				b.errorf("customActions", "%s: standard action %s has no default number in go-msi; schedule it with an absolute number", table, a.Name)

				continue
			}

			b.errorf("customActions", "%s: action %s is scheduled but not defined", table, a.Name)
		}
	}

	if err := b.diags.Err(); err != nil {
		return err
	}

	for _, table := range sequenceTables {
		numbered, err := sequence.Number(b.schedules[table])
		if err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}

		b.numbered[table] = numbered
		t := b.table(table)

		for _, n := range numbered {
			t.Add(n.Name, nullable(n.Condition), n.Sequence)
		}
	}

	b.checkSequences()

	scheduled := make(map[string]bool)

	for _, table := range sequenceTables {
		for _, a := range b.schedules[table] {
			scheduled[a.Name] = true
		}
	}

	ca := b.table("CustomAction")

	slices.SortStableFunc(b.actions, func(a, b *actionInfo) int { return strings.Compare(a.name, b.name) })

	for _, a := range b.actions {
		if !scheduled[a.name] {
			b.warnf("customActions", "custom action %s is not scheduled in any sequence", a.name)
		}

		ca.Add(a.name, a.typ, a.source, a.target, nil)
	}

	return nil
}

// checkSequences applies the ordering rules that numbering cannot: ICE77
// for script actions and the RemoveExistingProducts windows (ICE63).
func (b *build) checkSequences() {
	for _, table := range sequenceTables {
		seq := b.numbered[table]
		init := sequence.Find(seq, "InstallInitialize")
		fin := sequence.Find(seq, "InstallFinalize")

		for _, n := range seq {
			a := b.action(n.Name)
			if a == nil || a.typ&caDeferred == 0 {
				continue
			}

			if table != seqInstallExecute && table != seqAdminExecute {
				b.errorf("customActions", "%s: deferred action %s belongs in an execute sequence (ICE77)", table, n.Name)

				continue
			}

			if init < 0 || fin < 0 || n.Sequence <= init || n.Sequence >= fin {
				b.errorf("customActions", "%s: deferred action %s at %d must run between InstallInitialize and InstallFinalize (ICE77)", table, n.Name, n.Sequence)
			}
		}
	}

	seq := b.numbered[seqInstallExecute]

	rep := sequence.Find(seq, "RemoveExistingProducts")
	if rep < 0 {
		return
	}

	validate := sequence.Find(seq, "InstallValidate")
	init := sequence.Find(seq, "InstallInitialize")
	fin := sequence.Find(seq, "InstallFinalize")
	next := func(n int) int {
		if n < 0 {
			return -2
		}

		for _, a := range seq {
			if a.Sequence > n {
				return a.Sequence
			}
		}

		return -1
	}

	ok := (rep > validate && rep < init) || // window 1
		rep == next(init) || // window 2
		rep == next(sequence.Find(seq, "InstallExecute")) || // window 3
		rep == next(sequence.Find(seq, "InstallExecuteAgain")) ||
		rep > fin // window 4

	if !ok {
		b.errorf("upgrade.removeOld", "RemoveExistingProducts at %d is outside its four windows (ICE63); the engine fails with 2613", rep)
	}
}
