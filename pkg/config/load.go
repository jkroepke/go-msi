package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// Options control how a file is loaded.
type Options struct {
	// Vars are the command-line variables. They override the file's vars.
	Vars map[string]string
	// LookupEnv, when set, is consulted after Vars and before the file's vars.
	LookupEnv func(string) (string, bool)
}

// Load reads, parses and defaults the file at path. It does not validate;
// call Validate on the result.
func Load(path string, opts Options) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err //nolint:wrapcheck // the error names the file
	}

	cfg, err := Parse(data, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	cfg.BaseDir = filepath.Dir(path)

	return cfg, nil
}

// Parse parses a YAML document, substitutes variables and applies defaults.
func Parse(data []byte, opts Options) (*Config, error) { //nolint:cyclop // one pass in a fixed order; splitting would scatter it
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err //nolint:wrapcheck // yaml errors carry the line
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("the document must be a mapping")
	}

	root := doc.Content[0]

	vars, err := fileVars(root)
	if err != nil {
		return nil, err
	}

	lookup := func(name string) (string, bool) {
		if v, ok := opts.Vars[name]; ok {
			return v, true
		}

		if opts.LookupEnv != nil {
			if v, ok := opts.LookupEnv(name); ok {
				return v, true
			}
		}

		v, ok := vars[name]

		return v, ok
	}

	var errs []error

	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "vars" {
			continue
		}

		substitute(root.Content[i+1], lookup, &errs)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	encoded, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, err //nolint:wrapcheck // re-encoding a decoded node
	}

	dec := yaml.NewDecoder(bytes.NewReader(encoded))
	dec.KnownFields(true)

	cfg := &Config{}
	if err := dec.Decode(cfg); err != nil {
		return nil, err //nolint:wrapcheck // yaml errors carry the line
	}

	if cfg.Schema != SchemaV1 {
		return nil, fmt.Errorf("schema: got %q, want %q", cfg.Schema, SchemaV1)
	}

	if err := emptyComponents(cfg); err != nil {
		return nil, err
	}

	ApplyDefaults(cfg)

	return cfg, nil
}

// emptyComponents rejects null entries in the components, features and
// firewall lists, which decode to nil pointers.
func emptyComponents(cfg *Config) error { //nolint:cyclop // one pass in a fixed order; splitting would scatter it
	var errs []error

	for i, comp := range cfg.Components {
		if comp == nil {
			errs = append(errs, fmt.Errorf("components[%d]: empty entry", i))
		}
	}

	for i, rule := range cfg.Firewall {
		if rule == nil {
			errs = append(errs, fmt.Errorf("firewall[%d]: empty entry", i))
		}
	}

	var walk func([]*Feature, string)

	walk = func(features []*Feature, prefix string) {
		for i, feature := range features {
			featurePath := fmt.Sprintf("%s[%d]", prefix, i)
			if feature == nil {
				errs = append(errs, fmt.Errorf("%s: empty entry", featurePath))

				continue
			}

			for j, comp := range feature.Components {
				if comp == nil {
					errs = append(errs, fmt.Errorf("%s.components[%d]: empty entry", featurePath, j))
				}
			}

			for j, rule := range feature.Firewall {
				if rule == nil {
					errs = append(errs, fmt.Errorf("%s.firewall[%d]: empty entry", featurePath, j))
				}
			}

			walk(feature.Features, featurePath+".features")
		}
	}

	walk(cfg.Features, "features")

	return errors.Join(errs...)
}

func fileVars(root *yaml.Node) (map[string]string, error) {
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value != "vars" {
			continue
		}

		vars := make(map[string]string)
		if err := root.Content[i+1].Decode(&vars); err != nil {
			return nil, fmt.Errorf("vars: %w", err)
		}

		return vars, nil
	}

	return make(map[string]string), nil
}

// yamlNulls are the plain scalars YAML 1.1 and 1.2 read as null, except the
// empty string, which leaves an optional field unset.
var yamlNulls = map[string]bool{"null": true, "Null": true, "NULL": true, "~": true} //nolint:gochecknoglobals // lookup table

var varPattern = regexp.MustCompile(`\$\$\{|\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// substitute replaces ${NAME} in every scalar value below node. Mapping keys
// are left alone. "$${" writes a literal "${".
func substitute(node *yaml.Node, lookup func(string) (string, bool), errs *[]error) { //nolint:cyclop // one pass in a fixed order; splitting would scatter it
	switch node.Kind {
	case yaml.ScalarNode:
		if !strings.Contains(node.Value, "${") {
			return
		}

		node.Value = varPattern.ReplaceAllStringFunc(node.Value, func(m string) string {
			if m == "$${" {
				return "${"
			}

			name := m[2 : len(m)-1]

			v, ok := lookup(name)
			if !ok {
				*errs = append(*errs, fmt.Errorf("line %d: undefined variable ${%s}", node.Line, name))
			}

			if !utf8.ValidString(v) {
				*errs = append(*errs, fmt.Errorf("line %d: variable ${%s} is not valid UTF-8", node.Line, name))
			}

			return v
		})

		// Resolve the tag again from the substituted text, so that an
		// integer field can take "${PORT}". A value that spells null stays
		// a string: dropping it would turn --var NAME=null into an empty
		// field.
		if node.Style == 0 {
			node.Tag = ""
			if yamlNulls[node.Value] {
				node.Tag = "!!str"
			}
		}
	case yaml.MappingNode:
		for i := 1; i < len(node.Content); i += 2 {
			substitute(node.Content[i], lookup, errs)
		}
	case yaml.SequenceNode, yaml.DocumentNode:
		for _, c := range node.Content {
			substitute(c, lookup, errs)
		}
	case yaml.AliasNode:
		// Aliases point at nodes that are substituted where they are defined.
	}
}
