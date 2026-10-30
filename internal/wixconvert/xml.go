package wixconvert

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
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
	// skipped is set on an elseif or else whose branch was left out.
	skipped bool
}

// document is a parsed source file.
type document struct {
	root  *elem
	procs []procInst
	// extAttrs are the extension attributes on WiX elements, which the
	// converter does not read.
	extAttrs []extAttr
}

// extAttr is an extension attribute, such as util:Foo on a File.
type extAttr struct {
	elem string
	name string
	line int
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

	// line counts the newlines up to the token, without rescanning the
	// file for every token.
	line, counted := 1, int64(0)

	for {
		offset := dec.InputOffset()
		line += bytes.Count(data[counted:offset], []byte("\n"))
		counted = offset

		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}

		if err := tree.token(tok, line); err != nil {
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
	// branches has one entry per open <?if?>: whether the branch being
	// read is one the converter leaves out.
	branches []bool
	// skipDepth counts the open elements of a left-out branch.
	skipDepth int
}

// skipping reports whether the tokens belong to a left-out branch.
func (b *treeBuilder) skipping() bool {
	return slices.Contains(b.branches, true)
}

// procInst records a processing instruction. Only the first branch of an
// <?if?> is read: converting every branch makes duplicate IDs, and the
// first branch is usually the default build.
func (b *treeBuilder) procInst(tok xml.ProcInst, line int) {
	if tok.Target == "xml" {
		return
	}

	proc := procInst{target: tok.Target, inst: strings.TrimSpace(string(tok.Inst)), line: line}

	switch proc.target {
	case "if", "ifdef", "ifndef":
		b.branches = append(b.branches, false)
	case "elseif", "else":
		if len(b.branches) > 0 {
			proc.skipped = !b.skipping()
			b.branches[len(b.branches)-1] = true
		}
	case "endif":
		if len(b.branches) > 0 {
			b.branches = b.branches[:len(b.branches)-1]
		}
	}

	b.doc.procs = append(b.doc.procs, proc)
}

func (b *treeBuilder) token(tok xml.Token, line int) error { //nolint:cyclop // one branch per token type
	if b.skip(tok) {
		return nil
	}

	switch tok := tok.(type) {
	case xml.StartElement:
		elm := newElem(tok, line, b.comments)
		for _, attr := range tok.Attr {
			if attr.Name.Space != "" && attr.Name.Space != "xmlns" && !strings.HasPrefix(attr.Name.Space, "http://www.w3.org/") {
				name := prefix(attr.Name.Space) + ":" + attr.Name.Local
				b.doc.extAttrs = append(b.doc.extAttrs, extAttr{elem: elm.qname(), name: name, line: line})
			}
		}

		return b.start(elm)
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
		b.procInst(tok, line)
	}

	return nil
}

// skip drops a token of a left-out branch and reports whether it did. The
// end of an element that began before the branch is kept.
func (b *treeBuilder) skip(tok xml.Token) bool {
	if b.skipDepth == 0 && !b.skipping() {
		return false
	}

	switch tok.(type) {
	case xml.ProcInst:
		return false
	case xml.StartElement:
		b.skipDepth++
	case xml.EndElement:
		if b.skipDepth == 0 {
			return false
		}

		b.skipDepth--
	}

	b.comments = nil

	return true
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

// prefix names a namespace as a WiX author writes it, such as util.
func prefix(uri string) string {
	if short, ok := namespaces[uri]; ok && short != nsOther {
		return short
	}

	return shortSpace(uri)
}

// shortSpace names an unknown namespace by its last path segment.
func shortSpace(uri string) string {
	uri = strings.TrimRight(uri, "/")
	if i := strings.LastIndexAny(uri, "/:"); i >= 0 {
		return uri[i+1:]
	}

	return uri
}
