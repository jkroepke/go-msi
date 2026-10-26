package main

// Format-level cases: summary, CFB container and database variants.

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	cases["SUM-07"] = caseSUM07
	cases["SUM-02"] = caseSUM02
	cases["SUM-03"] = caseSUM03
	cases["SUM-04"] = caseSUM04
	cases["MSIDB-07"] = caseMSIDB07
	cases["MSIDB-04"] = caseMSIDB04
	cases["CFB-01"] = caseCFB01
	cases["CFB-02"] = caseCFB02
	cases["CFB-03"] = caseCFB03
	cases["CFB-04"] = caseCFB04
	cases["CFB-06"] = caseCFB06
	cases["MSIDB-09"] = caseMSIDB09
	cases["MSIDB-10"] = caseMSIDB10
}

// nativeOpen opens a package read-only, reads ProductCode and the summary.
func nativeOpen(c *caseLog, label, path string) uintptr {
	db, err := openDB(path, dbReadOnly)
	if err != nil {
		c.logf("%s: MsiOpenDatabaseW -> %v", label, err)
		return codeOf(err)
	}
	defer closeHandle(db)
	r, err := query(db, "SELECT `Value` FROM `Property` WHERE `Property`='ProductCode'")
	n := 0
	if tabs, err2 := query(db, "SELECT `Name` FROM `_Tables`"); err2 == nil {
		n = len(tabs)
	}
	c.logf("%s: open ok; ProductCode %v err=%v; %d tables", label, r, err, n)
	return codeOf(err)
}

// summaryOffsets returns the offset (within the stream) of each PID's value.
func summaryOffsets(d []byte) map[uint32]int {
	le := binary.LittleEndian
	sec := 0x30
	n := int(le.Uint32(d[sec+4:]))
	out := map[uint32]int{}
	for i := 0; i < n; i++ {
		pid := le.Uint32(d[sec+8+8*i:])
		off := int(le.Uint32(d[sec+12+8*i:]))
		out[pid] = sec + off
	}
	return out
}

func patchSummary(raw []byte, fn func(d []byte)) ([]byte, error) {
	f, err := readCFB(raw)
	if err != nil {
		return nil, err
	}
	e := f.ByName["\x05SummaryInformation"]
	d := append([]byte(nil), e.Data...)
	fn(d)
	return f.patchStream(e, d), nil
}

func writeVariant(c *caseLog, name string, b []byte) string {
	p, _ := filepath.Abs(c.path(name))
	os.WriteFile(p, b, 0o644)
	c.logf("variant %s sha256 %s", name, sha(b))
	return p
}

func tryInstall(c *caseLog, label, path string) int {
	r := runMsiexec(c, label+"-install.log", "/i", path)
	if r.Exit == 0 || r.Exit == 3010 {
		u := runMsiexec(c, label+"-uninstall.log", "/x", path)
		c.logf("%s: install %d, uninstall %d", label, r.Exit, u.Exit)
	} else {
		c.logf("%s: install %d", label, r.Exit)
	}
	return r.Exit
}

func caseSUM07() {
	c := newCase("SUM-07", "summary-system-identifier")
	defer c.close()
	p := &pkg{Name: "db14", Is64: true}
	out := control(c, p, "native.msi")
	if out == "" {
		return
	}
	raw, _ := os.ReadFile(out)
	f, _ := readCFB(raw)
	d := f.ByName["\x05SummaryInformation"].Data
	c.logf("native writer SystemIdentifier = %#08x", binary.LittleEndian.Uint32(d[4:]))
	for _, v := range []uint32{0x00020005, 0x00020006, 0x00000000, 0xFFFFFFFF} {
		b, _ := patchSummary(raw, func(d []byte) { binary.LittleEndian.PutUint32(d[4:], v) })
		path := writeVariant(c, fmt.Sprintf("sysid-%08x.msi", v), b)
		nativeOpen(c, fmt.Sprintf("sysid %#08x", v), path)
		tryInstall(c, fmt.Sprintf("sysid-%08x", v), path)
	}
}

func caseSUM02() {
	c := newCase("SUM-02", "template-platform-language")
	defer c.close()
	for _, t := range []string{"x64;1033", "x64;1033,1031", "x64;1031", "Intel;0", "x64;0", "X64;1033", "x64 ;1033", "intel;1033", "Intel64;1033", "AMD64;1033", "Arm64;1033", "arm64;1033", "Alpha,x64;1033", "x64,Intel;1033", ";1033", "x64"} {
		p := &pkg{Name: "sum02", Is64: true, Template: t}
		safe := strings.NewReplacer(";", "_", ",", "+", " ", "sp").Replace(t)
		out := control(c, p, "t-"+safe+".msi")
		if out == "" {
			continue
		}
		nativeOpen(c, "Template "+t, out)
		ex := tryInstall(c, "t-"+safe, out)
		c.logf("RESULT Template %q -> install %d", t, ex)
	}
}

func caseSUM03() {
	c := newCase("SUM-03", "page-count")
	defer c.close()
	for _, plat := range []string{"x64", "Intel"} {
		for _, pc := range []int32{0, 1, 99, 100, 101, 110, 120, 125, 130, 140, 149, 150, 199, 200, 301, 405, 500, 501, 510} {
			p := &pkg{Name: "sum03", Is64: plat == "x64", Template: plat + ";1033", PageCount: pc}
			if pc == 0 {
				p.PageCount = -1 // marker below
			}
			name := fmt.Sprintf("%s-pc%d.msi", plat, pc)
			out := control(c, p, name)
			if out == "" {
				continue
			}
			if pc == 0 {
				raw, _ := os.ReadFile(out)
				b, _ := patchSummary(raw, func(d []byte) { binary.LittleEndian.PutUint32(d[summaryOffsets(d)[14]+4:], 0) })
				out = writeVariant(c, name, b)
			}
			nativeOpen(c, name, out)
			ex := tryInstall(c, strings.TrimSuffix(name, ".msi"), out)
			c.logf("RESULT %s PageCount %d -> install %d", plat, pc, ex)
		}
	}
}

func caseSUM04() {
	c := newCase("SUM-04", "summary-codepage")
	defer c.close()
	p := &pkg{Name: "sum04", Is64: true}
	out := control(c, p, "base.msi")
	if out == "" {
		return
	}
	// native API: try to set PID 1 to various values
	for _, cp := range []int32{1252, 65001, 1200, 932, 0} {
		tmp := writeVariant(c, fmt.Sprintf("api-%d.msi", cp), mustRead(out))
		db, _ := openDB(tmp, dbTransact)
		si, err := openSummary(db, "", 5)
		if err == nil {
			err = setSummary(si, sumProp{PID: 1, VT: vtI2, Int: cp})
			if err == nil {
				err = persistSummary(si)
			}
			closeHandle(si)
		}
		cerr := commit(db)
		closeHandle(db)
		c.logf("API set PID1=%d: %v, commit %v", cp, err, cerr)
		raw, _ := os.ReadFile(tmp)
		f, _ := readCFB(raw)
		d := f.ByName["\x05SummaryInformation"].Data
		o := summaryOffsets(d)[1]
		c.logf("  stored VT %d value %d", binary.LittleEndian.Uint16(d[o:]), binary.LittleEndian.Uint16(d[o+4:]))
		nativeOpen(c, fmt.Sprintf("api PID1=%d", cp), tmp)
	}
	// raw patch: same bytes except PID 1 value
	raw := mustRead(out)
	for _, cp := range []uint16{65001, 1200, 1201, 932, 0, 32767, 32768} {
		b, _ := patchSummary(raw, func(d []byte) { binary.LittleEndian.PutUint16(d[summaryOffsets(d)[1]+4:], cp) })
		path := writeVariant(c, fmt.Sprintf("raw-%d.msi", cp), b)
		nativeOpen(c, fmt.Sprintf("raw PID1=%d", cp), path)
		ex := tryInstall(c, fmt.Sprintf("raw-%d", cp), path)
		c.logf("RESULT summary codepage %d -> install %d", cp, ex)
	}
}

func mustRead(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return b
}

func caseMSIDB07() {
	c := newCase("MSIDB-07", "column-limit")
	defer c.close()
	for _, n := range []int{31, 32, 33} {
		path, _ := filepath.Abs(c.path(fmt.Sprintf("cols%d.msi", n)))
		os.Remove(path)
		db, err := openDB(path, dbCreate)
		if err != nil {
			c.logf("create: %v", err)
			continue
		}
		var cols []string
		for i := 1; i <= n; i++ {
			cols = append(cols, fmt.Sprintf("`C%d` SHORT", i))
		}
		sql := fmt.Sprintf("CREATE TABLE `Wide` (%s PRIMARY KEY `C1`)", strings.Join(cols, ", "))
		_, err = query(db, sql)
		c.logf("%d columns: CREATE TABLE -> %v", n, err)
		if err == nil {
			var vals, names []string
			for i := 1; i <= n; i++ {
				vals = append(vals, fmt.Sprint(i))
				names = append(names, fmt.Sprintf("`C%d`", i))
			}
			_, err = query(db, fmt.Sprintf("INSERT INTO `Wide` (%s) VALUES (%s)", strings.Join(names, ","), strings.Join(vals, ",")))
			c.logf("  INSERT -> %v", err)
			c.logf("  commit -> %v", commit(db))
		}
		closeHandle(db)
		db, err = openDB(path, dbReadOnly)
		if err != nil {
			c.logf("  reopen -> %v", err)
			continue
		}
		rows, err := query(db, "SELECT * FROM `Wide`")
		c.logf("  reopen SELECT -> %d rows, %d fields, err=%v", len(rows), lenFirst(rows), err)
		cr, _ := query(db, "SELECT `Number` FROM `_Columns` WHERE `Table`='Wide'")
		c.logf("  _Columns rows for Wide: %d", len(cr))
		closeHandle(db)
		raw, _ := os.ReadFile(path)
		if rdb, err := openRawMSI(raw); err == nil {
			c.logf("  raw: %d columns in catalog, stream present=%v", len(rdb.Columns["Wide"]), rdb.TableStr["Wide"] != nil)
		}
	}
	// Same limits through the .idt importer and through ALTER TABLE.
	for _, n := range []int{31, 32, 33} {
		path, _ := filepath.Abs(c.path(fmt.Sprintf("idt%d.msi", n)))
		os.Remove(path)
		db, _ := openDB(path, dbCreate)
		dir, _ := os.MkdirTemp("", "idt")
		t := tbl{Name: "Wide", Keys: []string{"C1"}}
		var row []string
		for i := 1; i <= n; i++ {
			t.Cols = append(t.Cols, fmt.Sprintf("C%d i2", i))
			row = append(row, fmt.Sprint(i))
		}
		t.Rows = [][]string{row}
		err := importTable(db, dir, t)
		c.logf("%d columns via MsiDatabaseImportW -> %v; commit %v", n, err, commit(db))
		closeHandle(db)
		os.RemoveAll(dir)
	}
	path, _ := filepath.Abs(c.path("alter.msi"))
	os.Remove(path)
	db, _ := openDB(path, dbCreate)
	var cols []string
	for i := 1; i <= 31; i++ {
		cols = append(cols, fmt.Sprintf("`C%d` SHORT", i))
	}
	_, err := query(db, fmt.Sprintf("CREATE TABLE `Wide` (%s PRIMARY KEY `C1`)", strings.Join(cols, ", ")))
	c.logf("ALTER path: create 31 -> %v", err)
	_, err = query(db, "ALTER TABLE `Wide` ADD `C32` SHORT")
	c.logf("ALTER TABLE ADD 32nd column -> %v", err)
	closeHandle(db)
}

func lenFirst(r [][]string) int {
	if len(r) == 0 {
		return 0
	}
	return len(r[0])
}

func caseMSIDB04() {
	c := newCase("MSIDB-04", "long-strings")
	defer c.close()
	path, _ := filepath.Abs(c.path("long.msi"))
	os.Remove(path)
	db, _ := openDB(path, dbCreate)
	query(db, "CREATE TABLE `Long` (`K` CHAR(72) NOT NULL, `V` LONGCHAR PRIMARY KEY `K`)")
	lens := []int{65534, 65535, 65536, 70000}
	for i, n := range lens {
		v := strings.Repeat(string(rune('a'+i)), n)
		_, err := query(db, "INSERT INTO `Long` (`K`,`V`) VALUES (?, ?)", fmt.Sprintf("k%d", n), v)
		c.logf("insert %d bytes: %v", n, err)
	}
	// neighbours
	query(db, "INSERT INTO `Long` (`K`,`V`) VALUES (?, ?)", "k_small", "after")
	c.logf("commit: %v", commit(db))
	closeHandle(db)
	db, err := openDB(path, dbReadOnly)
	if err != nil {
		c.logf("reopen: %v", err)
		return
	}
	rows, err := query(db, "SELECT `K`,`V` FROM `Long`")
	for _, r := range rows {
		c.logf("native read %s -> %d chars", r[0], len(r[1]))
	}
	closeHandle(db)
	raw, _ := os.ReadFile(path)
	rdb, err := openRawMSI(raw)
	if err != nil {
		c.logf("raw reader: %v", err)
		return
	}
	sp := rdb.TableStr["_StringPool"].Data
	le := binary.LittleEndian
	i := 4
	for id := 1; id < len(rdb.Pool); id++ {
		e := rdb.Pool[id]
		if e.Long || e.Len >= 65534 {
			words := []uint16{}
			for k := 0; k < 4 && i+2*k+2 <= len(sp); k++ {
				words = append(words, le.Uint16(sp[i+2*k:]))
			}
			c.logf("pool id %d: len %d long=%v raw words %v", id, e.Len, e.Long, words)
		}
		if e.Long {
			i += 8
		} else {
			i += 4
		}
	}
	c.logf("raw reader with layout (0, refcount, low, high) decoded pool OK; _StringData %d bytes", len(rdb.TableStr["_StringData"].Data))
}

// repack rewrites every root stream of an MSI with the spec writer.
func repack(raw []byte, o cfbWriteOpts) ([]byte, *cfbFile, error) {
	f, err := readCFB(raw)
	if err != nil {
		return nil, nil, err
	}
	if o.CLSID == ([16]byte{}) {
		o.CLSID = f.Entries[0].CLSID
	}
	b, err := writeCFB(f.streamsOf(), o)
	return b, f, err
}

func caseCFB01() {
	c := newCase("CFB-01", "v3-vs-v4")
	defer c.close()
	p := &pkg{Name: "cfb01", Is64: true}
	out := control(c, p, "native.msi")
	if out == "" {
		return
	}
	raw := mustRead(out)
	nf, _ := readCFB(raw)
	c.logf("native output: CFB v%d, %d bytes, root CLSID % x", nf.Major, len(raw), nf.Entries[0].CLSID)
	for _, m := range []int{3, 4} {
		b, f, err := repack(raw, cfbWriteOpts{Major: m})
		if err != nil {
			c.logf("repack v%d: %v", m, err)
			continue
		}
		path := writeVariant(c, fmt.Sprintf("repack-v%d.msi", m), b)
		// logical equality
		g, _ := readCFB(b)
		same := len(g.ByName) == len(f.ByName)
		for k, e := range f.ByName {
			if h := g.ByName[k]; h == nil || string(h.Data) != string(e.Data) {
				same = false
			}
		}
		c.logf("v%d: %d bytes, all streams byte-identical to native: %v", m, len(b), same)
		nativeOpen(c, fmt.Sprintf("repack v%d", m), path)
		tryInstall(c, fmt.Sprintf("repack-v%d", m), path)
	}
}

func caseCFB02() {
	c := newCase("CFB-02", "root-clsid")
	defer c.close()
	p := &pkg{Name: "cfb02", Is64: true}
	out := control(c, p, "native.msi")
	if out == "" {
		return
	}
	raw := mustRead(out)
	variants := map[string][16]byte{"correct": msiCLSID, "zero": {}}
	mut := msiCLSID
	mut[0] ^= 1
	variants["one-byte"] = mut
	mst := msiCLSID
	mst[0] = 0x82 // transform CLSID
	variants["mst-clsid"] = mst
	for _, name := range []string{"correct", "zero", "one-byte", "mst-clsid"} {
		b := append([]byte(nil), raw...)
		f, _ := readCFB(b)
		cl := variants[name]
		copy(b[f.Entries[0].dirOffset+0x50:], cl[:])
		path := writeVariant(c, name+".msi", b)
		if _, err := readCFB(b); err != nil {
			c.logf("%s: generic CFB reader error %v", name, err)
		} else {
			c.logf("%s: generic CFB reader ok", name)
		}
		nativeOpen(c, name, path)
		ex := tryInstall(c, name, path)
		c.logf("RESULT root CLSID %s -> install %d", name, ex)
	}
}

func caseCFB03() {
	c := newCase("CFB-03", "empty-stream-start")
	defer c.close()
	// Package with a Binary row whose stream is empty, written by the native engine.
	p := &pkg{Name: "cfb03", Is64: true}
	out := control(c, p, "base.msi")
	if out == "" {
		return
	}
	db, _ := openDB(out, dbTransact)
	_, err := query(db, "CREATE TABLE `Binary` (`Name` CHAR(72) NOT NULL, `Data` OBJECT NOT NULL PRIMARY KEY `Name`)")
	c.logf("create Binary: %v", err)
	empty, _ := filepath.Abs(c.path("empty.bin"))
	os.WriteFile(empty, nil, 0o644)
	c.logf("insert empty Binary row: %v", insertStream(db, "Binary", "Empty", empty))
	c.logf("commit: %v", commit(db))
	closeHandle(db)
	raw := mustRead(out)
	f, _ := readCFB(raw)
	var emptyNames []string
	for _, e := range f.Entries {
		if e.Type == 2 && e.Size == 0 {
			n, _ := decodeStreamName(e.Name)
			emptyNames = append(emptyNames, fmt.Sprintf("%s start=%#x", n, e.Start))
		}
	}
	c.logf("native writer: size-0 streams: %v", emptyNames)
	if len(emptyNames) == 0 {
		c.logf("native writer stored no stream for the empty Binary value")
	}
	// Repack with our writer adding a stray empty stream with three start values.
	for _, st := range []uint32{endOfChain, freeSect, 0} {
		streams := f.streamsOf()
		streams = append(streams, cfbStream{Name: encodeStreamName("Binary.Empty", false), Data: nil})
		// drop duplicates if native already wrote one
		seen := map[string]bool{}
		var uniq []cfbStream
		for _, s := range streams {
			k := string(utf16ToRunes(s.Name))
			if !seen[k] {
				seen[k] = true
				uniq = append(uniq, s)
			}
		}
		b, err := writeCFB(uniq, cfbWriteOpts{Major: 3, CLSID: msiCLSID, EmptyStart: st, EmptyStartSet: true})
		if err != nil {
			c.logf("write: %v", err)
			continue
		}
		path := writeVariant(c, fmt.Sprintf("empty-start-%x.msi", st), b)
		if _, err := readCFB(b); err != nil {
			c.logf("start %#x: our reader error %v", st, err)
		}
		nativeOpen(c, fmt.Sprintf("empty start %#x", st), path)
		if db, err := openDB(path, dbReadOnly); err == nil {
			data, err := readStreamField(db, "SELECT `Data` FROM `Binary` WHERE `Name`='Empty'", 2)
			c.logf("  native read Binary.Empty -> %d bytes err=%v", len(data), err)
			closeHandle(db)
		}
		// native commit through it
		tmp := writeVariant(c, fmt.Sprintf("empty-start-%x-commit.msi", st), b)
		if db, err := openDB(tmp, dbTransact); err == nil {
			_, qerr := query(db, "INSERT INTO `Property` (`Property`,`Value`) VALUES ('ZZTEST','1')")
			c.logf("  native insert+commit: %v / %v", qerr, commit(db))
			closeHandle(db)
		}
		ex := tryInstall(c, fmt.Sprintf("empty-start-%x", st), path)
		c.logf("RESULT empty stream start %#x -> install %d", st, ex)
	}
}

func utf16ToRunes(u []uint16) []rune {
	r := make([]rune, len(u))
	for i, x := range u {
		r[i] = rune(x)
	}
	return r
}

func caseCFB04() {
	c := newCase("CFB-04", "tree-colours")
	defer c.close()
	p := &pkg{Name: "cfb04", Is64: true}
	out := control(c, p, "native.msi")
	if out == "" {
		return
	}
	raw := mustRead(out)
	for _, v := range []struct {
		name string
		mut  func(b []byte, f *cfbFile)
		o    cfbWriteOpts
	}{
		{"red-black", nil, cfbWriteOpts{Major: 3}},
		{"all-black", nil, cfbWriteOpts{Major: 3, AllBlack: true}},
		{"all-red", func(b []byte, f *cfbFile) {
			for _, e := range f.Entries[1:] {
				if e.Type != 0 {
					b[e.dirOffset+0x43] = 0
				}
			}
		}, cfbWriteOpts{Major: 3}},
		{"root-red", func(b []byte, f *cfbFile) { b[f.Entries[0].dirOffset+0x43] = 0 }, cfbWriteOpts{Major: 3}},
		{"degenerate-chain", func(b []byte, f *cfbFile) {
			// rebuild links as a right-leaning chain in sorted order (unequal black height)
			var ents []*cfbEntry
			for _, e := range f.Entries {
				if e.Type == 2 && e.Parent == 0 {
					ents = append(ents, e)
				}
			}
			sortEntries(ents)
			le := binary.LittleEndian
			le.PutUint32(b[f.Entries[0].dirOffset+0x4C:], uint32(ents[0].SID))
			for i, e := range ents {
				le.PutUint32(b[e.dirOffset+0x44:], noStream)
				r := uint32(noStream)
				if i+1 < len(ents) {
					r = uint32(ents[i+1].SID)
				}
				le.PutUint32(b[e.dirOffset+0x48:], r)
				b[e.dirOffset+0x43] = 1
			}
		}, cfbWriteOpts{Major: 3}},
	} {
		b, _, err := repack(raw, v.o)
		if err != nil {
			c.logf("%s: %v", v.name, err)
			continue
		}
		if v.mut != nil {
			f, _ := readCFB(b)
			v.mut(b, f)
		}
		path := writeVariant(c, v.name+".msi", b)
		if _, err := readCFB(b); err != nil {
			c.logf("%s: our reader %v", v.name, err)
		}
		nativeOpen(c, v.name, path)
		ex := tryInstall(c, v.name, path)
		c.logf("RESULT tree %s -> install %d", v.name, ex)
	}
}

func sortEntries(e []*cfbEntry) {
	for i := 1; i < len(e); i++ {
		for j := i; j > 0 && cfbCompare(e[j].Name, e[j-1].Name) < 0; j-- {
			e[j], e[j-1] = e[j-1], e[j]
		}
	}
}

func caseCFB06() {
	c := newCase("CFB-06", "sibling-order")
	defer c.close()
	p := &pkg{Name: "cfb06", Is64: true}
	out := control(c, p, "native.msi")
	if out == "" {
		return
	}
	raw := mustRead(out)
	b, _, _ := repack(raw, cfbWriteOpts{Major: 3})
	// swap left and right links of the top node: tree no longer ordered
	f, _ := readCFB(b)
	top := f.Entries[f.Entries[0].Child]
	le := binary.LittleEndian
	mut := append([]byte(nil), b...)
	le.PutUint32(mut[top.dirOffset+0x44:], top.Right)
	le.PutUint32(mut[top.dirOffset+0x48:], top.Left)
	path := writeVariant(c, "swapped-top-children.msi", mut)
	if _, err := readCFB(mut); err != nil {
		c.logf("our reader (walks, no search): %v", err)
	} else {
		c.logf("our reader (full walk, no tree search): opens fine")
	}
	nativeOpen(c, "swapped", path)
	ex := tryInstall(c, "swapped", path)
	c.logf("RESULT unordered siblings -> install %d", ex)
	// case-variant duplicate names: add 'property' encoded vs existing? Names differing only by case
	streams := f.streamsOf()
	streams = append(streams, cfbStream{Name: u16("ABC"), Data: []byte("x")}, cfbStream{Name: u16("abc"), Data: []byte("y")})
	if _, err := writeCFB(streams, cfbWriteOpts{Major: 3, CLSID: msiCLSID}); err != nil {
		c.logf("writer rejects case-only duplicates: %v", err)
	}
}

func caseMSIDB09() {
	c := newCase("MSIDB-09", "empty-table")
	defer c.close()
	p := &pkg{Name: "db09", Is64: true, Extra: map[string][][]string{"LaunchCondition": nil}}
	out := control(c, p, "native.msi")
	if out == "" {
		return
	}
	raw := mustRead(out)
	rdb, _ := openRawMSI(raw)
	has := false
	for _, t := range rdb.Tables {
		if t == "LaunchCondition" {
			has = true
		}
	}
	c.logf("native: LaunchCondition in _Tables=%v, stream present=%v", has, rdb.TableStr["LaunchCondition"] != nil)
	f, _ := readCFB(raw)
	streams := f.streamsOf()
	streams = append(streams, cfbStream{Name: encodeStreamName("LaunchCondition", true), Data: nil})
	b, err := writeCFB(streams, cfbWriteOpts{Major: 3, CLSID: msiCLSID})
	if err != nil {
		c.logf("write: %v", err)
		return
	}
	path := writeVariant(c, "empty-physical-stream.msi", b)
	nativeOpen(c, "empty physical stream", path)
	tmp := writeVariant(c, "empty-physical-stream-commit.msi", b)
	if db, err := openDB(tmp, dbTransact); err == nil {
		_, e1 := query(db, "INSERT INTO `LaunchCondition` (`Condition`,`Description`) VALUES ('1','x')")
		c.logf("insert into empty-stream table + commit: %v / %v", e1, commit(db))
		closeHandle(db)
	}
	tryInstall(c, "no-stream", out)
	ex := tryInstall(c, "empty-stream", path)
	c.logf("RESULT empty physical stream -> install %d", ex)
}

func caseMSIDB10() {
	c := newCase("MSIDB-10", "validation-table")
	defer c.close()
	p := &pkg{Name: "db10", Is64: true}
	out := control(c, p, "missing.msi")
	if out == "" {
		return
	}
	tryInstall(c, "missing", out)
	// empty _Validation
	tmp := writeVariant(c, "empty.msi", mustRead(out))
	db, _ := openDB(tmp, dbTransact)
	_, err := query(db, "CREATE TABLE `_Validation` (`Table` CHAR(32) NOT NULL, `Column` CHAR(32) NOT NULL, `Nullable` CHAR(4) NOT NULL, `MinValue` LONG, `MaxValue` LONG, `KeyTable` CHAR(255), `KeyColumn` SHORT, `Category` CHAR(32), `Set` CHAR(255), `Description` CHAR(255) PRIMARY KEY `Table`, `Column`)")
	c.logf("create empty _Validation: %v, commit %v", err, commit(db))
	closeHandle(db)
	ex := tryInstall(c, "empty", tmp)
	c.logf("RESULT missing _Validation installs (see missing-install.log); empty _Validation -> %d", ex)
}
