package builder

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jkroepke/go-msi/pkg/config"
)

// regTypeKey marks a registry row that creates a key without a value.
const regTypeKey = "key"

var registryRoots = map[string]int{"HKMU": -1, "HKCR": 0, "HKCU": 1, "HKLM": 2, "HKU": 3} //nolint:gochecknoglobals // lookup table

// registryID returns the Registry key of a value: the explicit id, else one
// generated from the value's identity.
func (b *build) registryID(info *componentInfo, r *config.RegistryValue) string {
	if r.ID != "" {
		return r.ID
	}

	return generatedID("reg", "Registry", info.cfg.ID, r.Root, strings.ToLower(r.Key), strings.ToLower(r.Name))
}

// registryValue encodes a typed value the way the Registry table expects
// (section 8.13): #n for REG_DWORD, #x for REG_BINARY, #% for
// REG_EXPAND_SZ, [~]-joined strings for REG_MULTI_SZ, ## for a literal #.
func registryValue(reg *config.RegistryValue) any { //nolint:cyclop // one pass over the format; splitting would scatter it
	switch reg.Type {
	case regTypeKey:
		return nil
	case "dword":
		v := reg.Value[0]
		if strings.Contains(v, "[") {
			return "#" + v
		}

		n, _ := strconv.ParseInt(v, 0, 64)

		return "#" + strconv.FormatInt(n, 10)
	case "binary":
		h := strings.TrimPrefix(strings.ToLower(reg.Value[0]), "0x")

		return "#x" + h
	case "expandString":
		return "#%" + reg.Value[0]
	case "multiString":
		joined := strings.Join(reg.Value, "[~]")

		switch {
		case reg.Append:
			return "[~]" + joined
		case reg.Prepend:
			return joined + "[~]"
		default:
			return "[~]" + joined + "[~]"
		}
	default:
		v := reg.Value[0]
		if strings.HasPrefix(v, "#") {
			return "#" + v
		}

		return nullable(v)
	}
}

func (b *build) writeRegistry(info *componentInfo) {
	tbl := b.table("Registry")
	comp := info.cfg

	for _, reg := range comp.Registry {
		var name any

		switch {
		case reg.Type == "key" && reg.RemoveOnUninstall:
			name = "*"
		case reg.Type == "key":
			name = "+"
		default:
			name = nullable(reg.Name)
		}

		tbl.Add(b.registryID(info, reg), registryRoots[reg.Root], reg.Key, name, registryValue(reg), comp.ID)
	}

	for _, e := range comp.EventLog {
		key := `SYSTEM\CurrentControlSet\Services\EventLog\` + e.Log + `\` + e.Source
		tbl.Add(generatedID("reg", "EventLog", comp.ID, e.Log, e.Source, "EventMessageFile"), registryRoots["HKLM"], key, "EventMessageFile",
			"#%"+e.MessageFile, comp.ID)
		tbl.Add(generatedID("reg", "EventLog", comp.ID, e.Log, e.Source, "TypesSupported"), registryRoots["HKLM"], key, "TypesSupported",
			"#"+strconv.Itoa(*e.TypesSupported), comp.ID)
	}
}

// writeEnvironment emits Environment rows. The removal flag is always
// written explicitly, because the machine-scope flag does not imply it
// (MSISPEC-RESULTS RUN-08).
func (b *build) writeEnvironment(info *componentInfo) {
	tbl := b.table("Environment")
	comp := info.cfg

	for _, env := range comp.Environment {
		var prefix string

		switch env.Action {
		case "setIfAbsent":
			prefix = "+"
		case "remove":
			prefix = "!"
		default:
			prefix = "="
		}

		if *env.RemoveOnUninstall {
			prefix += "-"
		}

		if env.Scope == "machine" {
			prefix += "*"
		}

		var value any

		switch env.Action {
		case "append":
			value = "[~];" + env.Value
		case "prepend":
			value = env.Value + ";[~]"
		default:
			value = nullable(env.Value)
		}

		tbl.Add(generatedID("env", "Environment", comp.ID, env.Name, env.Action, env.Scope), prefix+env.Name, value, comp.ID)
	}
}

// Service constants.
const (
	serviceOwnProcess   = 0x10
	serviceShareProcess = 0x20
	startAuto           = 2
	startDemand         = 3
	startDisabled       = 4
	errorIgnore         = 0
	errorNormal         = 1
	errorCritical       = 3
	errorVital          = 0x8000

	eventStartInstall    = 1
	eventStopInstall     = 2
	eventDeleteInstall   = 8
	eventStartUninstall  = 16
	eventStopUninstall   = 32
	eventDeleteUninstall = 128

	configEventInstall    = 1
	configEventReinstall  = 4
	configDelayedStart    = 3
	configFailureNonCrash = 4
)

func (b *build) writeServices(info *componentInfo) { //nolint:cyclop,gocognit // one pass over the format; splitting would scatter it
	comp := info.cfg

	for _, svc := range comp.Services {
		installID := serviceID("svi", svc.Name)

		typ := serviceOwnProcess
		if svc.Type == "shareProcess" {
			typ = serviceShareProcess
		}

		start := startAuto

		switch svc.Start {
		case "demand":
			start = startDemand
		case "disabled":
			start = startDisabled
		}

		errCtl := errorNormal

		switch svc.ErrorControl {
		case "ignore":
			errCtl = errorIgnore
		case "critical":
			errCtl = errorCritical
		}

		if svc.Vital {
			errCtl |= errorVital
		}

		var deps any
		if len(svc.Dependencies) > 0 {
			deps = strings.Join(svc.Dependencies, "[~]") + "[~][~]"
		}

		b.table("ServiceInstall").Add(installID, svc.Name, nullable(svc.DisplayName), typ, start, errCtl, nil, deps,
			nullable(svc.Account), nullable(svc.Password), nullable(svc.Arguments.String()), comp.ID, nullable(svc.Description))

		if svc.Password != "" {
			for _, prop := range propertyRefs(svc.Password) {
				b.hidden[prop] = true
			}
		}

		event := 0

		if *svc.Control.StartOnInstall {
			event |= eventStartInstall
		}

		if *svc.Control.StopOnInstall {
			event |= eventStopInstall
		}

		if *svc.Control.StopOnUninstall {
			event |= eventStopUninstall
		}

		if *svc.Control.DeleteOnUninstall {
			event |= eventDeleteUninstall
		}

		if event != 0 {
			wait := 0
			if *svc.Control.Wait {
				wait = 1
			}

			b.table("ServiceControl").Add(serviceID("sc", svc.Name), svc.Name, event, nil, wait, comp.ID)
		}

		if svc.DelayedAutoStart {
			b.table("MsiServiceConfig").Add(serviceID("scd", svc.Name), svc.Name, configEventInstall|configEventReinstall, configDelayedStart, "1", comp.ID)
		}

		if svc.FailureActionsOnNonCrashFailures {
			b.table("MsiServiceConfig").Add(serviceID("scf", svc.Name), svc.Name, configEventInstall|configEventReinstall, configFailureNonCrash, "1", comp.ID)
		}

		if svc.Recovery != nil {
			b.recoveryAction(info, svc)
		}
	}
}

// serviceID builds a table key from a service name.
func serviceID(prefix, name string) string {
	if config.IsIdentifier(name) && len(prefix)+1+len(name) <= 72 {
		return prefix + "_" + name
	}

	return generatedID(prefix, name)
}

// recoveryAction schedules the sc.exe failure command that replaces the
// MsiServiceConfigFailureActions table, which fails with 1939 for restart
// actions (MSISPEC-RESULTS RUN-05).
func (b *build) recoveryAction(info *componentInfo, svc *config.Service) {
	rec := svc.Recovery

	var actions []string

	for _, a := range rec.Actions {
		switch a.Action {
		case "restart":
			delay := rec.RestartDelay
			if a.Restart != nil {
				delay = *a.Restart
			}

			actions = append(actions, "restart/"+strconv.FormatInt(int64(time.Duration(delay)/time.Millisecond), 10))
		default:
			actions = append(actions, "none/0")
		}
	}

	reset := int64(rec.ResetAfterDays) * 86400
	if rec.ResetAfter != nil {
		reset = int64(time.Duration(*rec.ResetAfter) / time.Second)
	}

	for len(actions) < 3 {
		actions = append(actions, "none/0")
	}

	sysDir := b.systemDir()
	cmd := fmt.Sprintf(`"[%s]sc.exe" failure "%s" reset= %d actions= %s`, sysDir, svc.Name, reset, strings.Join(actions, "/"))
	name := serviceID("ServiceRecovery", svc.Name)

	b.addAction(&actionInfo{name: name, typ: caExec | caDeferred | caNoImpersonate | caContinue, source: sysDir, target: cmd})
	b.schedule("InstallExecuteSequence", name, "$"+info.cfg.ID+"=3", "", actInstallServices, 0)
}

// purgeAction schedules a deferred rmdir of the component's directory tree
// on a real uninstall. RemoveFile rows cannot do it: they never recurse and
// they also run on the uninstall half of a major upgrade (RUN-26).
func (b *build) purgeAction(info *componentInfo) {
	comp := info.cfg
	if !comp.Purge.Enabled() {
		return
	}

	sysDir := b.systemDir()
	// The "." keeps the trailing backslash of the folder path away from the
	// closing quote.
	cmd := fmt.Sprintf(`"[%s]cmd.exe" /c rmdir /s /q "[%s]."`, sysDir, comp.Directory)

	typ := caExec | caDeferred | caContinue
	if b.cfg.Package.Scope == config.ScopePerMachine {
		typ |= caNoImpersonate
	}

	// An empty property would leave "." alone, the working folder.
	cond := `REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE AND ` + comp.Directory
	if comp.Purge.Condition != "" {
		cond += " AND (" + comp.Purge.Condition + ")"
	}

	name := generatedID("Purge", "Purge", comp.ID)
	if config.IsIdentifier("Purge_"+comp.ID) && len("Purge_"+comp.ID) <= 72 {
		name = "Purge_" + comp.ID
	}

	b.addAction(&actionInfo{name: name, typ: typ, source: sysDir, target: cmd})
	b.schedule(seqInstallExecute, name, cond, actInstallFinalize, "", 0)
}

// systemDir returns the Directory key of the system folder that holds the
// native command-line tools for this package's bitness.
func (b *build) systemDir() string {
	dirID := "SystemFolder"
	if b.is64 {
		dirID = "System64Folder"
	}

	b.ensureStandardDir(dirID)

	return dirID
}

// propertyRefs lists the [PROPERTY] references in a formatted string.
func propertyRefs(text string) []string {
	var out []string

	for {
		i := strings.IndexByte(text, '[')
		if i < 0 {
			return out
		}

		end := strings.IndexByte(text[i:], ']')
		if end < 0 {
			return out
		}

		name := text[i+1 : i+end]
		if config.IsIdentifier(name) {
			out = append(out, name)
		}

		text = text[i+end+1:]
	}
}
