package wixconvert

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Namespaces of the WiX schemas, by the short name the converter uses.
// The v3 URIs and the v4 and later ones mean the same elements.
var namespaces = map[string]string{ //nolint:gochecknoglobals // lookup table
	"":                                     nsCore,
	"http://wixtoolset.org/schemas/v4/wxs": nsCore,
	"http://schemas.microsoft.com/wix/2006/wi":             nsCore,
	"http://wixtoolset.org/schemas/v4/wxs/util":            nsUtil,
	"http://schemas.microsoft.com/wix/UtilExtension":       nsUtil,
	"http://wixtoolset.org/schemas/v4/wxs/firewall":        nsFirewall,
	"http://schemas.microsoft.com/wix/FirewallExtension":   nsFirewall,
	"http://wixtoolset.org/schemas/v4/wxs/ui":              nsUI,
	"http://wixtoolset.org/schemas/v4/wxs/netfx":           nsOther,
	"http://schemas.microsoft.com/wix/NetFxExtension":      nsOther,
	"http://wixtoolset.org/schemas/v4/wxs/difxapp":         nsOther,
	"http://schemas.microsoft.com/wix/DifxAppExtension":    nsOther,
	"http://wixtoolset.org/schemas/v4/wxs/iis":             nsOther,
	"http://schemas.microsoft.com/wix/IIsExtension":        nsOther,
	"http://wixtoolset.org/schemas/v4/wxs/sql":             nsOther,
	"http://schemas.microsoft.com/wix/SqlExtension":        nsOther,
	"http://wixtoolset.org/schemas/v4/wxs/http":            nsOther,
	"http://wixtoolset.org/schemas/v4/wxs/dependency":      nsOther,
	"http://schemas.microsoft.com/wix/DependencyExtension": nsOther,
}

// Short namespace names.
const (
	nsCore     = ""
	nsUtil     = "util"
	nsFirewall = "fw"
	nsUI       = "ui"
	nsOther    = "ext"
)

// elem is an element of the source file.
type elem struct {
	attrs    map[string]string
	ns       string
	space    string
	name     string
	text     string
	order    []string
	kids     []*elem
	comments []string
	line     int
}

// qname is the element's name with its namespace prefix, as a WiX author
// would write it.
func (e *elem) qname() string {
	switch {
	case e.ns == nsCore:
		return e.name
	case e.ns == nsOther && e.space != "":
		return e.space + ":" + e.name
	default:
		return e.ns + ":" + e.name
	}
}

func (e *elem) attr(name string) string { return e.attrs[name] }

func (e *elem) has(name string) bool {
	_, ok := e.attrs[name]

	return ok
}

// yes reports whether a WiX yes/no attribute is yes.
func (e *elem) yes(name string) bool {
	switch strings.ToLower(e.attrs[name]) {
	case "yes", "true", "1":
		return true
	}

	return false
}

// no reports whether a WiX yes/no attribute is explicitly no.
func (e *elem) no(name string) bool {
	switch strings.ToLower(e.attrs[name]) {
	case "no", "false", "0":
		return true
	}

	return false
}

// is reports whether the element is the core element name.
func (e *elem) is(name string) bool { return e.ns == nsCore && e.name == name }

// procInst is a preprocessor instruction such as <?define?> or <?if?>.
type procInst struct {
	target string
	inst   string
	line   int
}

// document is a parsed source file.
type document struct {
	root  *elem
	procs []procInst
}

// parse reads a .wxs file into an element tree. Preprocessor instructions
// are kept aside: the converter cannot evaluate them.
func parse(r io.Reader) (*document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	tree := &treeBuilder{doc: &document{}}

	for {
		offset := dec.InputOffset()

		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}

		if err := tree.token(tok, 1+bytes.Count(data[:offset], []byte("\n"))); err != nil {
			return nil, err
		}
	}

	if tree.doc.root == nil {
		return nil, errors.New("parse: no root element")
	}

	return tree.doc, nil
}

// treeBuilder builds the element tree from the tokens of a file.
type treeBuilder struct {
	doc      *document
	stack    []*elem
	comments []string
}

func (b *treeBuilder) token(tok xml.Token, line int) error {
	switch tok := tok.(type) {
	case xml.StartElement:
		return b.start(newElem(tok, line, b.comments))
	case xml.EndElement:
		b.stack = b.stack[:len(b.stack)-1]
		b.comments = nil
	case xml.CharData:
		if len(b.stack) > 0 {
			b.stack[len(b.stack)-1].text += string(tok)
		}
	case xml.Comment:
		b.comments = append(b.comments, string(tok))
	case xml.ProcInst:
		if tok.Target != "xml" {
			b.doc.procs = append(b.doc.procs, procInst{target: tok.Target, inst: strings.TrimSpace(string(tok.Inst)), line: line})
		}
	}

	return nil
}

func (b *treeBuilder) start(elm *elem) error {
	b.comments = nil

	switch {
	case len(b.stack) > 0:
		parent := b.stack[len(b.stack)-1]
		parent.kids = append(parent.kids, elm)
	case b.doc.root != nil:
		return errors.New("parse: more than one root element")
	default:
		b.doc.root = elm
	}

	b.stack = append(b.stack, elm)

	return nil
}

func newElem(tok xml.StartElement, line int, comments []string) *elem {
	short, known := namespaces[tok.Name.Space]
	if !known {
		short = nsOther
	}

	elm := &elem{
		ns:       short,
		space:    tok.Name.Space,
		name:     tok.Name.Local,
		attrs:    make(map[string]string, len(tok.Attr)),
		line:     line,
		comments: comments,
	}

	if !known {
		elm.space = shortSpace(tok.Name.Space)
	}

	for _, attr := range tok.Attr {
		// Namespace declarations and the attributes of extensions
		// (util:..., bal:...) are not converted.
		if attr.Name.Space != "" || attr.Name.Local == "xmlns" {
			continue
		}

		elm.attrs[attr.Name.Local] = attr.Value
		elm.order = append(elm.order, attr.Name.Local)
	}

	return elm
}

// shortSpace names an unknown namespace by its last path segment.
func shortSpace(uri string) string {
	uri = strings.TrimRight(uri, "/")
	if i := strings.LastIndexAny(uri, "/:"); i >= 0 {
		return uri[i+1:]
	}

	return uri
}
