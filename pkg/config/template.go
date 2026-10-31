package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"text/template"
	"unicode/utf8"
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
// Options.Template is set. The template's data maps each Options.Vars name
// to its value, so {{ .ARCH }} reads --var ARCH=...; a name that is not set
// is an error. The functions are:
//
//   - var NAME [DEFAULT]: the variable from Vars, then from LookupEnv, else
//     DEFAULT; without a DEFAULT an unset variable is an error.
//   - has NAME: whether var NAME finds a value.
//   - quote S: S as a double-quoted YAML string, with ${ written as $${ so
//     that the ${NAME} substitution after rendering leaves it as it is.
//   - indent N S: S with every line after the first indented by N spaces.
//   - split SEP S: the parts of S between each SEP, for a range over a list
//     such as --var LANGS=en;de; an empty S has no parts, and SEP must not
//     be empty.
//
// The rendered text then goes through ${NAME} substitution like any file.
//
// The file's vars block is not seen by the template: it is YAML that only
// exists after rendering. ${NAME} references pass through unchanged. The
// output is capped at 16 MiB, but a template is code: like a build script,
// only render files you trust, as a loop that writes nothing is not bounded.
func Render(data []byte, opts Options) ([]byte, error) {
	tmpl, err := template.New("msi.yaml").Option("missingkey=error").Funcs(templateFuncs(opts)).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("template: %w", templateErr(err))
	}

	out := &limitedBuffer{left: maxRendered}
	if err := tmpl.Execute(out, maps.Clone(opts.Vars)); err != nil {
		if errors.Is(err, errRenderedTooLarge) {
			return nil, fmt.Errorf("template: %w", errRenderedTooLarge)
		}

		return nil, fmt.Errorf("template: %w", templateErr(err))
	}

	return out.Bytes(), nil
}

// templateFuncs returns the functions Render adds to Go's built-in ones.
func templateFuncs(opts Options) template.FuncMap {
	lookup := opts.lookup

	return template.FuncMap{
		"var": func(name string, def ...string) (string, error) {
			if v, ok := lookup(name); ok {
				return v, nil
			}

			switch len(def) {
			case 0:
				return "", fmt.Errorf("variable %s is not set", name)
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

	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// lookup finds a variable for a template: from Vars, then from LookupEnv.
func (o Options) lookup(name string) (string, bool) {
	if v, ok := o.Vars[name]; ok {
		return v, true
	}

	if o.LookupEnv != nil {
		return o.LookupEnv(name)
	}

	return "", false
}

// templateErr drops the template name that text/template puts in every
// message, which Load already names, so an error reads
// "template: line 3:5: at <.ARCH>: ...".
func templateErr(err error) error {
	msg := strings.TrimPrefix(err.Error(), "template: ")
	msg = strings.TrimPrefix(msg, "msi.yaml:")
	msg = strings.Replace(msg, `executing "msi.yaml" at `, "at ", 1)

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
