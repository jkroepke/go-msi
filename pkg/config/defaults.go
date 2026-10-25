package config

import "time"

// Defaults of docs/design/yaml-schema.md section 4.
const (
	DefaultLanguage         = 1033
	DefaultInstallerVersion = "5.0"
	DefaultCodepage         = 1252
	DefaultCompression      = "mszip"
	DefaultRemoveOld        = "afterInstallInitialize"
	DefaultUpgradeProperty  = "OLDERVERSIONBEINGUPGRADED"
	DefaultNewerProperty    = "NEWERVERSIONDETECTED"
	DefaultDowngradeMessage = "A later version of [ProductName] is already installed. Setup will now exit."
	DefaultRestartDelay     = time.Minute

	// Auto marks a value that the builder derives (GUIDs, file versions).
	Auto = "auto"
	// None marks an explicitly absent value.
	None = "none"
)

func ptr[T any](v T) *T { return &v }

// ApplyDefaults fills in every unset field with its default. It only sets
// empty fields, so applying it twice changes nothing. Load and Validate call
// it; a Config built in Go can call it directly.
func ApplyDefaults(cfg *Config) {
	p := &cfg.Product
	if p.Language == nil {
		p.Language = ptr(DefaultLanguage)
	}

	if p.ProductCode == "" {
		p.ProductCode = Auto
	}

	if p.PackageCode == "" {
		p.PackageCode = Auto
	}

	if p.Description == "" {
		p.Description = p.Name + " " + p.Version + " installer"
	}

	if p.Comments == "" {
		p.Comments = "This installer database contains the logic and data required to install " + p.Name + "."
	}

	pkg := &cfg.Package
	if pkg.Scope == "" {
		pkg.Scope = ScopePerMachine
	}

	if pkg.InstallerVersion == "" {
		pkg.InstallerVersion = DefaultInstallerVersion
	}

	if pkg.Codepage == 0 {
		pkg.Codepage = DefaultCodepage
	}

	if pkg.Compression == "" {
		pkg.Compression = DefaultCompression
	}

	if pkg.Cabinet == "" {
		pkg.Cabinet = p.Name + ".cab"
	}

	if pkg.LongFileNames == nil {
		pkg.LongFileNames = ptr(true)
	}

	if pkg.Summary.Title == "" {
		pkg.Summary.Title = "Installation Database"
	}

	if pkg.Summary.Keywords == "" {
		pkg.Summary.Keywords = "Installer"
	}

	if pkg.Summary.Author == "" {
		pkg.Summary.Author = p.Manufacturer
	}

	expandShorthands(cfg)

	if cfg.Upgrade == nil {
		cfg.Upgrade = &Upgrade{}
	}

	defaultUpgrade(cfg.Upgrade)

	eachFeature(cfg.Features, func(feature *Feature, _ int) {
		if feature.Level == nil {
			feature.Level = new(defaultLevel(feature.Default))
			feature.levelFromDefault = feature.Default != ""
		}

		if feature.Display == "" {
			feature.Display = "collapsed"
		}

		if feature.AllowAbsent == nil {
			feature.AllowAbsent = new(true)
		}
	})

	for _, c := range cfg.Components {
		defaultComponent(c)
	}

	for _, ca := range cfg.CustomActions {
		if ca.Execute == "" {
			ca.Execute = "immediate"
		}

		if ca.Impersonate == nil {
			ca.Impersonate = ptr(true)
		}

		// exec actions must state how their exit code is handled; the
		// validator reports a missing value instead of defaulting it.
		if ca.Return == "" && ca.Exec == nil && ca.ExecProperty == nil {
			ca.Return = "check"
		}

		if ca.Exec != nil && ca.Exec.Directory == "" {
			ca.Exec.Directory = "TARGETDIR"
		}
	}

	for _, fw := range cfg.Firewall {
		if fw.Protocol == "" {
			fw.Protocol = "tcp"
		}

		if len(fw.Profiles) == 0 {
			fw.Profiles = StringList{"all"}
		}
	}

	if cfg.UI.Wizard == "" {
		cfg.UI.Wizard = "none"
	}
}

func defaultUpgrade(upg *Upgrade) {
	if upg.AllowSameVersion == nil {
		upg.AllowSameVersion = new(true)
	}

	if upg.Downgrade == "" {
		upg.Downgrade = "error"
	}

	if upg.DowngradeMessage == "" {
		upg.DowngradeMessage = DefaultDowngradeMessage
	}

	if upg.RemoveOld == "" {
		upg.RemoveOld = DefaultRemoveOld
	}

	if upg.MigrateFeatures == nil {
		upg.MigrateFeatures = new(true)
	}

	if upg.Property == "" {
		upg.Property = DefaultUpgradeProperty
	}

	if upg.NewerProperty == "" {
		upg.NewerProperty = DefaultNewerProperty
	}

	for i := range upg.Related {
		if upg.Related[i].IncludeMin == nil {
			upg.Related[i].IncludeMin = new(true)
		}
	}
}

// defaultLevel is the feature level for a default: on installs the feature
// at the default INSTALLLEVEL of 1, off needs a higher INSTALLLEVEL or
// ADDLOCAL, and disabled is level 0. An invalid value, which the validator
// reports, gives 1.
func defaultLevel(value string) int {
	switch value {
	case FeatureOff:
		return featureLevelOff
	case FeatureDisabled:
		return 0
	default:
		return 1
	}
}

func defaultComponent(c *Component) {
	if c.GUID == "" {
		c.GUID = Auto
	}

	if c.Bitness == "" {
		c.Bitness = Auto
	}

	for _, f := range c.Files {
		if f.Version == "" {
			f.Version = Auto
		}

		if f.Language == "" {
			f.Language = Auto
		}

		if f.Vital == nil {
			f.Vital = ptr(true)
		}
	}

	for _, r := range c.Registry {
		if r.Type == "" {
			r.Type = "string"
		}
	}

	for _, s := range c.Services {
		if s.DisplayName == "" {
			s.DisplayName = s.Name
		}

		if s.Type == "" {
			s.Type = "ownProcess"
		}

		if s.Start == "" {
			s.Start = "auto"
		}

		if s.ErrorControl == "" {
			s.ErrorControl = "normal"
		}

		ctl := &s.Control
		for _, b := range []**bool{&ctl.StartOnInstall, &ctl.StopOnInstall, &ctl.StopOnUninstall, &ctl.DeleteOnUninstall, &ctl.Wait} {
			if *b == nil {
				*b = ptr(true)
			}
		}

		if s.Recovery != nil && s.Recovery.RestartDelay == 0 {
			s.Recovery.RestartDelay = Duration(DefaultRestartDelay)
		}
	}

	for _, e := range c.EventLog {
		if e.Log == "" {
			e.Log = "Application"
		}

		if e.TypesSupported == nil {
			e.TypesSupported = ptr(7)
		}
	}

	for _, e := range c.Environment {
		if e.Action == "" {
			e.Action = "set"
		}

		if e.Scope == "" {
			e.Scope = "machine"
		}

		// A removal has nothing to undo at uninstall.
		if e.RemoveOnUninstall == nil {
			e.RemoveOnUninstall = ptr(e.Action != "remove")
		}
	}
}

// eachFeature calls fn for every feature, depth first, with its depth
// starting at 1.
func eachFeature(features []*Feature, fn func(*Feature, int)) {
	var walk func([]*Feature, int)

	walk = func(fs []*Feature, depth int) {
		for _, f := range fs {
			fn(f, depth)
			walk(f.Features, depth+1)
		}
	}

	walk(features, 1)
}
