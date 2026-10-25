// Package config defines go-msi's YAML input format (schema go-msi/v1).
//
// The format is specified in docs/design/yaml-schema.md. This package loads a
// file, substitutes ${NAME} variables, applies defaults and validates
// everything that can be checked without reading the referenced input files.
package config

// SchemaV1 is the only schema identifier this package accepts.
const SchemaV1 = "go-msi/v1"

// Config is a whole package description.
type Config struct {
	Schema           string            `yaml:"schema"`
	Vars             map[string]string `yaml:"vars"`
	Product          Product           `yaml:"product"`
	Package          Package           `yaml:"package"`
	Upgrade          *Upgrade          `yaml:"upgrade"`
	Directories      []*Directory      `yaml:"directories"`
	Features         []*Feature        `yaml:"features"`
	Components       []*Component      `yaml:"components"`
	Properties       Properties        `yaml:"properties"`
	LaunchConditions []LaunchCondition `yaml:"launchConditions"`
	CustomActions    []*CustomAction   `yaml:"customActions"`
	Icons            map[string]string `yaml:"icons"`
	Binaries         map[string]string `yaml:"binaries"`
	Firewall         []*FirewallRule   `yaml:"firewall"`
	UI               UI                `yaml:"ui"`
	Tables           []*Table          `yaml:"tables"`

	// BaseDir is the directory of the YAML file. Relative source paths
	// resolve against it. It is set by Load, not read from YAML.
	BaseDir string `yaml:"-"`
}

// Product holds the product's identity.
type Product struct {
	Name         string `yaml:"name"`
	Manufacturer string `yaml:"manufacturer"`
	Version      string `yaml:"version"`
	Language     *int   `yaml:"language"`
	UpgradeCode  string `yaml:"upgradeCode"`
	ProductCode  string `yaml:"productCode"`
	PackageCode  string `yaml:"packageCode"`
	Description  string `yaml:"description"`
	Comments     string `yaml:"comments"`
	ARP          ARP    `yaml:"arp"`
}

// ARP holds the Add/Remove Programs properties.
type ARP struct {
	Comments        string `yaml:"comments"`
	Contact         string `yaml:"contact"`
	HelpLink        string `yaml:"helpLink"`
	AboutLink       string `yaml:"aboutLink"`
	UpdateLink      string `yaml:"updateLink"`
	Icon            string `yaml:"icon"`
	NoRepair        bool   `yaml:"noRepair"`
	NoModify        bool   `yaml:"noModify"`
	NoRemove        bool   `yaml:"noRemove"`
	Size            int    `yaml:"size"`
	InstallLocation string `yaml:"installLocation"`
}

// Package holds platform and container settings.
type Package struct {
	Platform         Platform `yaml:"platform"`
	Scope            Scope    `yaml:"scope"`
	InstallerVersion string   `yaml:"installerVersion"`
	Codepage         int      `yaml:"codepage"`
	Compression      string   `yaml:"compression"`
	Cabinet          string   `yaml:"cabinet"`
	LongFileNames    *bool    `yaml:"longFileNames"`
	Summary          Summary  `yaml:"summary"`
}

// Summary overrides Summary Information strings.
type Summary struct {
	Title    string `yaml:"title"`
	Keywords string `yaml:"keywords"`
	Author   string `yaml:"author"`
}

// Upgrade is the major-upgrade policy. A nil *Upgrade writes no Upgrade rows.
type Upgrade struct {
	AllowSameVersion *bool            `yaml:"allowSameVersion"`
	Downgrade        string           `yaml:"downgrade"`
	DowngradeMessage string           `yaml:"downgradeMessage"`
	RemoveOld        string           `yaml:"removeOld"`
	MigrateFeatures  *bool            `yaml:"migrateFeatures"`
	Property         string           `yaml:"property"`
	NewerProperty    string           `yaml:"newerProperty"`
	Related          []RelatedProduct `yaml:"related"`
}

// RelatedProduct is an extra Upgrade row.
type RelatedProduct struct {
	UpgradeCode string `yaml:"upgradeCode"`
	MinVersion  string `yaml:"minVersion"`
	MaxVersion  string `yaml:"maxVersion"`
	IncludeMin  *bool  `yaml:"includeMin"`
	IncludeMax  bool   `yaml:"includeMax"`
	OnlyDetect  bool   `yaml:"onlyDetect"`
	Property    string `yaml:"property"`
}

// Directory is a node of the install tree.
type Directory struct {
	ID           string       `yaml:"id"`
	Name         string       `yaml:"name"`
	Configurable bool         `yaml:"configurable"`
	Children     []*Directory `yaml:"children"`
}

// Feature is a node of the feature tree.
type Feature struct {
	ID             string     `yaml:"id"`
	Title          string     `yaml:"title"`
	Description    string     `yaml:"description"`
	Level          *int       `yaml:"level"`
	Display        string     `yaml:"display"`
	Directory      string     `yaml:"directory"`
	AllowAbsent    *bool      `yaml:"allowAbsent"`
	AllowAdvertise bool       `yaml:"allowAdvertise"`
	Components     []string   `yaml:"components"`
	Features       []*Feature `yaml:"features"`
}

// Component is a unit that is installed, repaired and removed together.
type Component struct {
	ID                string           `yaml:"id"`
	Directory         string           `yaml:"directory"`
	GUID              string           `yaml:"guid"`
	Files             []*File          `yaml:"files"`
	Registry          []*RegistryValue `yaml:"registry"`
	Services          []*Service       `yaml:"services"`
	EventLog          []*EventSource   `yaml:"eventLog"`
	Environment       []*Environment   `yaml:"environment"`
	CreateFolder      bool             `yaml:"createFolder"`
	Condition         string           `yaml:"condition"`
	Permanent         bool             `yaml:"permanent"`
	NeverOverwrite    bool             `yaml:"neverOverwrite"`
	SharedDLLRefCount bool             `yaml:"sharedDllRefCount"`
	Transitive        bool             `yaml:"transitive"`
	KeyPath           string           `yaml:"keyPath"`
	Bitness           string           `yaml:"bitness"`
}

// File is a payload file.
type File struct {
	Source                     string `yaml:"source"`
	Name                       string `yaml:"name"`
	ID                         string `yaml:"id"`
	Version                    string `yaml:"version"`
	Language                   string `yaml:"language"`
	AllowUnversionedExecutable bool   `yaml:"allowUnversionedExecutable"`
	Vital                      *bool  `yaml:"vital"`
	Checksum                   bool   `yaml:"checksum"`
	ReadOnly                   bool   `yaml:"readOnly"`
	Hidden                     bool   `yaml:"hidden"`
	System                     bool   `yaml:"system"`
}

// RegistryValue is a typed registry value or key.
type RegistryValue struct {
	ID                string     `yaml:"id"`
	Root              string     `yaml:"root"`
	Key               string     `yaml:"key"`
	Name              string     `yaml:"name"`
	Type              string     `yaml:"type"`
	Value             StringList `yaml:"value"`
	Append            bool       `yaml:"append"`
	Prepend           bool       `yaml:"prepend"`
	RemoveOnUninstall bool       `yaml:"removeOnUninstall"`
}

// Service is a Windows service installed from the component's key file.
type Service struct {
	Name                             string           `yaml:"name"`
	DisplayName                      string           `yaml:"displayName"`
	Description                      string           `yaml:"description"`
	Type                             string           `yaml:"type"`
	Start                            string           `yaml:"start"`
	DelayedAutoStart                 bool             `yaml:"delayedAutoStart"`
	ErrorControl                     string           `yaml:"errorControl"`
	Vital                            bool             `yaml:"vital"`
	Account                          string           `yaml:"account"`
	Password                         string           `yaml:"password"`
	Arguments                        string           `yaml:"arguments"`
	Dependencies                     []string         `yaml:"dependencies"`
	FailureActionsOnNonCrashFailures bool             `yaml:"failureActionsOnNonCrashFailures"`
	Control                          ServiceControl   `yaml:"control"`
	Recovery                         *ServiceRecovery `yaml:"recovery"`
}

// ServiceControl selects the ServiceControl events.
type ServiceControl struct {
	StartOnInstall    *bool `yaml:"startOnInstall"`
	StopOnInstall     *bool `yaml:"stopOnInstall"`
	StopOnUninstall   *bool `yaml:"stopOnUninstall"`
	DeleteOnUninstall *bool `yaml:"deleteOnUninstall"`
	Wait              *bool `yaml:"wait"`
}

// ServiceRecovery is applied with sc.exe after InstallServices.
type ServiceRecovery struct {
	ResetAfterDays int      `yaml:"resetAfterDays"`
	Actions        []string `yaml:"actions"`
	RestartDelay   Duration `yaml:"restartDelay"`
}

// EventSource is an event log source registered with plain Registry rows.
type EventSource struct {
	Source         string `yaml:"source"`
	Log            string `yaml:"log"`
	MessageFile    string `yaml:"messageFile"`
	TypesSupported *int   `yaml:"typesSupported"`
}

// Environment is an environment variable change.
type Environment struct {
	Name              string `yaml:"name"`
	Value             string `yaml:"value"`
	Action            string `yaml:"action"`
	Scope             string `yaml:"scope"`
	RemoveOnUninstall *bool  `yaml:"removeOnUninstall"`
}

// LaunchCondition must be true for the install to start.
type LaunchCondition struct {
	Condition string `yaml:"condition"`
	Message   string `yaml:"message"`
}

// CustomAction is one custom action and where it runs.
type CustomAction struct {
	ID           string        `yaml:"id"`
	SetProperty  *SetProperty  `yaml:"setProperty"`
	SetDirectory *SetDirectory `yaml:"setDirectory"`
	Error        string        `yaml:"error"`
	Exec         *Exec         `yaml:"exec"`
	ExecProperty *ExecProperty `yaml:"execProperty"`
	DLL          *DLL          `yaml:"dll"`
	Execute      string        `yaml:"execute"`
	Impersonate  *bool         `yaml:"impersonate"`
	Return       string        `yaml:"return"`
	HideTarget   bool          `yaml:"hideTarget"`
	Once         string        `yaml:"once"`
	Schedule     []Placement   `yaml:"schedule"`
}

// SetProperty is a type 51 action.
type SetProperty struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// SetDirectory is a type 35 action.
type SetDirectory struct {
	Directory string `yaml:"directory"`
	Value     string `yaml:"value"`
}

// Exec is a type 34 action: a command line run in a directory.
type Exec struct {
	Command   string `yaml:"command"`
	Directory string `yaml:"directory"`
}

// ExecProperty is a type 50 action: the executable path is in a property.
type ExecProperty struct {
	Property  string `yaml:"property"`
	Arguments string `yaml:"arguments"`
}

// DLL is a type 1 action from the Binary table.
type DLL struct {
	Binary string `yaml:"binary"`
	Entry  string `yaml:"entry"`
}

// Placement puts an action into one sequence table.
type Placement struct {
	Sequence  string `yaml:"sequence"`
	Before    string `yaml:"before"`
	After     string `yaml:"after"`
	At        int    `yaml:"at"`
	Condition string `yaml:"condition"`
}

// FirewallRule is an inbound rule that follows a component's state.
type FirewallRule struct {
	Name            string     `yaml:"name"`
	Description     string     `yaml:"description"`
	Component       string     `yaml:"component"`
	Program         string     `yaml:"program"`
	Protocol        string     `yaml:"protocol"`
	Port            string     `yaml:"port"`
	RemoteAddresses string     `yaml:"remoteAddresses"`
	Profiles        StringList `yaml:"profiles"`
}

// UI selects the built-in wizard and adds dialogs to it.
type UI struct {
	Wizard  string    `yaml:"wizard"`
	Dialogs []*Dialog `yaml:"dialogs"`
}

// Dialog is a form dialog laid out by the builder.
type Dialog struct {
	ID          string        `yaml:"id"`
	Title       string        `yaml:"title"`
	Description string        `yaml:"description"`
	After       string        `yaml:"after"`
	Fields      []DialogField `yaml:"fields"`
}

// DialogField is one label and edit field bound to a property.
type DialogField struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label"`
}

// Table is a custom table written as given.
type Table struct {
	Name    string   `yaml:"name"`
	Columns []Column `yaml:"columns"`
	Rows    [][]any  `yaml:"rows"`
}

// Column is a custom table column in .idt type notation (s72, I2, L0, V0, ...).
type Column struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Key  bool   `yaml:"key"`
}
