package main

// ENV-02: equivalent one-file packages from WiX and msitools (wixl), both
// used as black boxes. The sources and payload are in env02/; wix.msi is
// built here with the wix.exe on PATH, wixl.msi on Linux by
// env02/build-wixl.sh and passed in through ENV02_WIXL.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func init() { cases["ENV-02"] = caseENV02 }

// env02Pkg is what one package showed; compare() lines up two of them.
type env02Pkg struct {
	name    string
	streams map[string]string   // decoded stream name -> "size sha256"
	columns map[string]string   // table -> "Name type, ..."
	rows    map[string][]string // table -> rows, each "a | b | c"
	summary map[uint32]string
}

func caseENV02() {
	src, _ := filepath.Abs("env02")
	payload, err := os.ReadFile(filepath.Join(src, "hello.txt"))
	if err != nil {
		fmt.Println("ENV-02 needs env02/hello.txt:", err)
		return
	}

	c := newCase("ENV-02", "wix")
	wixMSI, _ := filepath.Abs(c.path("wix.msi"))
	c.logf("wix --version: %s", strings.TrimSpace(cmdOut("wix", "--version")))
	cmd := exec.Command("wix", "build", "-arch", "x64", "-o", wixMSI, filepath.Join(src, "wix.wxs"))
	cmd.Dir = src
	out, err := cmd.CombinedOutput()
	c.logf("wix build -arch x64 (err=%v): %s", err, strings.TrimSpace(string(out)))
	var wix *env02Pkg
	if err == nil {
		wix = env02Analyze(c, wixMSI, filepath.Join(pf64(), "MsiSpecEnv02Wix", "hello.txt"), payload)
	}
	c.close()

	c = newCase("ENV-02", "wixl")
	var wixl *env02Pkg
	if p := os.Getenv("ENV02_WIXL"); p == "" {
		c.logf("ENV02_WIXL is not set; build it with env02/build-wixl.sh")
	} else {
		wixl = env02Analyze(c, p, filepath.Join(pf64(), "MsiSpecEnv02Wixl", "hello.txt"), payload)
	}
	c.close()

	if wix == nil || wixl == nil {
		return
	}
	c = newCase("ENV-02", "compare")
	defer c.close()
	env02Compare(c, wix, wixl)
}

func env02Analyze(c *caseLog, path, installed string, payload []byte) *env02Pkg {
	p := &env02Pkg{name: filepath.Base(path), streams: map[string]string{}, columns: map[string]string{}, rows: map[string][]string{}, summary: map[uint32]string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		c.logf("read: %v", err)
		return nil
	}
	c.logf("package %s: %d bytes sha256 %s", p.name, len(raw), sha(raw))

	// Container and streams, read with this directory's CFB reader.
	db, err := openRawMSI(raw)
	if err != nil {
		c.logf("raw open: %v", err)
		return nil
	}
	f := db.f
	le := binary.LittleEndian
	c.logf("CFB major %d, sector %d, %d directory entries, root CLSID % x (MSI database CLSID: %v)",
		f.Major, f.SS, len(f.Entries), f.Entries[0].CLSID, f.Entries[0].CLSID == msiCLSID)
	c.logf("header: minor %#x, mini cutoff %d, %d FAT sectors, transaction %d",
		le.Uint16(raw[0x18:]), le.Uint32(raw[0x38:]), le.Uint32(raw[0x2C:]), le.Uint32(raw[0x34:]))
	c.logf("")
	c.logf("| Stream | Kind | Size | SHA-256 |")
	c.logf("| --- | --- | ---: | --- |")
	var names []string
	kind := map[string]string{}
	for _, e := range f.Entries[1:] {
		if e.Type == 0 {
			continue // unused directory slot
		}
		if e.Type != 2 {
			c.logf("| entry %q | type %d | | |", e.NameString(), e.Type)
			continue
		}
		n, isTable := decodeStreamName(e.Name)
		k := "stream"
		if isTable {
			k = "table"
		} else if strings.HasPrefix(n, "\x05") {
			k = "property set"
			n = "\\005" + n[1:]
		}
		key := k + " " + n
		names = append(names, key)
		kind[key] = k
		p.streams[key] = fmt.Sprintf("%d %s", len(e.Data), sha(e.Data)[:16])
	}
	sort.Strings(names)
	for _, n := range names {
		c.logf("| `%s` | %s | %s |", strings.TrimPrefix(n, kind[n]+" "), kind[n], strings.Replace(p.streams[n], " ", " | ", 1))
	}
	c.logf("")

	// String pool.
	c.logf("string pool: code page %d, long refs %v, %d entries", db.CodePage, db.LongRefs, len(db.Pool)-1)
	want := db.refcounts()
	bad, unused := 0, 0
	for id := 1; id < len(db.Pool); id++ {
		e := db.Pool[id]
		if e.Len == 0 && e.Ref == 0 {
			continue
		}
		if want[uint32(id)] == 0 {
			unused++
		}
		if e.Ref != want[uint32(id)] {
			bad++
			if bad <= 5 {
				c.logf("  refcount id %d %q: stored %d, counted %d", id, e.Data, e.Ref, want[uint32(id)])
			}
		}
	}
	c.logf("refcounts differing from the counted references: %d; entries no row uses: %d", bad, unused)

	// Native view: summary information and every table.
	si, err := openSummary(0, path, 0)
	if err != nil {
		c.logf("native summary: %v", err)
	} else {
		for pid := uint32(1); pid <= 19; pid++ {
			v, err := getSummary(si, pid)
			if err != nil || v.VT == 1 || v.VT == 0 {
				continue
			}
			var s string
			switch v.VT {
			case vtLPSTR:
				s = fmt.Sprintf("%q", v.Text)
			case vtFILETIME:
				s = fmt.Sprintf("FILETIME %08x%08x", v.FT[1], v.FT[0])
			default:
				s = fmt.Sprintf("%d", v.Int)
			}
			p.summary[pid] = fmt.Sprintf("VT %d %s", v.VT, s)
			c.logf("summary PID %2d: %s", pid, p.summary[pid])
		}
		closeHandle(si)
	}
	h, err := openDB(path, dbReadOnly)
	if err != nil {
		c.logf("native open: %v", err)
		return nil
	}
	tables, err := query(h, "SELECT `Name` FROM `_Tables`")
	c.logf("native open ok; %d tables, err=%v", len(tables), err)
	for _, t := range tables {
		var cols []string
		for _, cd := range db.Columns[t[0]] {
			cols = append(cols, fmt.Sprintf("%s %#04x", cd.Name, cd.Type))
		}
		p.columns[t[0]] = strings.Join(cols, ", ")
		rows, err := query(h, "SELECT * FROM `"+t[0]+"`")
		if err != nil {
			c.logf("table %s: %v", t[0], err)
			continue
		}
		var rs []string
		for _, r := range rows {
			rs = append(rs, strings.ReplaceAll(strings.Join(r, " | "), nullMark, "NULL"))
		}
		sort.Strings(rs)
		p.rows[t[0]] = rs
		c.logf("")
		c.logf("table `%s` (%s), %d rows:", t[0], p.columns[t[0]], len(rs))
		for _, r := range rs {
			c.logf("    %s", r)
		}
	}
	c.logf("")

	// Embedded cabinet.
	media, _ := query(h, "SELECT `Cabinet` FROM `Media`")
	closeHandle(h)
	for _, m := range media {
		if !strings.HasPrefix(m[0], "#") {
			c.logf("Media cabinet %q is external", m[0])
			continue
		}
		e := db.Streams[m[0][1:]]
		if e == nil {
			c.logf("Media cabinet %q: stream missing", m[0])
			continue
		}
		cab := e.Data
		if len(cab) < 36 || string(cab[:4]) != "MSCF" {
			c.logf("cabinet %s: %d bytes, no cabinet header", m[0], len(cab))
			continue
		}
		if le.Uint16(cab[0x1E:])&7 != 0 || len(cab) < 36+8*int(le.Uint16(cab[0x1A:])) {
			c.logf("cabinet %s: flags %#x or %d bytes, not decoded", m[0], le.Uint16(cab[0x1E:]), len(cab))
			continue
		}
		c.logf("cabinet %s: %d bytes, header %q, set %d/%d, %d folders, %d files, flags %#x",
			m[0], len(cab), cab[:4], le.Uint16(cab[0x22:]), le.Uint16(cab[0x20:]), le.Uint16(cab[0x1A:]), le.Uint16(cab[0x1C:]), le.Uint16(cab[0x1E:]))
		for fi := 0; fi < int(le.Uint16(cab[0x1A:])); fi++ {
			fo := cab[36+8*fi:]
			c.logf("  folder %d: %d data blocks, compression %#04x", fi, le.Uint16(fo[4:]), le.Uint16(fo[6:]))
		}
		// readCab has no LZX decoder; expand.exe (cabinet.dll) reads every type.
		if files, err := readCab(cab); err != nil {
			c.logf("  msitest reader: %v", err)
		} else {
			for _, cf := range files {
				c.logf("  msitest reader: file %q, %d bytes, attrib %#x, payload match %v", cf.Name, len(cf.Data), cf.Attrib, bytes.Equal(cf.Data, payload))
			}
		}
		cabPath, dir := c.path("cab1.cab"), c.path("expanded")
		os.RemoveAll(dir) // never read an earlier run's output
		if err := os.WriteFile(cabPath, cab, 0o644); err != nil {
			c.logf("  write %s: %v", cabPath, err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			c.logf("  mkdir %s: %v", dir, err)
		}
		out, err := exec.Command("expand.exe", cabPath, "-F:*", dir).CombinedOutput()
		if err != nil {
			c.logf("  expand.exe: %v: %s", err, strings.TrimSpace(string(out)))
		}
		got, err := readExpanded(dir, "hello.txt")
		c.logf("  expand.exe: hello.txt payload match %v (err=%v)", err == nil && bytes.Equal(got, payload), err)
	}

	// Install and uninstall.
	_, _, ok := installCycle(c, path, map[string][]byte{installed: payload})
	c.logf("install/uninstall cycle ok: %v", ok)
	return p
}

func env02Compare(c *caseLog, a, b *env02Pkg) {
	c.logf("Differences between %s (A) and %s (B); identical entries are left out.", a.name, b.name)
	diff := func(what string, ka, kb map[string]string) {
		keys := map[string]bool{}
		for k := range ka {
			keys[k] = true
		}
		for k := range kb {
			keys[k] = true
		}
		var ks []string
		for k := range keys {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		c.logf("")
		c.logf("%s:", what)
		for _, k := range ks {
			va, ina := ka[k]
			vb, inb := kb[k]
			switch {
			case !inb:
				c.logf("  only A: %s = %s", k, va)
			case !ina:
				c.logf("  only B: %s = %s", k, vb)
			case va != vb:
				c.logf("  %s: A %s / B %s", k, va, vb)
			}
		}
	}
	diff("streams (size sha)", a.streams, b.streams)
	diff("table columns", a.columns, b.columns)
	sa, sb := map[string]string{}, map[string]string{}
	for k, v := range a.summary {
		sa[fmt.Sprintf("PID %2d", k)] = v
	}
	for k, v := range b.summary {
		sb[fmt.Sprintf("PID %2d", k)] = v
	}
	diff("summary information", sa, sb)
	ra, rb := map[string]string{}, map[string]string{}
	for t, rs := range a.rows {
		for _, r := range rs {
			ra[t+": "+r] = "row"
		}
	}
	for t, rs := range b.rows {
		for _, r := range rs {
			rb[t+": "+r] = "row"
		}
	}
	diff("rows", ra, rb)
}
