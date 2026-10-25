package config

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Platform is the target architecture. Its value is the exact Template
// token: "Intel", "x64" or "Arm64".
type Platform string

// Platforms as written to the Summary Information Template.
const (
	PlatformX86   Platform = "Intel"
	PlatformX64   Platform = "x64"
	PlatformArm64 Platform = "Arm64"
)

var platformNames = map[string]Platform{
	"x86":     PlatformX86,
	"intel":   PlatformX86,
	"x64":     PlatformX64,
	"amd64":   PlatformX64,
	"arm64":   PlatformArm64,
	"aarch64": PlatformArm64,
}

// UnmarshalYAML accepts the user-facing names in any case and stores the
// exact token, since the engine compares tokens case-sensitively (SUM-02).
func (p *Platform) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}

	v, ok := platformNames[strings.ToLower(strings.TrimSpace(s))]
	if !ok {
		return fmt.Errorf("line %d: unknown platform %q, want x86, x64 or arm64", node.Line, s)
	}

	*p = v

	return nil
}

// Is64Bit reports whether components get the 64-bit attribute.
func (p Platform) Is64Bit() bool {
	return p == PlatformX64 || p == PlatformArm64
}

// Scope is the install context.
type Scope string

// Install contexts.
const (
	ScopePerMachine Scope = "perMachine"
	ScopePerUser    Scope = "perUser"
)

// UnmarshalYAML accepts perMachine and perUser in any case.
func (s *Scope) UnmarshalYAML(node *yaml.Node) error {
	var v string
	if err := node.Decode(&v); err != nil {
		return err
	}

	switch strings.ToLower(v) {
	case "permachine":
		*s = ScopePerMachine
	case "peruser":
		*s = ScopePerUser
	default:
		return fmt.Errorf("line %d: unknown scope %q, want perMachine or perUser", node.Line, v)
	}

	return nil
}

// StringList is a list that may be written as a single scalar.
type StringList []string

// UnmarshalYAML accepts a scalar or a sequence of scalars.
func (l *StringList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}

		*l = StringList{s}

		return nil
	}

	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}

	*l = list

	return nil
}

// Duration is a time.Duration written as "60s" or "1m".
type Duration time.Duration

// UnmarshalYAML parses a Go duration string.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}

	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("line %d: %w", node.Line, err)
	}

	*d = Duration(v)

	return nil
}

// UnmarshalYAML accepts a scalar action name or a one-key {restart: delay} map.
func (a *RecoveryAction) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if err := node.Decode(&a.Action); err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}

		return nil
	}

	if node.Kind != yaml.MappingNode || len(node.Content) != 2 || node.Content[0].Value != "restart" {
		return fmt.Errorf("line %d: a recovery action is restart, none or {restart: <delay>}", node.Line)
	}

	var delay Duration
	if err := node.Content[1].Decode(&delay); err != nil {
		return fmt.Errorf("line %d: %w", node.Line, err)
	}

	a.Action = "restart"
	a.Restart = &delay

	return nil
}

// MarshalYAML writes the form UnmarshalYAML reads, so that the product code
// derived from the marshalled config sees every action.
func (a RecoveryAction) MarshalYAML() (any, error) {
	if a.Restart == nil {
		return a.Action, nil
	}

	return map[string]string{"restart": time.Duration(*a.Restart).String()}, nil
}

// Property is one Property table entry and its flags.
type Property struct {
	Value  string `yaml:"value"`
	Secure *bool  `yaml:"secure"`
	Hidden bool   `yaml:"hidden"`
}

// Properties maps property names to their settings.
type Properties map[string]*Property

// UnmarshalYAML accepts "NAME: value" as a short form of "NAME: {value: value}".
func (p *Properties) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: properties must be a mapping", node.Line)
	}

	out := make(Properties, len(node.Content)/2)

	for i := 0; i < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if _, dup := out[name]; dup {
			return fmt.Errorf("line %d: property %s defined twice", node.Content[i].Line, name)
		}

		prop := &Property{}

		switch value.Kind {
		case yaml.ScalarNode:
			if value.Tag != "!!null" {
				prop.Value = value.Value
			}
		case yaml.MappingNode:
			if err := decodeStrict(value, prop); err != nil {
				return err
			}
		default:
			return fmt.Errorf("line %d: property %s must be a value or a mapping", value.Line, name)
		}

		out[name] = prop
	}

	*p = out

	return nil
}

// Names returns the property names in sorted order.
func (p Properties) Names() []string {
	names := make([]string, 0, len(p))
	for name := range p {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// decodeStrict decodes node into v and rejects unknown keys. yaml.Node.Decode
// does not honour Decoder.KnownFields, so mappings decoded by hand go through
// a re-encoded document.
func decodeStrict(node *yaml.Node, v any) error {
	data, err := yaml.Marshal(node)
	if err != nil {
		return err
	}

	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)

	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("line %d: %w", node.Line, err)
	}

	return nil
}
