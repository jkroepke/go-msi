package builder

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jkroepke/go-msi/config"
)

var registryRoots = map[string]int{"HKMU": -1, "HKCR": 0, "HKCU": 1, "HKLM": 2, "HKU": 3}

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
func registryValue(r *config.RegistryValue) any {
	switch r.Type {
	case "key":
		return nil
	case "dword":
		v := r.Value[0]
		if strings.Contains(v, "[") {
			return "#" + v
		}

		n, _ := strconv.ParseInt(v, 0, 64)

		return "#" + strconv.FormatInt(n, 10)
	case "binary":
		h := strings.TrimPrefix(strings.ToLower(r.Value[0]), "0x")

		return "#x" + h
	case "expandString":
		return "#%" + r.Value[0]
	case "multiString":
		joined := strings.Join(r.Value, "[~]")

		switch {
		case r.Append:
			return "[~]" + joined
		case r.Prepend:
			return joined + "[~]"
		default:
			return "[~]" + joined + "[~]"
		}
	default:
		v := r.Value[0]
		if strings.HasPrefix(v, "#") {
			return "#" + v
		}

		return nullable(v)
	}
}

func (b *build) writeRegistry(info *componentInfo) {
	t := b.table("Registry")
	c := info.cfg

	for _, r := range c.Registry {
		var name any

		switch {
		case r.Type == "key" && r.RemoveOnUninstall:
			name = "*"
		case r.Type == "key":
			name = "+"
		default:
			name = nullable(r.Name)
		}

		t.Add(b.registryID(info, r), registryRoots[r.Root], r.Key, name, registryValue(r), c.ID)
	}

	for _, e := range c.EventLog {
		key := `SYSTEM\CurrentControlSet\Services\EventLog\` + e.Log + `\` + e.Source
		t.Add(generatedID("reg", "EventLog", c.ID, e.Log, e.Source, "EventMessageFile"), registryRoots["HKLM"], key, "EventMessageFile", "#%"+e.MessageFile, c.ID)
		t.Add(generatedID("reg", "EventLog", c.ID, e.Log, e.Source, "TypesSupported"), registryRoots["HKLM"], key, "TypesSupported",
			"#"+strconv.Itoa(*e.TypesSupported), c.ID)
	}
}

// writeEnvironment emits Environment rows. The removal flag is always
// written explicitly, because the machine-scope flag does not imply it
// (MSISPEC-RESULTS RUN-08).
func (b *build) writeEnvironment(info *componentInfo) {
	t := b.table("Environment")
	c := info.cfg

	for _, e := range c.Environment {
		var prefix string

		switch e.Action {
		case "setIfAbsent":
			prefix = "+"
		case "remove":
			prefix = "!"
		default:
			prefix = "="
		}

		if *e.RemoveOnUninstall {
			prefix += "-"
		}

		if e.Scope == "machine" {
			prefix += "*"
		}

		var value any

		switch e.Action {
		case "append":
			value = "[~];" + e.Value
		case "prepend":
			value = e.Value + ";[~]"
		default:
			value = nullable(e.Value)
		}

		t.Add(generatedID("env", "Environment", c.ID, e.Name, e.Action, e.Scope), prefix+e.Name, value, c.ID)
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

func (b *build) writeServices(info *componentInfo) {
	c := info.cfg

	for _, s := range c.Services {
		id := serviceID("svi", s.Name)

		typ := serviceOwnProcess
		if s.Type == "shareProcess" {
			typ = serviceShareProcess
		}

		start := startAuto

		switch s.Start {
		case "demand":
			start = startDemand
		case "disabled":
			start = startDisabled
		}

		errCtl := errorNormal

		switch s.ErrorControl {
		case "ignore":
			errCtl = errorIgnore
		case "critical":
			errCtl = errorCritical
		}

		if s.Vital {
			errCtl |= errorVital
		}

		var deps any
		if len(s.Dependencies) > 0 {
			deps = strings.Join(s.Dependencies, "[~]") + "[~][~]"
		}

		b.table("ServiceInstall").Add(id, s.Name, nullable(s.DisplayName), typ, start, errCtl, nil, deps,
			nullable(s.Account), nullable(s.Password), nullable(s.Arguments), c.ID, nullable(s.Description))

		if s.Password != "" {
			for _, prop := range propertyRefs(s.Password) {
				b.hidden[prop] = true
			}
		}

		event := 0

		if *s.Control.StartOnInstall {
			event |= eventStartInstall
		}

		if *s.Control.StopOnInstall {
			event |= eventStopInstall
		}

		if *s.Control.StopOnUninstall {
			event |= eventStopUninstall
		}

		if *s.Control.DeleteOnUninstall {
			event |= eventDeleteUninstall
		}

		if event != 0 {
			wait := 0
			if *s.Control.Wait {
				wait = 1
			}

			b.table("ServiceControl").Add(serviceID("sc", s.Name), s.Name, event, nil, wait, c.ID)
		}

		if s.DelayedAutoStart {
			b.table("MsiServiceConfig").Add(serviceID("scd", s.Name), s.Name, configEventInstall|configEventReinstall, configDelayedStart, "1", c.ID)
		}

		if s.FailureActionsOnNonCrashFailures {
			b.table("MsiServiceConfig").Add(serviceID("scf", s.Name), s.Name, configEventInstall|configEventReinstall, configFailureNonCrash, "1", c.ID)
		}

		if s.Recovery != nil {
			b.recoveryAction(info, s)
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
func (b *build) recoveryAction(info *componentInfo, s *config.Service) {
	rec := s.Recovery
	delay := strconv.FormatInt(int64(time.Duration(rec.RestartDelay)/time.Millisecond), 10)

	var actions []string

	for _, a := range rec.Actions {
		switch a {
		case "restart":
			actions = append(actions, "restart/"+delay)
		default:
			actions = append(actions, "none/0")
		}
	}

	for len(actions) < 3 {
		actions = append(actions, "none/0")
	}

	sysDir := b.systemDir()
	cmd := fmt.Sprintf(`"[%s]sc.exe" failure "%s" reset= %d actions= %s`, sysDir, s.Name, rec.ResetAfterDays*86400, strings.Join(actions, "/"))
	name := serviceID("ServiceRecovery", s.Name)

	b.addAction(&actionInfo{name: name, typ: caExec | caDeferred | caNoImpersonate | caContinue, source: sysDir, target: cmd})
	b.schedule("InstallExecuteSequence", name, "$"+info.cfg.ID+"=3", "", "InstallServices", 0)
}

// systemDir returns the Directory key of the system folder that holds the
// native command-line tools for this package's bitness.
func (b *build) systemDir() string {
	id := "SystemFolder"
	if b.is64 {
		id = "System64Folder"
	}

	b.ensureStandardDir(id)

	return id
}

// propertyRefs lists the [PROPERTY] references in a formatted string.
func propertyRefs(s string) []string {
	var out []string

	for {
		i := strings.IndexByte(s, '[')
		if i < 0 {
			return out
		}

		j := strings.IndexByte(s[i:], ']')
		if j < 0 {
			return out
		}

		name := s[i+1 : i+j]
		if config.IsIdentifier(name) {
			out = append(out, name)
		}

		s = s[i+j+1:]
	}
}
