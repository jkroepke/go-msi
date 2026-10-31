package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// maxRendered caps the text a template writes. Without it a template such
// as {{ range 1000000000 }} could fill the memory.
const maxRendered = 16 << 20

// maxIndent caps the indent function, so one call cannot allocate a huge
// string. It does not bound a template's memory: a template is code.
const maxIndent = 1024

// errRenderedTooLarge is returned when a template writes more than
// maxRendered bytes.
var errRenderedTooLarge = fmt.Errorf("the template writes more than %d bytes", maxRendered)

// Render runs a document through Go's text/template, as Parse does when
// Options.Template is set. Variables are found as ${NAME} finds them: in
// Vars, then through LookupEnv, then in the file's vars block. {{ .NAME }}
// reads one and fails when it is not set. The functions are:
//
//   - var NAME [DEFAULT]: the variable, else DEFAULT; without a DEFAULT an
//     unset variable is an error.
//   - has NAME: whether the variable is set.
//   - quote S: S as a double-quoted YAML string, with ${ written as $${ so
//     that the ${NAME} substitution after rendering leaves it as it is.
//   - indent N S: S with every line after the first indented by N spaces.
//   - split SEP S: the parts of S between each SEP, for a range over a list
//     such as --var LANGS=en;de; an empty S has no parts, and SEP must not
//     be empty.
//
// The vars block is read from the raw text before rendering, so it must not
// use template syntax. The rendered text then goes through ${NAME}
// substitution like any file. The output is capped at 16 MiB, but a
// template is code: like a build script, only render files you trust, as a
// loop that writes nothing is not bounded.
func Render(data []byte, opts Options) ([]byte, error) {
	vars, lines, err := templateVars(data)
	if err != nil {
		return nil, err
	}

	var errs []error

	res := &resolver{
		opts: opts, vars: vars, lines: lines, resolved: make(map[string]string),
		docLeft: maxSubstituted, varsLeft: maxSubstituted, errs: &errs,
	}

	tmpl, err := template.New("msi.yaml").Option("missingkey=error").Funcs(templateFuncs(res.resolve, opts)).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("template: %w", templateErr(err, opts))
	}

	values := templateValues(opts, vars, lines)

	// Environment variables cannot be listed, so the names the template
	// uses are looked up too; a name that is not set stays out of the map
	// and fails as missing.
	for _, tree := range tmpl.Templates() {
		if tree.Tree != nil {
			fieldNames(tree.Root, func(name string) {
				if value, ok := res.resolve(name); ok {
					values[name] = value
				}
			})
		}
	}

	out := &limitedBuffer{left: maxRendered}
	err = tmpl.Execute(out, values)

	switch {
	case len(errs) > 0:
		return nil, errors.Join(errs...)
	case errors.Is(err, errRenderedTooLarge):
		return nil, fmt.Errorf("template: %w", errRenderedTooLarge)
	case err != nil:
		return nil, fmt.Errorf("template: %w", templateErr(err, opts))
	}

	return out.Bytes(), nil
}

// templateValues returns the data of a template: every --var and every
// entry of vars that resolves without an error, so {{ index . "NAME" }} and
// {{ $.NAME }} see them too. A broken entry is left out here and reported
// where the template uses it, as ${NAME} reports it.
func templateValues(opts Options, vars map[string]string, lines map[string]int) map[string]string {
	var scratch []error

	res := &resolver{
		opts: opts, vars: vars, lines: lines, resolved: make(map[string]string),
		docLeft: maxSubstituted, varsLeft: maxSubstituted, errs: &scratch,
	}

	values := make(map[string]string, len(opts.Vars)+len(vars))

	for name := range vars {
		before := len(scratch)
		if value, ok := res.resolve(name); ok && len(scratch) == before {
			values[name] = value
		}
	}

	maps.Copy(values, opts.Vars)

	return values
}

// varsKey finds the vars key at the top level of a raw file, also quoted
// or after a byte order mark.
var varsKey = regexp.MustCompile(`(?m)^(?:\x{FEFF})?(?:vars|"vars"|'vars')[ \t]*:`)

// varsBeforeRendering starts the errors about the vars block of a template.
const varsBeforeRendering = "with --template the vars block is read before rendering"

// templateVars reads the vars block of a template before it is rendered:
// the lines from "vars:" up to the next line that starts in column one.
func templateVars(data []byte) (map[string]string, map[string]int, error) {
	found := varsKey.FindAllIndex(data, -1)

	switch len(found) {
	case 0:
		return make(map[string]string), make(map[string]int), nil
	case 1:
	default:
		return nil, nil, fmt.Errorf("vars: lines %d and %d both start with vars:; %s, so there must be one, outside any {{ if }} or comment",
			lineOf(data, found[0][0]), lineOf(data, found[1][0]), varsBeforeRendering)
	}

	loc := found[0]
	block := data[loc[0]:blockEnd(data, loc[1])]

	if i := bytes.Index(block, []byte("{{")); i >= 0 {
		return nil, nil, fmt.Errorf("line %d: vars: %s and must not hold {{, also not in a comment", lineOf(data, loc[0]+i), varsBeforeRendering)
	}

	// Leading newlines keep the line numbers of the file.
	text := append(bytes.Repeat([]byte("\n"), lineOf(data, loc[0])-1), block...)

	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil {
		return nil, nil, fmt.Errorf("vars: %w (%s on its own, so anchors defined outside it are not visible)", err, varsBeforeRendering)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, errors.New("vars: the block is not a mapping")
	}

	return fileVars(doc.Content[0])
}

// lineOf returns the line of data that offset pos is on, counted from 1.
func lineOf(data []byte, pos int) int {
	return bytes.Count(data[:pos], []byte("\n")) + 1
}

// blockEnd returns where the top-level block that goes on at pos ends: at
// the next line that starts in column one with neither space, tab, comment
// nor the end of a flow collection, or at the end of data.
func blockEnd(data []byte, pos int) int {
	for {
		next := bytes.IndexByte(data[pos:], '\n')
		if next < 0 {
			return len(data)
		}

		pos += next + 1
		if pos < len(data) && !strings.ContainsRune(" \t#\r\n}]", rune(data[pos])) {
			return pos
		}
	}
}

// fieldNames calls use with the first name of every field reference such
// as .NAME below node.
func fieldNames(node parse.Node, use func(string)) { //nolint:cyclop // one case per node type
	switch node := node.(type) {
	case *parse.ListNode:
		if node != nil {
			for _, kid := range node.Nodes {
				fieldNames(kid, use)
			}
		}
	case *parse.ActionNode:
		fieldNames(node.Pipe, use)
	case *parse.PipeNode:
		if node != nil {
			for _, cmd := range node.Cmds {
				fieldNames(cmd, use)
			}
		}
	case *parse.CommandNode:
		for _, arg := range node.Args {
			fieldNames(arg, use)
		}
	case *parse.FieldNode:
		use(node.Ident[0])
	case *parse.VariableNode:
		if node.Ident[0] == "$" && len(node.Ident) > 1 {
			use(node.Ident[1])
		}
	case *parse.ChainNode:
		fieldNames(node.Node, use)
	case *parse.IfNode:
		branchNames(&node.BranchNode, use)
	case *parse.RangeNode:
		branchNames(&node.BranchNode, use)
	case *parse.WithNode:
		branchNames(&node.BranchNode, use)
	case *parse.TemplateNode:
		fieldNames(node.Pipe, use)
	}
}

func branchNames(node *parse.BranchNode, use func(string)) {
	fieldNames(node.Pipe, use)
	fieldNames(node.List, use)
	fieldNames(node.ElseList, use)
}

// templateFuncs returns the functions Render adds to Go's built-in ones.
func templateFuncs(lookup func(string) (string, bool), opts Options) template.FuncMap {
	return template.FuncMap{
		"var": func(name string, def ...string) (string, error) {
			if v, ok := lookup(name); ok {
				return v, nil
			}

			switch len(def) {
			case 0:
				return "", notSet(name, opts)
			case 1:
				return def[0], nil
			default:
				return "", errors.New("var takes a name and at most one default")
			}
		},
		"has": func(name string) bool {
			_, ok := lookup(name)

			return ok
		},
		"quote": quote,
		"indent": func(n int, s string) (string, error) {
			if n < 0 || n > maxIndent {
				return "", fmt.Errorf("indent: %d is not between 0 and %d", n, maxIndent)
			}

			return strings.ReplaceAll(s, "\n", "\n"+strings.Repeat(" ", n)), nil
		},
		"split": func(sep, list string) ([]string, error) {
			if sep == "" {
				return nil, errors.New("split: the separator is empty")
			}

			if list == "" {
				return nil, nil
			}

			return strings.Split(list, sep), nil
		},
	}
}

// notSet is the error for a variable that is not set.
func notSet(name string, opts Options) error {
	if opts.LookupEnv == nil {
		return fmt.Errorf("variable %s is not set; pass --var %s=VALUE, add it to vars, or pass --env to read it from the environment", name, name)
	}

	return fmt.Errorf("variable %s is not set; pass --var %s=VALUE, set it in the environment or add it to vars", name, name)
}

// quote writes value as a double-quoted YAML string. ${ becomes $${, which
// the ${NAME} substitution after rendering turns back into ${.
func quote(value string) (string, error) {
	// json.Marshal would replace invalid bytes silently.
	if !utf8.ValidString(value) {
		return "", errors.New("quote: the value is not valid UTF-8")
	}

	// A JSON string is a YAML double-quoted scalar; HTML escaping would
	// only make the rendered file harder to read.
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(strings.ReplaceAll(value, "${", "$${")); err != nil {
		return "", err //nolint:wrapcheck // encoding a string does not fail
	}

	// JSON leaves characters raw that YAML does not allow in a file.
	var out strings.Builder

	for _, r := range strings.TrimSuffix(buf.String(), "\n") {
		if unicode.Is(yamlPrintable, r) {
			out.WriteRune(r)
		} else {
			fmt.Fprintf(&out, `\u%04X`, r)
		}
	}

	return out.String(), nil
}

// yamlPrintable holds the characters YAML allows unescaped in a file
// (YAML 1.2, production c-printable).
var yamlPrintable = &unicode.RangeTable{ //nolint:gochecknoglobals // lookup table
	R16: []unicode.Range16{
		{Lo: 0x09, Hi: 0x0A, Stride: 1}, {Lo: 0x0D, Hi: 0x0D, Stride: 1},
		{Lo: 0x20, Hi: 0x7E, Stride: 1}, {Lo: 0x85, Hi: 0x85, Stride: 1},
		{Lo: 0xA0, Hi: 0xD7FF, Stride: 1}, {Lo: 0xE000, Hi: 0xFFFD, Stride: 1},
	},
	R32: []unicode.Range32{{Lo: 0x10000, Hi: 0x10FFFF, Stride: 1}},
}

// missingKey matches text/template's error for a name the data lacks.
var missingKey = regexp.MustCompile(`map has no entry for key "([^"]*)"`)

// templateErr drops the template name that text/template puts in every
// message, which Load already names, and says how to set a missing
// variable, so an error reads "template: line 3:5: at <.ARCH>: variable
// ARCH is not set; ...".
func templateErr(err error, opts Options) error {
	msg := strings.TrimPrefix(err.Error(), "template: ")
	msg = strings.TrimPrefix(msg, "msi.yaml:")
	msg = strings.Replace(msg, `executing "msi.yaml" at `, "at ", 1)
	msg = strings.ReplaceAll(msg, "started at msi.yaml:", "started at line ")

	if m := missingKey.FindStringSubmatch(msg); m != nil {
		msg = strings.Replace(msg, m[0], notSet(m[1], opts).Error(), 1)
	}

	if line, rest, ok := strings.Cut(msg, ":"); ok && line != "" && strings.Trim(line, "0123456789") == "" {
		return fmt.Errorf("line %s:%s", line, rest)
	}

	return errors.New(msg)
}

// limitedBuffer is a bytes.Buffer that refuses to grow past left bytes.
type limitedBuffer struct {
	bytes.Buffer

	left int
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.left {
		b.left = -1

		return 0, errRenderedTooLarge
	}

	b.left -= len(data)

	return b.Buffer.Write(data) //nolint:wrapcheck // bytes.Buffer does not fail
}
