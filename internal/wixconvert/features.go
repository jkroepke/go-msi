package wixconvert

import "slices"

import "strings"

func (c *converter) eachFeature(fn func(*feature)) {
	var walk func([]*feature)

	walk = func(fs []*feature) {
		for _, f := range fs {
			fn(f)
			walk(f.kids)
		}
	}

	walk(c.features)
}

// members resolves the components of a member list.
func (c *converter) members(ms []member, seen map[string]bool) []*comp {
	var out []*comp

	for _, member := range ms {
		switch {
		case member.comp != nil:
			out = append(out, member.comp)
		case member.ref != "":
			if cp := c.compByID[member.ref]; cp != nil {
				out = append(out, cp)
			} else {
				c.todof(nil, "", 0, "ComponentRef %s names no component of this file", member.ref)
			}
		case member.group != "":
			group := c.groups[member.group]
			if group == nil {
				c.todof(nil, "", 0, "ComponentGroupRef %s names no component group of this file", member.group)

				continue
			}

			if !seen[member.group] {
				seen[member.group] = true
				out = append(out, c.members(group.members, seen)...)
			}
		}
	}

	return out
}

// needFeatures reports whether the features say more than msi-builder's
// one default feature does.
func (c *converter) needFeatures() bool {
	if len(c.features) != 1 || c.chooseFeatures() {
		return len(c.features) > 0
	}

	first := c.features[0].e
	if len(c.features[0].kids) > 0 {
		return true
	}

	if slices.ContainsFunc([]string{"Title", "Description", "Display", "AllowAbsent", "Absent"}, first.has) {
		return true
	}

	if l := first.attr("Level"); l != "" && l != "1" {
		return true
	}

	if d := first.attr("ConfigurableDirectory"); d != "" && (c.installDir == nil || d != c.installDir.id) {
		return true
	}

	return false
}

func (c *converter) emitFeatures() {
	c.eachFeature(func(f *feature) {
		for _, cp := range c.members(f.members, make(map[string]bool)) {
			cp.inFeature = true
		}
	})

	if !c.needFeatures() {
		return
	}

	for _, f := range c.features {
		c.out.add("features", c.feature(f))
	}

	for _, cp := range c.comps {
		if !cp.inFeature && cp.out != nil {
			c.todof(cp.out, "", cp.e.line, "no feature lists this component; add it to one")
		}
	}
}

func (c *converter) feature(feat *feature) *omap { //nolint:cyclop // a flat list of independent WiX attributes
	elm := feat.e
	out := newMap()
	out.order("id", "title", "description", "default", "display", "directory", "allowAbsent", "components", "features")
	out.head = comments(elm)
	out.set("id", elm.attr("Id"))
	out.set("title", c.value(elm, "Title"))
	out.set("description", c.value(elm, "Description"))

	switch l := elm.attr("Level"); l {
	case "", "1":
	case "0":
		out.set("default", "disabled")
	default:
		out.set("default", "off")
	}

	switch display := elm.attr("Display"); display {
	case "":
	case "expand":
		out.set("display", "expanded")
	case "collapse":
		out.set("display", "collapsed")
	case "hidden":
		out.set("display", "hidden")
	default:
		c.todof(out, "display", elm.line, "Display=%q has no counterpart", display)
	}

	if d := elm.attr("ConfigurableDirectory"); d != "" {
		out.set("directory", c.dirRef(d))
	}

	if elm.no("AllowAbsent") || elm.attr("Absent") == "disallow" {
		out.set("allowAbsent", false)
	}

	for _, a := range []string{"InstallDefault", "TypicalDefault"} {
		if v := elm.attr(a); v != "" && v != "local" && v != onInstall {
			c.todof(out, "", elm.line, "%s=%q has no counterpart", a, v)
		}
	}

	if elm.yes("AllowAdvertise") {
		c.todof(out, "", elm.line, "advertised features are never supported")
	}

	for _, k := range elm.kids {
		if k.is("Level") || k.is("Condition") {
			c.todof(out, "default", k.line, "conditional feature levels have no counterpart; the condition was %q",
				strings.TrimSpace(k.attr("Condition")+k.text))
		}
	}

	var ids []string

	for _, cp := range c.members(feat.members, make(map[string]bool)) {
		if cp.out != nil {
			ids = append(ids, c.compID(cp))
		}
	}

	out.set("components", ids)

	for _, k := range feat.kids {
		out.add("features", c.feature(k))
	}

	return out
}
