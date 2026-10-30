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

	var errs []error

	// A value from the file's vars may use other variables, which are
	// expanded like any other string; values from the command line and the
	// environment are taken as they are.
	resolving := make(map[string]bool)
	resolved := make(map[string]string)

	var lookup func(string) (string, bool)

	lookup = func(name string) (string, bool) {
		if v, ok := opts.Vars[name]; ok {
			return v, true
		}

		if opts.LookupEnv != nil {
			if v, ok := opts.LookupEnv(name); ok {
				return v, true
			}
		}

		if v, ok := resolved[name]; ok {
			return v, true
		}

		value, ok := vars[name]
		if !ok {
			return "", false
		}

		if resolving[name] {
			errs = append(errs, fmt.Errorf("vars.%s: refers to itself through ${%s}", name, name))

			return "", true
		}

		resolving[name] = true
		value = expand(value, "vars."+name, lookup, &errs)
		resolving[name] = false
		resolved[name] = value

		return value, true
	}

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

	if cfg.Schema == legacySchemaV1 {
		return nil, fmt.Errorf("schema: %q is the identifier from before the rename to msi-builder; write %q", cfg.Schema, SchemaV1)
	}

	if cfg.Schema != SchemaV1 {
		return nil, fmt.Errorf("schema: got %q, want %q", cfg.Schema, SchemaV1)
	}

	if paths := nullEntries(cfg); len(paths) > 0 {
		errs := make([]error, 0, len(paths))
		for _, path := range paths {
			errs = append(errs, fmt.Errorf("%s: empty entry", path))
		}

		return nil, errors.Join(errs...)
	}

	ApplyDefaults(cfg)

	return cfg, nil
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

// expand replaces ${NAME} in s; "$${" writes a literal "${". where names
// the place for an error.
func expand(s, where string, lookup func(string) (string, bool), errs *[]error) string {
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		if m == "$${" {
			return "${"
		}

		name := m[2 : len(m)-1]

		value, ok := lookup(name)
		if !ok {
			*errs = append(*errs, fmt.Errorf("%s: undefined variable ${%s}", where, name))
		}

		if !utf8.ValidString(value) {
			*errs = append(*errs, fmt.Errorf("%s: variable ${%s} is not valid UTF-8", where, name))
		}

		return value
	})
}

// substitute replaces ${NAME} in every scalar value below node. Mapping keys
// are left alone. "$${" writes a literal "${".
func substitute(node *yaml.Node, lookup func(string) (string, bool), errs *[]error) {
	switch node.Kind {
	case yaml.ScalarNode:
		if !strings.Contains(node.Value, "${") {
			return
		}

		node.Value = expand(node.Value, fmt.Sprintf("line %d", node.Line), lookup, errs)

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
