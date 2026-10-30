package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

	vars, lines, err := fileVars(root)
	if err != nil {
		return nil, err
	}

	var errs []error

	r := &resolver{opts: opts, vars: vars, lines: lines, resolved: make(map[string]string), budget: maxSubstituted, errs: &errs}

	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "vars" {
			continue
		}

		substitute(root.Content[i+1], r.lookup, &errs)
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

// fileVars returns the file's vars and the line of each value.
func fileVars(root *yaml.Node) (map[string]string, map[string]int, error) {
	lines := make(map[string]int)

	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value != "vars" {
			continue
		}

		node := root.Content[i+1]

		vars := make(map[string]string)
		if err := node.Decode(&vars); err != nil {
			return nil, nil, fmt.Errorf("vars: %w", err)
		}

		if node.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(node.Content); j += 2 {
				lines[node.Content[j].Value] = node.Content[j+1].Line
			}
		}

		return vars, lines, nil
	}

	return make(map[string]string), lines, nil
}

// resolver looks up variables for substitution. A value from the file's
// vars may use other variables, which are expanded like any other string;
// values from the command line and the environment are taken as they are.
// Every value handed out counts against one budget, so nested vars cannot
// blow up exponentially.
type resolver struct {
	opts     Options
	vars     map[string]string
	lines    map[string]int
	resolved map[string]string
	errs     *[]error
	stack    []string
	budget   int
}

func (r *resolver) lookup(name string) (string, bool) {
	value, ok := r.resolve(name)
	if r.budget < 0 {
		return "", ok
	}

	r.budget -= len(value)
	if r.budget < 0 {
		*r.errs = append(*r.errs, fmt.Errorf("variables expand to more than %d bytes in total", maxSubstituted))

		return "", ok
	}

	return value, ok
}

func (r *resolver) resolve(name string) (string, bool) {
	if v, ok := r.opts.Vars[name]; ok {
		return v, true
	}

	if r.opts.LookupEnv != nil {
		if v, ok := r.opts.LookupEnv(name); ok {
			return v, true
		}
	}

	if v, ok := r.resolved[name]; ok {
		return v, true
	}

	value, ok := r.vars[name]
	if !ok {
		return "", false
	}

	// The cycle is reported on the entry that closes it.
	if i := slices.Index(r.stack, name); i >= 0 {
		chain := "${" + strings.Join(append(slices.Clone(r.stack[i:]), name), "} -> ${") + "}"
		*r.errs = append(*r.errs, fmt.Errorf("%s: %s is a cycle", r.where(r.stack[len(r.stack)-1]), chain))

		return "", true
	}

	r.stack = append(r.stack, name)
	before := len(*r.errs)
	value = expand(value, r.where(name), r.lookup, r.errs)
	r.stack = r.stack[:len(r.stack)-1]

	// A broken entry is reported once, here, and not again where it is used.
	if len(*r.errs) > before {
		value = ""
	}

	r.resolved[name] = value

	return value, true
}

func (r *resolver) where(name string) string {
	return fmt.Sprintf("line %d: vars.%s", r.lines[name], name)
}

// maxSubstituted caps the text that variables put into one document.
const maxSubstituted = 16 << 20

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
