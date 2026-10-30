// Package wixconvert converts a WiX source file (.wxs) to an msi-builder
// YAML file. The conversion is best effort: it maps the elements that
// docs/Migrating from WiX.md describes and writes a "# TODO" comment for
// everything it cannot convert, such as preprocessor conditions, includes,
// bind variables, hand-built dialogs and extensions without a counterpart.
// The result is a starting point that a person reviews; it is not meant to
// build the same package byte for byte.
package wixconvert

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"strings"

	"go.yaml.in/yaml/v3"
)

// Note is something the converter could not convert, with the line of the
// source file it comes from (0 when it has none).
type Note struct {
	Message string
	Line    int
}

func (n Note) String() string {
	if n.Line == 0 {
		return n.Message
	}

	return fmt.Sprintf("line %d: %s", n.Line, n.Message)
}

// Result is a converted file.
type Result struct {
	// YAML is the msi.yaml text.
	YAML []byte
	// Notes are the TODO comments in YAML, in source order.
	Notes []Note
	// Vars are the ${NAME} variables that YAML references and does not
	// define; the WiX file took them from wix build -d or the
	// environment, msi-builder build takes them with --var.
	Vars []string
}

// Options tune the conversion.
type Options struct {
	// Source names the input in the header comment, such as app.wxs.
	Source string
}

// Convert reads a .wxs file and returns the msi.yaml that describes the
// same package as far as msi-builder can express it.
func Convert(r io.Reader, opts Options) (*Result, error) {
	doc, err := parse(r)
	if err != nil {
		return nil, err
	}

	if !doc.root.is("Wix") {
		return nil, fmt.Errorf("the root element is <%s>, not <Wix>", doc.root.qname())
	}

	conv := newConverter(doc)
	if err := conv.index(); err != nil {
		return nil, err
	}

	conv.emit()

	text, err := conv.render(opts)
	if err != nil {
		return nil, err
	}

	notes := append([]Note(nil), conv.notes...)
	slices.SortStableFunc(notes, func(a, b Note) int { return cmp.Compare(a.Line, b.Line) })

	return &Result{YAML: text, Notes: notes, Vars: conv.undefinedVars()}, nil
}

// converter holds the indexed source file and the output.
type converter struct {
	compByID     map[string]*comp
	doc          *document
	wixVars      map[string]string
	dirs         map[string]*dir
	wixUI        *elem
	out          *omap
	pkg          *elem
	major        *elem
	summary      *elem
	usedVars     map[string]bool
	groups       map[string]*group
	actions      map[string]*elem
	needDirs     map[string]bool
	renames      map[string]string
	installDir   *dir
	uiSet        string
	platform     string
	launchAction string
	properties   []*elem
	actionOrder  []*elem
	actionRefs   []*elem
	binaries     []*elem
	upgrades     []*elem
	media        []*elem
	launches     []*elem
	icons        []*elem
	defines      [][2]string
	sequences    []*elem
	setters      []*elem
	structure    map[string]func(c *converter, e *elem, sc scope)
	features     []*feature
	comps        []*comp
	notes        []Note
	header       []string
	dirOrder     []*dir
	files        []*fileRef
	pending      []pendingShortcut
	pendingTypes []pendingType
	anonymous    int
	uiLine       int
	v3           bool
}

type dir struct {
	parent *dir
	id     string
	name   string
	line   int
	std    bool
}

type comp struct {
	e           *elem
	out         *omap
	id          string
	dir         string
	inFeature   bool
	dropIfEmpty bool
}

type group struct {
	members []member
}

// member is an entry of a feature or component group: a component or a
// reference to a component or group.
type member struct {
	comp  *comp
	ref   string // ComponentRef Id
	group string // ComponentGroupRef Id
}

type feature struct {
	e       *elem
	kids    []*feature
	members []member
}

func newConverter(doc *document) *converter {
	return &converter{
		doc:       doc,
		dirs:      make(map[string]*dir),
		compByID:  make(map[string]*comp),
		groups:    make(map[string]*group),
		actions:   make(map[string]*elem),
		wixVars:   make(map[string]string),
		renames:   make(map[string]string),
		needDirs:  make(map[string]bool),
		usedVars:  make(map[string]bool),
		out:       newMap(),
		structure: structureHandlers(),
	}
}

// todof records a note as a comment above key in target, above target itself when
// key is empty, or in the file header when target is nil. A note that was
// already recorded is not repeated.
func (c *converter) todof(target *omap, key string, line int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	note := Note{Line: line, Message: msg}

	if slices.Contains(c.notes, note) {
		return
	}

	c.notes = append(c.notes, note)

	text := "TODO: " + msg
	if line != 0 {
		text = fmt.Sprintf("TODO(line %d): %s", line, msg)
	}

	if target == nil {
		c.header = append(c.header, text)

		return
	}

	if key == "" {
		target.head = append(target.head, text)

		return
	}

	target.comment(key, text)
}

// index walks the file once and records the elements by kind.
func (c *converter) index() error {
	c.preprocessor()

	var packages []*elem

	for _, elm := range c.doc.root.kids {
		switch {
		case elm.is("Package"), elm.is("Product"):
			packages = append(packages, elm)
		case elm.is("Fragment"):
		case elm.is("Bundle"), elm.is("Module"), elm.is("Patch"), elm.is("PatchCreation"):
			return fmt.Errorf("line %d: <%s> is not a package; msi-builder builds packages (.msi) only", elm.line, elm.name)
		}
	}

	if len(packages) != 1 {
		return errors.New("the file needs exactly one <Package> (WiX v3: <Product>)")
	}

	c.pkg = packages[0]
	c.v3 = c.pkg.is("Product")

	for _, e := range c.doc.root.kids {
		switch {
		case e == c.pkg, e.is("Fragment"):
			c.walk(e, scope{})
		default:
			c.todof(nil, "", e.line, "<%s> has no counterpart", e.qname())
		}
	}

	c.chooseInstallDir()
	c.choosePlatform()

	return nil
}

// scope is where an element sits: its directory, component group and
// feature.
type scope struct {
	group   *group
	feature *feature
	dir     string
}

func (c *converter) walk(parent *elem, within scope) {
	for _, elm := range parent.kids {
		if elm.ns != nsCore {
			c.walkExtension(elm)

			continue
		}

		if c.walkElem(elm, within) {
			continue
		}

		if hint, ok := unsupported[elm.name]; ok {
			c.todof(nil, "", elm.line, "<%s> is not supported: %s", elm.qname(), hint)

			continue
		}

		c.todof(nil, "", elm.line, "<%s> has no counterpart", elm.qname())
	}
}

// structureHandlers index directories, components, groups and features.
// They walk the elements they contain, so they are built by a function: a
// package variable would refer to itself through walk.
func structureHandlers() map[string]func(c *converter, e *elem, sc scope) {
	return map[string]func(c *converter, e *elem, sc scope){
		"StandardDirectory": func(c *converter, e *elem, sc scope) {
			c.addDir(e, nil, true)
			c.walk(e, scope{dir: e.attr("Id"), group: sc.group, feature: sc.feature})
		},
		"Directory": func(c *converter, e *elem, sc scope) {
			added := c.addDir(e, c.dirs[sc.dir], false)
			c.walk(e, scope{dir: added.id, group: sc.group, feature: sc.feature})
		},
		"DirectoryRef": func(c *converter, e *elem, sc scope) {
			c.walk(e, scope{dir: e.attr("Id"), group: sc.group, feature: sc.feature})
		},
		"ComponentGroup": func(conv *converter, elm *elem, sc scope) {
			inner := scope{dir: sc.dir, group: conv.group(elm.attr("Id"))}
			if folder := elm.attr(attrDirectory); folder != "" {
				inner.dir = folder
			}

			conv.walk(elm, inner)
		},
		"Component":         (*converter).addComp,
		"ComponentRef":      func(c *converter, e *elem, sc scope) { c.addMember(sc, member{ref: e.attr("Id")}) },
		"ComponentGroupRef": func(c *converter, e *elem, sc scope) { c.addMember(sc, member{group: e.attr("Id")}) },
		"Feature": func(conv *converter, elm *elem, within scope) {
			feat := &feature{e: elm}
			if within.feature != nil {
				within.feature.kids = append(within.feature.kids, feat)
			} else {
				conv.features = append(conv.features, feat)
			}

			conv.walk(elm, scope{dir: within.dir, feature: feat})
		},
		// References only pull fragments in; the converter reads every
		// fragment of the file anyway.
		"FeatureRef":      func(*converter, *elem, scope) {},
		"FeatureGroupRef": func(*converter, *elem, scope) {},
		"PropertyRef":     func(*converter, *elem, scope) {},
		"FeatureGroup":    func(c *converter, e *elem, sc scope) { c.walk(e, sc) },
	}
}

// packageElems index the elements that belong to the package as a whole.
var packageElems = map[string]func(c *converter, e *elem){ //nolint:gochecknoglobals // dispatch table
	"Property": func(c *converter, e *elem) { c.properties = append(c.properties, e) },
	"CustomAction": func(c *converter, e *elem) {
		c.actions[e.attr("Id")] = e
		c.actionOrder = append(c.actionOrder, e)
	},
	"SetProperty":              func(c *converter, e *elem) { c.setters = append(c.setters, e) },
	"SetDirectory":             func(c *converter, e *elem) { c.setters = append(c.setters, e) },
	"InstallExecuteSequence":   func(c *converter, e *elem) { c.sequences = append(c.sequences, e) },
	"InstallUISequence":        func(c *converter, e *elem) { c.sequences = append(c.sequences, e) },
	"AdminExecuteSequence":     func(c *converter, e *elem) { c.sequences = append(c.sequences, e) },
	"AdminUISequence":          func(c *converter, e *elem) { c.sequences = append(c.sequences, e) },
	"AdvertiseExecuteSequence": func(c *converter, e *elem) { c.sequences = append(c.sequences, e) },
	"Icon":                     func(c *converter, e *elem) { c.icons = append(c.icons, e) },
	"Binary":                   func(c *converter, e *elem) { c.binaries = append(c.binaries, e) },
	"WixVariable":              func(c *converter, e *elem) { c.wixVars[e.attr("Id")] = e.attr("Value") },
	"UI":                       (*converter).walkUI,
	"UIRef":                    (*converter).uiRef,
	"CustomActionRef":          func(c *converter, e *elem) { c.actionRefs = append(c.actionRefs, e) },
	"MajorUpgrade":             func(c *converter, e *elem) { c.major = e },
	"Upgrade":                  func(c *converter, e *elem) { c.upgrades = append(c.upgrades, e) },
	"Media":                    func(c *converter, e *elem) { c.media = append(c.media, e) },
	"MediaTemplate":            func(c *converter, e *elem) { c.media = append(c.media, e) },
	"Launch":                   func(c *converter, e *elem) { c.launches = append(c.launches, e) },
	"Condition":                func(c *converter, e *elem) { c.launches = append(c.launches, e) },
	"SummaryInformation":       func(c *converter, e *elem) { c.summary = e },
}

// walkElem indexes one core element and reports whether it knows it.
func (c *converter) walkElem(elm *elem, sc scope) bool {
	if fn, ok := c.structure[elm.name]; ok {
		fn(c, elm, sc)

		return true
	}

	if fn, ok := packageElems[elm.name]; ok {
		fn(c, elm)

		return true
	}

	// The v3 Package element is the summary information.
	if elm.name == "Package" && c.v3 {
		c.summary = elm

		return true
	}

	return false
}

func (c *converter) walkExtension(e *elem) {
	switch {
	case e.ns == nsUI && e.name == "WixUI":
		c.wixUI = e
	default:
		c.todof(nil, "", e.line, "<%s> has no counterpart", e.qname())
	}
}

func (c *converter) addDir(elm *elem, parent *dir, standard bool) *dir {
	dirID := elm.attr("Id")
	if dirID == "" {
		dirID = fmt.Sprintf("dir_line%d", elm.line)
	}

	if d, ok := c.dirs[dirID]; ok {
		return d
	}

	folder := &dir{id: dirID, name: elm.attr("Name"), parent: parent, line: elm.line}

	switch {
	case dirID == "TARGETDIR":
		folder.std, folder.name, folder.parent = true, "", nil
	case standard, isStandardFolder(dirID):
		folder.std, folder.name = true, ""
		folder.parent = nil
	}

	c.dirs[dirID] = folder
	c.dirOrder = append(c.dirOrder, folder)

	return folder
}

func (c *converter) group(id string) *group {
	grp, ok := c.groups[id]
	if !ok {
		grp = &group{}
		c.groups[id] = grp
	}

	return grp
}

func (c *converter) addComp(e *elem, within scope) {
	comp := &comp{e: e, id: e.attr("Id"), dir: e.attr(attrDirectory)}
	if comp.dir == "" {
		comp.dir = within.dir
	}

	c.comps = append(c.comps, comp)
	if comp.id != "" {
		c.compByID[comp.id] = comp
	}

	c.addMember(within, member{comp: comp})
}

func (c *converter) addMember(sc scope, entry member) {
	switch {
	case sc.feature != nil:
		sc.feature.members = append(sc.feature.members, entry)
	case sc.group != nil:
		sc.group.members = append(sc.group.members, entry)
	}
}

// preprocessor turns <?define?> into vars and notes everything else.
func (c *converter) preprocessor() {
	for _, proc := range c.doc.procs {
		switch proc.target {
		case "define":
			name, value, ok := strings.Cut(proc.inst, "=")
			if !ok {
				c.todof(nil, "", proc.line, "<?define %s?> has no value", proc.inst)

				continue
			}

			value = strings.Trim(strings.TrimSpace(value), `"`)
			c.defines = append(c.defines, [2]string{varName(strings.TrimSpace(name)), value})
		case "include":
			c.todof(nil, "", proc.line, "<?include %s?>: convert the included file and merge it in by hand", proc.inst)
		case "if", "ifdef", "ifndef", "elseif", "else", "foreach":
			c.todof(nil, "", proc.line,
				"<?%s %s?>: preprocessor conditions and loops are not evaluated; the elements of every branch were converted",
				proc.target, proc.inst)
		case "endif", "endforeach":
		default:
			c.todof(nil, "", proc.line, "<?%s %s?> has no counterpart", proc.target, proc.inst)
		}
	}
}

// undefinedVars are the variables the output references and does not
// define, in name order.
func (c *converter) undefinedVars() []string {
	var undefined []string

	for _, name := range slices.Sorted(maps.Keys(c.usedVars)) {
		if !slices.ContainsFunc(c.defines, func(d [2]string) bool { return d[0] == name }) {
			undefined = append(undefined, name)
		}
	}

	return undefined
}

// render writes the output as YAML.
func (c *converter) render(opts Options) ([]byte, error) {
	source := opts.Source
	if source == "" {
		source = "a WiX file"
	}

	head := []string{
		"Converted from " + source + " by msi-builder convert.",
		"Review it, resolve the TODO comments, then run msi-builder validate.",
	}
	if undefined := c.undefinedVars(); len(undefined) > 0 {
		head = append(head, "WiX variables became ${NAME} references; pass them with --var NAME=value: "+strings.Join(undefined, ", ")+".")
	}

	head = append(head, c.header...)

	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{c.out.node()}}
	doc.Content[0].Content[0].HeadComment = commentText(head)

	var buf bytes.Buffer

	buf.WriteString("# yaml-language-server: $schema=https://raw.githubusercontent.com/jkroepke/msi-builder/main/pkg/config/schema.json\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	return buf.Bytes(), nil
}
