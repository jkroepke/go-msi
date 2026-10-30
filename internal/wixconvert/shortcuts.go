package wixconvert

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	fileTarget   = regexp.MustCompile(`^\[[#!]([A-Za-z_][A-Za-z0-9_.]*)\]$`)
	pathTarget   = regexp.MustCompile(`^\[([A-Za-z_][A-Za-z0-9_.]*)\]\\?([^\[\]\\]+)$`)
	folderTarget = regexp.MustCompile(`^\[([A-Za-z_][A-Za-z0-9_.]*)\]\\?$`)
)

// shortcut converts the attributes that every kind of shortcut has. dirID
// is the WiX directory the shortcut goes into.
func (c *converter) shortcut(elm *elem, dirID string, tree map[*dir]bool) *omap { //nolint:cyclop // a flat list of independent WiX attributes
	link := newMap()
	link.order("id", "name", "file", "folder", "program", "url", "directory", "arguments", "description",
		"workingDirectory", "icon", "iconFile", "iconIndex", "show", "properties")
	link.head = comments(elm)
	link.set("name", c.value(elm, "Name"))

	if d := elm.attr(attrDirectory); d != "" {
		dirID = d
	}

	dirValue := c.dirValue(dirID, tree)
	if dirValue == "" && c.installDir != nil { // the install dir; empty means the Start menu
		dirValue = c.dirRef(c.installDir.id)
	}

	link.set("directory", dirValue)
	link.set("arguments", c.value(elm, "Arguments"))
	link.set("description", c.value(elm, "Description"))
	link.set("icon", c.value(elm, "Icon"))

	if n, err := strconv.Atoi(elm.attr("IconIndex")); err == nil && n != 0 {
		link.set("iconIndex", n)
	}

	switch show := elm.attr("Show"); show {
	case "", "normal":
	case "maximized", "minimized":
		link.set("show", show)
	default:
		c.todof(link, "", elm.line, "Show=%q has no counterpart", show)
	}

	if elm.yes("Advertise") {
		c.todof(link, "", elm.line, "advertised shortcuts are never supported; this one is a plain shortcut")
	}

	for _, a := range []string{"Hotkey", "DescriptionResourceDll", "DisplayResourceDll"} {
		if elm.has(a) {
			c.todof(link, "", elm.line, "Shortcut %s has no counterpart", a)
		}
	}

	for _, kid := range elm.kids {
		switch kid.qname() {
		case "ShortcutProperty":
			link.child("properties").set(kid.attr("Key"), c.value(kid, "Value"))
		case "Icon":
			c.icons = append(c.icons, kid)
			link.set("icon", kid.attr("Id"))
		default:
			c.todof(link, "", kid.line, "<%s> in a Shortcut has no counterpart", kid.qname())
		}
	}

	return link
}

// fileShortcut converts a Shortcut nested in a File.
func (c *converter) fileShortcut(f *omap, e *elem, ref *fileRef, tree map[*dir]bool) {
	s := c.shortcut(e, ref.comp.dir, tree)
	c.workingDirectory(s, e, ref.dir)
	f.add("shortcuts", s)
}

func (c *converter) workingDirectory(s *omap, e *elem, fileDir string) {
	if wd := e.attr("WorkingDirectory"); wd != "" && wd != fileDir {
		s.set("workingDirectory", c.dirRef(wd))
	}
}

// placeShortcut puts a Shortcut outside a File onto the file it starts, or
// makes it a shortcut of its component.
func (c *converter) placeShortcut(pending pendingShortcut) {
	elm := pending.e

	if elm.qname() == "util:InternetShortcut" {
		c.urlShortcut(pending)

		return
	}

	target := elm.attr("Target")
	if file := c.targetFile(target); file != nil {
		link := c.shortcut(elm, pending.comp.dir, pending.tree)
		c.workingDirectory(link, elm, file.dir)

		// A shortcut component with a code stays, so that an upgrade
		// that removes the old version late keeps its shortcut (UPG-11).
		if pending.comp.dropIfEmpty && pending.comp.out.get("guid") != nil {
			link.set("file", file.key)
			pending.comp.out.add("shortcuts", link)

			return
		}

		file.out.add("shortcuts", link)

		return
	}

	link := c.shortcut(elm, pending.comp.dir, pending.tree)

	switch match := folderTarget.FindStringSubmatch(target); {
	case match != nil && !c.isDir(match[1]):
		// A property, such as a URL, not a folder.
		link.set("program", c.expand(elm, target))
	case target == "":
		// The shortcut is left out; its notes stay with the component.
		pending.comp.out.head = append(pending.comp.out.head, link.commentLines()...)
		c.todof(pending.comp.out, "", elm.line,
			"the shortcut %q has no Target: advertised shortcuts are never supported; point it at a file", elm.attr("Name"))

		return
	case match != nil:
		link.set("folder", c.dirRef(match[1]))
	default:
		link.set("program", c.expand(elm, target))
	}

	link.set("workingDirectory", c.dirRef(elm.attr("WorkingDirectory")))
	pending.comp.out.add("shortcuts", link)
}

// isDir reports whether an ID names a directory of the file or a standard
// folder.
func (c *converter) isDir(id string) bool {
	return c.dirs[id] != nil || isStandardFolder(id)
}

// targetFile finds the packaged file a shortcut target names, as [#id] or
// as [DIRECTORY]name.
func (c *converter) targetFile(target string) *fileRef {
	if m := fileTarget.FindStringSubmatch(target); m != nil {
		for _, f := range c.files {
			if f.key == m[1] {
				return f
			}
		}

		return nil
	}

	if m := pathTarget.FindStringSubmatch(target); m != nil {
		for _, f := range c.files {
			if f.dir == m[1] && strings.EqualFold(f.name, m[2]) {
				return f
			}
		}
	}

	return nil
}

// urlShortcut converts a util:InternetShortcut to a url shortcut.
func (c *converter) urlShortcut(pending pendingShortcut) {
	elm := pending.e
	link := c.shortcut(elm, pending.comp.dir, pending.tree)
	link.set("url", c.value(elm, "Target"))

	if elm.attr("Type") == "link" {
		c.todof(link, "", elm.line, "Type=\"link\" writes a .lnk; msi-builder writes a .url file for url shortcuts")
	}

	if icon := elm.attr("IconFile"); icon != "" {
		if m := fileTarget.FindStringSubmatch(icon); m != nil {
			link.set("iconFile", m[1])
		} else {
			c.todof(link, "", elm.line, "IconFile=%q: iconFile names a file of the package", icon)
		}
	}

	pending.comp.out.add("shortcuts", link)
}
