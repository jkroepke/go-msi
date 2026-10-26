// Package config defines go-msi's YAML input format (schema go-msi/v1).
//
// The format is specified in docs/design/yaml-schema.md. This package loads a
// file, substitutes ${NAME} variables, applies defaults and validates
// everything that can be checked without reading the referenced input files.
package config

import (
	"io/fs"
	"strconv"
	"strings"
)

// SchemaV1 is the only schema identifier this package accepts.
const SchemaV1 = "go-msi/v1"

// Config is a whole package description.
type Config struct { //nolint:govet // field order is the YAML key order
	Schema           string            `yaml:"schema"`
	Vars             map[string]string `yaml:"vars"`
	Product          Product           `yaml:"product"`
	Package          Package           `yaml:"package"`
	Upgrade          *Upgrade          `yaml:"upgrade"`
	InstallDir       InstallDir        `yaml:"installDir"`
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

	// FS, when set, is where every relative input path (files, icons,
	// binaries, binary cells of custom tables, the license and theme
	// pictures) is read from instead of BaseDir.
	// Paths in it use forward slashes. It is for Go callers and is never
	// read from YAML.
	FS fs.FS `yaml:"-"`
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
type ARP struct { //nolint:govet // field order is the YAML key order
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
type Package struct { //nolint:govet // field order is the YAML key order
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

// Upgrade is the major-upgrade policy. ApplyDefaults turns a nil *Upgrade
// into the standard policy; "upgrade: false" sets Disabled, which writes no
// Upgrade rows.
type Upgrade struct { //nolint:govet // field order is the YAML key order
	Disabled         bool             `yaml:"-"`
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
type RelatedProduct struct { //nolint:govet // field order is the YAML key order
	UpgradeCode string `yaml:"upgradeCode"`
	MinVersion  string `yaml:"minVersion"`
	MaxVersion  string `yaml:"maxVersion"`
	IncludeMin  *bool  `yaml:"includeMin"`
	IncludeMax  bool   `yaml:"includeMax"`
	OnlyDetect  bool   `yaml:"onlyDetect"`
	Property    string `yaml:"property"`
}

// InstallDir is the product's main folder, the default directory of every
// component. ApplyDefaults adds it to the directory tree unless directories
// already define its id.
type InstallDir struct {
	// ID is the Directory key and the property that holds the path.
	ID string `yaml:"id"`
	// Name is the folder name below Parent; "/" separates nested folders.
	Name string `yaml:"name"`
	// Parent is the standard folder the install dir is created in.
	Parent string `yaml:"parent"`
	// Remember stores the chosen folder in the registry and reads it back
	// on the next install, so that a major upgrade keeps a custom folder
	// (RUN-25).
	Remember bool `yaml:"remember"`

	// expanded is set by the first expandShorthands.
	expanded bool
}

// Directory is a node of the install tree.
type Directory struct { //nolint:govet // field order is the YAML key order
	ID           string       `yaml:"id"`
	Name         string       `yaml:"name"`
	Configurable bool         `yaml:"configurable"`
	Children     []*Directory `yaml:"children"`

	// origin is the YAML path of the value a generated node comes from.
	origin string
}

// Feature is a node of the feature tree.
type Feature struct { //nolint:govet // field order is the YAML key order
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	// Default is on, off or disabled: whether the feature is installed by
	// default. ApplyDefaults turns it into Level 1, 2 or 0.
	Default        string `yaml:"default"`
	Level          *int   `yaml:"level"`
	Display        string `yaml:"display"`
	Directory      string `yaml:"directory"`
	AllowAbsent    *bool  `yaml:"allowAbsent"`
	AllowAdvertise bool   `yaml:"allowAdvertise"`
	// Components are the feature's components: an id of an entry in
	// components, or a component defined in place. ApplyDefaults moves
	// components defined in place to Config.Components, so afterwards every
	// entry only carries an ID.
	Components []*Component `yaml:"components"`
	Features   []*Feature   `yaml:"features"`
	// Firewall are rules that follow the feature's state. ApplyDefaults
	// moves them to Config.Firewall, with a generated component that
	// carries the state.
	Firewall []*FirewallRule `yaml:"firewall"`

	// defaultedLevel and defaultedValue record the Level that ApplyDefaults
	// filled in, so that a later run can tell it from one set by the caller.
	defaultedLevel *int
	defaultedValue int
}

// levelIsDefault reports whether Level is still the one ApplyDefaults filled
// in.
func (f *Feature) levelIsDefault() bool {
	return f.Level != nil && f.Level == f.defaultedLevel && *f.Level == f.defaultedValue
}

// Component is a unit that is installed, repaired and removed together.
type Component struct { //nolint:govet // field order is the YAML key order
	ID          string           `yaml:"id"`
	Directory   string           `yaml:"directory"`
	GUID        string           `yaml:"guid"`
	Files       []*File          `yaml:"files"`
	Registry    []*RegistryValue `yaml:"registry"`
	Services    []*Service       `yaml:"services"`
	EventLog    []*EventSource   `yaml:"eventLog"`
	Environment []*Environment   `yaml:"environment"`
	// Purge deletes the component's directory with everything in it when
	// the product is uninstalled, but not on a major upgrade (RUN-26).
	Purge          *Purge `yaml:"purge"`
	CreateFolder   bool   `yaml:"createFolder"`
	Condition      string `yaml:"condition"`
	Permanent      bool   `yaml:"permanent"`
	NeverOverwrite bool   `yaml:"neverOverwrite"`
	// Preserve keeps the component's key file or registry value as the user
	// left it across major upgrades: it sets NeverOverwrite, and removeOld
	// defaults to afterInstallExecute (RUN-24).
	Preserve          bool         `yaml:"preserve"`
	SharedDLLRefCount bool         `yaml:"sharedDllRefCount"`
	Transitive        bool         `yaml:"transitive"`
	KeyPath           string       `yaml:"keyPath"`
	Bitness           string       `yaml:"bitness"`
	FolderPermissions *Permissions `yaml:"folderPermissions"`

	// ref marks a features[].components entry written as an id.
	ref bool
	// defined marks a component decoded from a YAML mapping.
	defined bool
	// origin is the YAML path of a component defined inside a feature.
	origin string
	// firewallFeature is the feature whose firewall rules a generated
	// component carries.
	firewallFeature string
	// carrierOf is that feature.
	carrierOf *Feature
}

// FirewallFeature returns the feature whose firewall rules the component
// carries, for a component generated from features[].firewall, else "".
func (c *Component) FirewallFeature() string {
	return c.firewallFeature
}

// Path returns the component's YAML path, given its index in
// Config.Components: components[i], or its place in a feature for a
// component defined there.
func (c *Component) Path(index int) string {
	if c.origin != "" {
		return c.origin
	}

	return "components[" + strconv.Itoa(index) + "]"
}

// Purge removes a directory tree on uninstall. "purge: true" is the short
// form of an empty mapping; "purge: false" sets Disabled.
type Purge struct {
	// Condition is ANDed with the uninstall condition, e.g. DELETE_DATA="1".
	Condition string `yaml:"condition"`
	Disabled  bool   `yaml:"-"`
}

// Enabled reports whether p asks for a purge.
func (p *Purge) Enabled() bool {
	return p != nil && !p.Disabled
}

// Permissions secures an installed object with an SDDL string through
// MsiLockPermissionsEx (specification section 6.2).
type Permissions struct {
	SDDL      string `yaml:"sddl"`
	Condition string `yaml:"condition"`
}

// File is a payload file.
type File struct { //nolint:govet // field order is the YAML key order
	Source                     string       `yaml:"source"`
	Name                       string       `yaml:"name"`
	ID                         string       `yaml:"id"`
	Version                    string       `yaml:"version"`
	Language                   string       `yaml:"language"`
	AllowUnversionedExecutable bool         `yaml:"allowUnversionedExecutable"`
	Vital                      *bool        `yaml:"vital"`
	Checksum                   bool         `yaml:"checksum"`
	ReadOnly                   bool         `yaml:"readOnly"`
	Hidden                     bool         `yaml:"hidden"`
	System                     bool         `yaml:"system"`
	Permissions                *Permissions `yaml:"permissions"`
	Shortcuts                  []*Shortcut  `yaml:"shortcuts"`

	// FS, when set, is where Source is read from, overriding Config.FS for
	// this file. AddTree sets it. It is never read from YAML.
	FS fs.FS `yaml:"-"`
}

// Shortcut is a non-advertised shortcut to the file it is listed under, in
// the Start menu, on the desktop or in any other folder.
type Shortcut struct { //nolint:govet // field order is the YAML key order
	ID string `yaml:"id"`
	// Name is the shortcut's name without ".lnk", default product.name.
	Name string `yaml:"name"`
	// Directory is a directory ID, a standard folder or a folder path,
	// default ProgramMenuFolder.
	Directory   string `yaml:"directory"`
	Arguments   string `yaml:"arguments"`
	Description string `yaml:"description"`
	// WorkingDirectory is a directory ID, default the file's directory.
	WorkingDirectory string `yaml:"workingDirectory"`
	// Icon names an entry of Config.Icons; empty shows the target's icon.
	Icon      string `yaml:"icon"`
	IconIndex int    `yaml:"iconIndex"`
	// Show is normal (default), maximized or minimized.
	Show string `yaml:"show"`
}

// RegistryValue is a typed registry value or key.
type RegistryValue struct { //nolint:govet // field order is the YAML key order
	ID                string       `yaml:"id"`
	Root              string       `yaml:"root"`
	Key               string       `yaml:"key"`
	Name              string       `yaml:"name"`
	Type              string       `yaml:"type"`
	Value             StringList   `yaml:"value"`
	Append            bool         `yaml:"append"`
	Prepend           bool         `yaml:"prepend"`
	RemoveOnUninstall bool         `yaml:"removeOnUninstall"`
	Permissions       *Permissions `yaml:"permissions"`
}

// Service is a Windows service installed from the component's key file.
type Service struct { //nolint:govet // field order is the YAML key order
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
	Arguments                        Arguments        `yaml:"arguments"`
	Dependencies                     []string         `yaml:"dependencies"`
	FailureActionsOnNonCrashFailures bool             `yaml:"failureActionsOnNonCrashFailures"`
	Control                          ServiceControl   `yaml:"control"`
	Recovery                         *ServiceRecovery `yaml:"recovery"`
	Permissions                      *Permissions     `yaml:"permissions"`
}

// Arguments is a service command line. Each entry is a piece of it; a piece
// with a condition is only added when the condition is true at install time.
type Arguments []Argument

// Argument is one piece of a service command line.
type Argument struct {
	Value     string `yaml:"value"`
	Condition string `yaml:"condition"`
}

// String joins the pieces that have no condition.
func (a Arguments) String() string {
	parts := make([]string, 0, len(a))

	for _, arg := range a {
		if arg.Condition == "" && arg.Value != "" {
			parts = append(parts, arg.Value)
		}
	}

	return strings.Join(parts, " ")
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
type ServiceRecovery struct { //nolint:govet // field order is the YAML key order
	ResetAfterDays int              `yaml:"resetAfterDays"`
	ResetAfter     *Duration        `yaml:"resetAfter"`
	Actions        []RecoveryAction `yaml:"actions"`
	RestartDelay   Duration         `yaml:"restartDelay"`
}

// RecoveryAction is one failure step: "restart", "none", or
// {restart: 5m} for a restart with its own delay.
type RecoveryAction struct {
	Restart *Duration `yaml:"restart"`
	Action  string    `yaml:"-"`
}

// EventSource is an event log source registered with plain Registry rows.
type EventSource struct { //nolint:govet // field order is the YAML key order
	Source         string `yaml:"source"`
	Log            string `yaml:"log"`
	MessageFile    string `yaml:"messageFile"`
	TypesSupported *int   `yaml:"typesSupported"`
}

// Environment is an environment variable change.
type Environment struct { //nolint:govet // field order is the YAML key order
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
type CustomAction struct { //nolint:govet // field order is the YAML key order
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

	// origin is the YAML path of the service argument an action was
	// generated from.
	origin string
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
type Placement struct { //nolint:govet // field order is the YAML key order
	Sequence  string `yaml:"sequence"`
	Before    string `yaml:"before"`
	After     string `yaml:"after"`
	At        int    `yaml:"at"`
	Condition string `yaml:"condition"`
	// When is install, upgrade, uninstall or maintenance: a condition that
	// is ANDed with Condition (see Placement.FullCondition).
	When string `yaml:"when"`
}

// FirewallRule is an inbound rule that follows a component's state.
type FirewallRule struct { //nolint:govet // field order is the YAML key order
	Name            string     `yaml:"name"`
	Description     string     `yaml:"description"`
	Component       string     `yaml:"component"`
	Program         string     `yaml:"program"`
	Protocol        string     `yaml:"protocol"`
	Port            string     `yaml:"port"`
	RemoteAddresses string     `yaml:"remoteAddresses"`
	Profiles        StringList `yaml:"profiles"`

	// origin is the YAML path of a rule written under a feature.
	origin string
	// ownComponent records that a rule under a feature named a component.
	ownComponent bool
	// carrier is the generated component of a rule under a feature.
	carrier *Component
}

// Path returns the YAML path of the rule at index in Config.Firewall.
func (r *FirewallRule) Path(index int) string {
	if r.origin != "" {
		return r.origin
	}

	return "firewall[" + strconv.Itoa(index) + "]"
}

// UI selects the built-in wizard and adds dialogs to it.
type UI struct {
	Wizard string `yaml:"wizard"`
	// License is a license file shown on a page with an accept check box
	// after the welcome page: RTF, or plain text that is converted to RTF.
	License string `yaml:"license"`
	// Launch adds a check box to the finish page that starts an installed
	// program when the user clicks Finish.
	Launch  *Launch   `yaml:"launch"`
	Theme   Theme     `yaml:"theme"`
	Dialogs []*Dialog `yaml:"dialogs"`
	// InstallDirPage adds a page after the welcome and license pages that
	// shows the install folder and lets the user change it.
	InstallDirPage bool `yaml:"installDirPage"`
}

// Launch is the finish page's check box that starts an installed program.
type Launch struct {
	// Checked sets the box's initial state; the default is checked.
	Checked *bool `yaml:"checked"`
	// File is the id of an installed file: files[].id, or its name when
	// it has no id.
	File      string `yaml:"file"`
	Arguments string `yaml:"arguments"`
	// Text is the check box label, "&" marks the access key; the default is
	// "&Launch [ProductName]".
	Text string `yaml:"text"`
}

// Theme replaces the wizard's pictures or the colours they are drawn in.
type Theme struct {
	// Banner is the picture at the top of the inner pages, shown at
	// 370×44 dialog units (493×58 pixels at 96 dpi).
	Banner string `yaml:"banner"`
	// Dialog is the background of the welcome and finish pages, shown at
	// 370×234 dialog units (493×312 pixels at 96 dpi). The page text is
	// drawn on its right part, from 135 units on.
	Dialog string      `yaml:"dialog"`
	Colors ThemeColors `yaml:"colors"`
}

// ThemeColors are the colours of the built-in pictures as #RRGGBB.
type ThemeColors struct {
	// Dark and Light are the two ends of the side panel gradient.
	Dark  string `yaml:"dark"`
	Light string `yaml:"light"`
	// Accent draws the banner stripe and the shapes on the side panel.
	Accent string `yaml:"accent"`
	// Paper is the background behind the page titles and texts.
	Paper string `yaml:"paper"`
}

// Dialog is a form dialog laid out by the builder.
type Dialog struct {
	ID          string        `yaml:"id"`
	Title       string        `yaml:"title"`
	Description string        `yaml:"description"`
	After       string        `yaml:"after"`
	Fields      []DialogField `yaml:"fields"`
}

// DialogField is one field bound to a property: a label and an edit field,
// or a check box.
type DialogField struct {
	Property string `yaml:"property"`
	Label    string `yaml:"label"`
	// Type is text (default), password or checkbox.
	Type string `yaml:"type"`
	// Value is what a checked checkbox sets the property to, default 1.
	Value string `yaml:"value"`
}

// Dialog field types.
const (
	FieldText     = "text"
	FieldPassword = "password"
	FieldCheckBox = "checkbox"
)

// CheckBoxValue returns the value a checked checkbox field sets.
func (f *DialogField) CheckBoxValue() string {
	if f.Value == "" {
		return "1"
	}

	return f.Value
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
