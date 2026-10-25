package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	cases["DB-02"] = caseDB02
	cases["DB-05"] = caseDB05
	cases["DB-06"] = caseDB06
	cases["DB-08"] = caseDB08
	cases["DB-11"] = caseDB11
	cases["SUM-05"] = caseSum05
	cases["SUM-06"] = caseSum06
	cases["CAB-04"] = caseCab04
	cases["CAB-05"] = caseCab05
	cases["CAB-06"] = caseCab06
	cases["RUST-11"] = caseRust11
}

func caseDB02() {
	c := newCase("DB-02", "nullable-integers")
	defer c.close()
	path, _ := filepath.Abs(c.path("ints.msi"))
	os.Remove(path)
	db, _ := openDB(path, dbCreate)
	_, err := query(db, "CREATE TABLE `T` (`K` SHORT NOT NULL, `S` SHORT, `L` LONG, `C` CHAR(10), `B` OBJECT PRIMARY KEY `K`)")
	c.logf("create: %v", err)
	vals := [][2]string{{"1", "-32767"}, {"2", "32767"}, {"3", "0"}, {"4", "-1"}, {"5", ""}}
	for _, v := range vals {
		s := v[1]
		if s == "" {
			s = "NULL"
		}
		_, err := query(db, fmt.Sprintf("INSERT INTO `T` (`K`,`S`,`L`,`C`) VALUES (%s, %s, %s, %s)", v[0], s, map[string]string{"1": "-2147483647", "2": "2147483647", "3": "0", "4": "-1", "5": "NULL"}[v[0]], map[string]string{"5": "NULL"}[v[0]]+map[bool]string{true: "", false: "'x'"}[v[0] == "5"]))
		c.logf("insert row %s: %v", v[0], err)
	}
	for _, bad := range []string{"INSERT INTO `T` (`K`,`S`) VALUES (6, -32768)", "INSERT INTO `T` (`K`,`L`) VALUES (7, -2147483648)", "INSERT INTO `T` (`K`,`S`) VALUES (8, 40000)"} {
		_, err := query(db, bad)
		c.logf("%s -> %v", bad, err)
	}
	c.logf("commit %v", commit(db))
	closeHandle(db)
	db, _ = openDB(path, dbReadOnly)
	rows, _ := query(db, "SELECT `K`,`S`,`L`,`C` FROM `T`")
	for _, r := range rows {
		c.logf("native: %q", r)
	}
	closeHandle(db)
	rdb, err := openRawMSI(mustRead(path))
	if err != nil {
		c.logf("raw: %v", err)
		return
	}
	rr, cols, _ := rdb.rows("T")
	for _, r := range rr {
		var s []string
		for i, cell := range r {
			s = append(s, fmt.Sprintf("%s=%#x(%s)", cols[i].Name, cell.Raw, cell.Text))
		}
		c.logf("raw: %s", strings.Join(s, " "))
	}
	var types []string
	for _, col := range cols {
		types = append(types, fmt.Sprintf("%s=%#04x", col.Name, col.Type))
	}
	c.logf("column types: %s", strings.Join(types, " "))
}

func bulkTable(c *caseLog, path string, n int, value func(i int) string) {
	os.Remove(path)
	db, _ := openDB(path, dbCreate)
	defer closeHandle(db)
	dir, _ := os.MkdirTemp("", "bulk")
	defer os.RemoveAll(dir)
	t := tbl{Name: "Bulk", Cols: []string{"K i4", "V S255"}, Keys: []string{"K"}}
	for i := 1; i <= n; i++ {
		t.Rows = append(t.Rows, []string{fmt.Sprint(i), value(i)})
	}
	c.logf("import %d rows: %v; commit %v", n, importTable(db, dir, t), commit(db))
}

func caseDB05() {
	c := newCase("DB-05", "refcount-overflow")
	defer c.close()
	for _, n := range []int{32766, 32767, 32768, 65534, 65535, 65536, 70000} {
		path, _ := filepath.Abs(c.path(fmt.Sprintf("ref%d.msi", n)))
		if os.Getenv("REUSE") == "" || !exists(path) {
			bulkTable(c, path, n, func(int) string { return "same" })
		}
		raw := mustRead(path)
		rdb, err := openRawMSI(raw)
		if err != nil {
			c.logf("raw: %v", err)
			continue
		}
		var dups []string
		for i, e := range rdb.Pool {
			if string(e.Data) == "same" {
				dups = append(dups, fmt.Sprintf("id %d len %d ref %d", i, e.Len, e.Ref))
			}
		}
		c.logf("n=%d: pool entries holding 'same': %v; total pool %d", n, dups, len(rdb.Pool)-1)
		id := rdb.idOf("same")
		c.logf("n=%d: stored refcount of 'same' (id %d) = %d (real refs %d); longRefs %v", n, id, rdb.Pool[id].Ref, n, rdb.LongRefs)
		db, err := openDB(path, dbReadOnly)
		if err != nil {
			c.logf("  native reopen: %v", err)
			continue
		}
		r, err := query(db, "SELECT `K` FROM `Bulk` WHERE `V`='same'")
		c.logf("  native rows with V='same': %d err=%v", len(r), err)
		r1, err := query(db, "SELECT `V` FROM `Bulk` WHERE `K`=1")
		c.logf("  native K=1 -> %q err=%v", r1, err)
		e := rdb.TableStr["Bulk"]
		c.logf("  raw first V id: %d (stream %d bytes)", binary.LittleEndian.Uint16(e.Data[n*4:]), e.Size)
		sp := rdb.TableStr["_StringPool"].Data
		c.logf("  raw _StringPool tail: % x", sp[max(0, len(sp)-12):])
		closeHandle(db)
	}
}

func caseDB06() {
	c := newCase("DB-06", "string-id-width")
	defer c.close()
	for _, n := range []int{65400, 65520, 65526, 65527, 65528, 65529, 65530, 70000} {
		path, _ := filepath.Abs(c.path(fmt.Sprintf("ids%d.msi", n)))
		bulkTable(c, path, n, func(i int) string { return fmt.Sprintf("s%06d", i) })
		raw := mustRead(path)
		rdb, err := openRawMSI(raw)
		if err != nil {
			c.logf("n=%d raw: %v", n, err)
			continue
		}
		maxID := len(rdb.Pool) - 1
		e := rdb.TableStr["Bulk"]
		c.logf("n=%d distinct values: pool entries %d (highest id %d), long-ref flag %v, Bulk stream %d bytes = %d bytes/row", n, maxID, maxID, rdb.LongRefs, e.Size, int(e.Size)/n)
		db, err := openDB(path, dbReadOnly)
		if err == nil {
			r, err := query(db, "SELECT `V` FROM `Bulk` WHERE `K`="+fmt.Sprint(n))
			c.logf("  native last row: %v err=%v", r, err)
			closeHandle(db)
		}
	}
}

func caseDB08() {
	for _, v := range []struct {
		name string
		cp   int
		text string
	}{
		{"cp1252", 1252, "CafÃ© â‚¬"},
		{"cp65001", 65001, "CafÃ© â‚¬ æ—¥æœ¬"},
		{"cp1251", 1251, "ÐŸÑ€Ð¸Ð²ÐµÑ‚"},
		{"cp0-nonascii", 0, "CafÃ©"},
	} {
		c := newCase("DB-08", v.name)
		p := &pkg{Name: "db08" + v.name, Is64: true, InstallDirName: "MsiSpec" + v.name,
			Files: []payload{{Key: "f_readme", Name: "msispec|" + "RÃ©sumÃ© " + v.name + ".txt", Data: []byte("x")}}}
		p.ExtraProps = [][]string{{"TESTTEXT", v.text}}
		p.Files[0].Name = "readme.txt|RÃ©sumÃ©.txt"
		if v.cp == 65001 {
			p.Files[0].Name = "readme.txt|æ—¥æœ¬.txt"
		}
		if v.cp == 1251 {
			p.Files[0].Name = "readme.txt|ÐŸÑ€.txt"
		}
		out, _ := filepath.Abs(c.path("pkg.msi"))
		err := buildWithCodepage(p, out, v.cp)
		c.logf("build with database code page %d: %v", v.cp, err)
		if err != nil {
			c.close()
			continue
		}
		raw := mustRead(out)
		rdb, _ := openRawMSI(raw)
		id := rdb.idOf(encodeCP(v.text, v.cp))
		c.logf("raw pool code page %d; TESTTEXT stored bytes % x (found id %d)", rdb.CodePage, []byte(encodeCP(v.text, v.cp)), id)
		db, _ := openDB(out, dbReadOnly)
		r, err := query(db, "SELECT `Value` FROM `Property` WHERE `Property`='TESTTEXT'")
		if len(r) > 0 {
			c.logf("native read back %q equal=%v err=%v", r[0][0], r[0][0] == v.text, err)
		} else {
			c.logf("native read: %v", err)
		}
		closeHandle(db)
		ex := tryInstallKeep(c, out, p)
		c.logf("RESULT %s: install %d", v.name, ex)
		c.close()
	}
}

func encodeCP(s string, cp int) string {
	if cp == 65001 {
		return s
	}
	b, err := cpEncode(s, cp)
	if err != nil {
		return s
	}
	return string(b)
}

func tryInstallKeep(c *caseLog, out string, p *pkg) int {
	r := runMsiexec(c, "install.log", "/i", out)
	dir := filepath.Join(pf64(), p.InstallDirName)
	ents, _ := os.ReadDir(dir)
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	c.logf("installed files in %s: %q", dir, names)
	if r.Exit == 0 {
		runMsiexec(c, "uninstall.log", "/x", out)
	}
	return r.Exit
}

// buildWithCodepage builds p but imports _ForceCodepage first.
func buildWithCodepage(p *pkg, out string, cp int) error {
	forcedCodepage = cp
	defer func() { forcedCodepage = -1 }()
	return p.build(out)
}

func caseDB11() {
	c := newCase("DB-11", "binary-stream-names")
	defer c.close()
	path, _ := filepath.Abs(c.path("bin.msi"))
	os.Remove(path)
	db, _ := openDB(path, dbCreate)
	query(db, "CREATE TABLE `Binary` (`Name` CHAR(72) NOT NULL, `Data` OBJECT NOT NULL PRIMARY KEY `Name`)")
	query(db, "CREATE TABLE `Two` (`A` CHAR(72) NOT NULL, `B` SHORT NOT NULL, `Data` OBJECT PRIMARY KEY `A`, `B`)")
	data, _ := filepath.Abs(c.path("data.bin"))
	os.WriteFile(data, []byte("payload"), 0o644)
	for _, n := range []int{10, 55, 56, 57, 60} {
		key := strings.Repeat("k", n)
		err := insertStream(db, "Binary", key, data)
		c.logf("Binary key %d chars (stream name %d chars): insert %v", n, n+7, err)
	}
	c.logf("commit: %v", commit(db))
	closeHandle(db)
	db, err := openDB(path, dbReadOnly)
	if err != nil {
		c.logf("reopen: %v", err)
		return
	}
	rows, _ := query(db, "SELECT `Name` FROM `Binary`")
	for _, r := range rows {
		b, err := readStreamField(db, "SELECT `Data` FROM `Binary` WHERE `Name`='"+r[0]+"'", 1)
		c.logf("stored key %d chars: read %q err=%v", len(r[0]), b, err)
	}
	closeHandle(db)
	rdb, _ := openRawMSI(mustRead(path))
	var names []string
	for n := range rdb.Streams {
		names = append(names, fmt.Sprintf("%s(%d units)", n, len(encodeStreamName(n, false))))
	}
	c.logf("raw streams: %v", names)
	// two-key binary table
	db, _ = openDB(path, dbTransact)
	_, err = query(db, "INSERT INTO `Two` (`A`,`B`) VALUES ('x', -5)")
	c.logf("two-key row with NULL stream: %v", err)
	commit(db)
	closeHandle(db)
}

func caseSum05() {
	c := newCase("SUM-05", "word-count")
	defer c.close()
	for _, wc := range []int32{0, 1, 2, 3, 4, 8, 10} {
		p := &pkg{Name: "sum05", Is64: true, WordCount: wc}
		if wc == 0 {
			p.WordCount = -1
		}
		name := fmt.Sprintf("wc%d.msi", wc)
		out := control(c, p, name)
		if wc&2 == 0 {
			// uncompressed: payload must sit in the source tree; with bit 1 short names
			sub := "MsiSpecTest"
			if wc&1 != 0 {
				sub = "msispect"
			}
			os.MkdirAll(filepath.Join(filepath.Dir(out), sub), 0o755)
			os.WriteFile(filepath.Join(filepath.Dir(out), sub, "readme.txt"), p.Files[0].Data, 0o644)
		}
		ex := tryInstall(c, strings.TrimSuffix(name, ".msi"), out)
		c.logf("RESULT Word Count %d -> install %d", wc, ex)
	}
}

func caseSum06() {
	c := newCase("SUM-06", "revision-and-times")
	defer c.close()
	for _, pc := range []string{"{12345678-1234-1234-1234-1234567890AB}", "{12345678-1234-1234-1234-1234567890ab}", "12345678-1234-1234-1234-1234567890AB", "not-a-guid"} {
		p := &pkg{Name: "sum06", Is64: true, PackageCode: pc}
		safe := strings.NewReplacer("{", "", "}", "").Replace(pc)
		out := control(c, p, "pc-"+safe+".msi")
		ex := tryInstall(c, "pc-"+safe, out)
		c.logf("RESULT PackageCode %q -> install %d", pc, ex)
	}
	p := &pkg{Name: "sum06t", Is64: true}
	out := control(c, p, "times.msi")
	si, _ := openSummary(0, out, 0)
	v12, _ := getSummary(si, 12)
	closeHandle(si)
	c.logf("PID 12 read back: %08x%08x (written 2026-01-01T00:00:00Z = 0x01DC7AB192810000)", v12.FT[1], v12.FT[0])
}

func cabWithMutation(files []payload, mut func(b []byte)) []byte {
	var cf []cabFile
	for _, f := range files {
		cf = append(cf, cabFile{Name: f.Key, Data: f.Data, Date: 0x0021, Attrib: 0x20})
	}
	b, _ := writeCab(cf, cabOpts{Compress: 1})
	if mut != nil {
		mut(b)
	}
	return b
}

func caseCab04() {
	c := newCase("CAB-04", "header-mutations")
	defer c.close()
	data := crossBlockData()[:50000]
	files := []payload{{Key: "f_data", Name: "data.bin", Data: data}}
	le := binary.LittleEndian
	muts := []struct {
		name string
		fn   func(b []byte)
	}{
		{"control", nil},
		{"minor-2", func(b []byte) { b[0x18] = 2 }},
		{"major-2", func(b []byte) { b[0x19] = 2 }},
		{"cbCabinet-plus-100", func(b []byte) { le.PutUint32(b[8:], le.Uint32(b[8:])+100) }},
		{"cbCabinet-minus-100", func(b []byte) { le.PutUint32(b[8:], le.Uint32(b[8:])-100) }},
		{"checksum-wrong", func(b []byte) { o := int(le.Uint32(b[36:])); b[o] ^= 0xFF }},
		{"checksum-zero", func(b []byte) { o := int(le.Uint32(b[36:])); le.PutUint32(b[o:], 0) }},
		{"payload-bitflip", func(b []byte) { o := int(le.Uint32(b[36:])); b[o+8+100] ^= 0x01 }},
		{"payload-bitflip-checksum-zero", func(b []byte) { o := int(le.Uint32(b[36:])); le.PutUint32(b[o:], 0); b[o+8+100] ^= 0x01 }},
	}
	for _, m := range muts {
		cab := cabWithMutation(files, m.fn)
		p := &pkg{Name: "cab04", Is64: true, Files: files, CabOverride: cab}
		out := control(c, p, m.name+".msi")
		os.WriteFile(c.path(m.name+".cab"), cab, 0o644)
		x := c.path("x-" + m.name)
		os.MkdirAll(x, 0o755)
		exp := lastLine(cmdOut("expand.exe", c.path(m.name+".cab"), "-F:*", x))
		got, _ := readExpanded(x, "f_data")
		target := filepath.Join(pf64(), "MsiSpecTest", "data.bin")
		r := runMsiexec(c, m.name+"-install.log", "/i", out)
		inst, _ := os.ReadFile(target)
		if r.Exit == 0 {
			runMsiexec(c, m.name+"-uninstall.log", "/x", out)
		}
		c.logf("RESULT %s: expand.exe %q extracted-equal=%v; msiexec %d installed-equal=%v", m.name, strings.TrimSpace(exp), bytes.Equal(got, data), r.Exit, bytes.Equal(inst, data))
	}
}

func caseCab05() {
	c := newCase("CAB-05", "zero-byte-file")
	defer c.close()
	files := []payload{{Key: "f_empty", Name: "empty.txt", Data: []byte{}}, {Key: "f_readme", Name: "readme.txt", Data: []byte("x\r\n")}}
	for _, mode := range []string{"makecab-mszip", "go-mszip", "go-stored"} {
		p := &pkg{Name: "cab05", Is64: true, Files: files, CabMode: mode}
		out := control(c, p, mode+".msi")
		r := runMsiexec(c, mode+"-install.log", "/i", out)
		e := filepath.Join(pf64(), "MsiSpecTest", "empty.txt")
		st, err := os.Stat(e)
		size := int64(-1)
		if err == nil {
			size = st.Size()
		}
		c.logf("RESULT %s embedded: install %d, empty.txt present=%v size=%d", mode, r.Exit, err == nil, size)
		if r.Exit == 0 {
			runMsiexec(c, mode+"-uninstall.log", "/x", out)
		}
	}
	// external cabinet
	p := &pkg{Name: "cab05x", Is64: true, Files: files, CabMode: "go-mszip"}
	out := control(c, p, "external.msi")
	rdb, _ := openRawMSI(mustRead(out))
	cab := rdb.Streams["cab1.cab"].Data
	os.WriteFile(filepath.Join(filepath.Dir(out), "cab1.cab"), cab, 0o644)
	db, _ := openDB(out, dbTransact)
	_, err := query(db, "UPDATE `Media` SET `Cabinet`='cab1.cab' WHERE `DiskId`=1")
	_, err2 := query(db, "DELETE FROM `_Streams` WHERE `Name`='cab1.cab'")
	c.logf("switch to external cab: %v %v commit %v", err, err2, commit(db))
	closeHandle(db)
	r := runMsiexec(c, "external-install.log", "/i", out)
	_, serr := os.Stat(filepath.Join(pf64(), "MsiSpecTest", "empty.txt"))
	c.logf("RESULT external cab: install %d, empty.txt present=%v", r.Exit, serr == nil)
	if r.Exit == 0 {
		runMsiexec(c, "external-uninstall.log", "/x", out)
	}
	os.Remove(filepath.Join(filepath.Dir(out), "cab1.cab"))
	r = runMsiexec(c, "external-missing-install.log", "/i", out)
	c.logf("RESULT external cab missing: install %d", r.Exit)
}

func caseCab06() {
	c := newCase("CAB-06", "two-cabinets")
	defer c.close()
	a := []byte(strings.Repeat("A", 1000))
	b := []byte(strings.Repeat("B", 1000))
	for _, v := range []struct {
		name  string
		media [][]string
	}{
		{"correct", [][]string{{"1", "1", "", "#cab1.cab", "", ""}, {"2", "2", "", "#cab2.cab", "", ""}}},
		{"lastsequence-too-high", [][]string{{"1", "2", "", "#cab1.cab", "", ""}, {"2", "3", "", "#cab2.cab", "", ""}}},
		{"lastsequence-decreasing", [][]string{{"1", "2", "", "#cab1.cab", "", ""}, {"2", "1", "", "#cab2.cab", "", ""}}},
		{"cabinet-name-case", [][]string{{"1", "1", "", "#CAB1.cab", "", ""}, {"2", "2", "", "#cab2.cab", "", ""}}},
	} {
		p := &pkg{Name: "cab06", Is64: true, Files: []payload{{Key: "f_a", Name: "a.txt", Data: a}, {Key: "f_b", Name: "b.txt", Data: b}}, SkipCab: true}
		out := control(c, p, v.name+".msi")
		db, _ := openDB(out, dbTransact)
		query(db, "DELETE FROM `Media`")
		for _, m := range v.media {
			query(db, fmt.Sprintf("INSERT INTO `Media` (`DiskId`,`LastSequence`,`Cabinet`) VALUES (%s, %s, '%s')", m[0], m[1], m[3]))
		}
		for i, f := range p.Files {
			cab, _ := writeCab([]cabFile{{Name: f.Key, Data: f.Data, Date: 0x21, Attrib: 0x20}}, cabOpts{Compress: 1})
			cp, _ := filepath.Abs(c.path(fmt.Sprintf("%s-cab%d.cab", v.name, i+1)))
			os.WriteFile(cp, cab, 0o644)
			c.logf("insert cab%d: %v", i+1, insertStream(db, "_Streams", fmt.Sprintf("cab%d.cab", i+1), cp))
		}
		c.logf("commit %v", commit(db))
		closeHandle(db)
		// word count must say compressed
		r := runMsiexec(c, v.name+"-install.log", "/i", out)
		ga, _ := os.ReadFile(filepath.Join(pf64(), "MsiSpecTest", "a.txt"))
		gb, _ := os.ReadFile(filepath.Join(pf64(), "MsiSpecTest", "b.txt"))
		c.logf("RESULT %s: install %d, a.txt ok=%v, b.txt ok=%v", v.name, r.Exit, bytes.Equal(ga, a), bytes.Equal(gb, b))
		if r.Exit == 0 {
			runMsiexec(c, v.name+"-uninstall.log", "/x", out)
		}
	}
}

func caseRust11() {
	c := newCase("RUST-11", "msifilehash")
	defer c.close()
	data := []byte("Hello from a clean-room MSI.\r\n")
	for _, v := range []struct {
		name, table string
		right       bool
	}{
		{"msifilehash-correct", "MsiFileHash", true},
		{"msifilehash-wrong", "MsiFileHash", false},
		{"filehash-table", "FileHash", true},
	} {
		p := &pkg{Name: "rust11", Is64: true, Files: []payload{{Key: "f_readme", Name: "readme.txt", Data: data}}}
		hp := c.path("h.txt")
		os.WriteFile(hp, data, 0o644)
		h, _ := fileHash(hp)
		if !v.right {
			h[0]++
		}
		row := []string{"f_readme", "0", fmt.Sprint(h[0]), fmt.Sprint(h[1]), fmt.Sprint(h[2]), fmt.Sprint(h[3])}
		if v.table == "FileHash" {
			schema["FileHash"] = schema["MsiFileHash"]
		}
		p.Extra = map[string][][]string{v.table: {row}}
		out := control(c, p, v.name+".msi")
		target := filepath.Join(pf64(), "MsiSpecTest", "readme.txt")
		r := runMsiexec(c, v.name+"-install.log", "/i", out)
		// corrupt same-size and repair with default REINSTALLMODE (omus: o = older; unversioned uses hash)
		os.WriteFile(target, bytes.ToUpper(data), 0o644)
		rr := runMsiexec(c, v.name+"-repair-omus.log", "/fo", out)
		g1, _ := os.ReadFile(target)
		os.WriteFile(target, bytes.ToUpper(data), 0o644)
		rr2 := runMsiexec(c, v.name+"-repair-c.log", "/fc", out)
		g2, _ := os.ReadFile(target)
		os.WriteFile(target, bytes.ToUpper(data), 0o644)
		rr3 := runMsiexec(c, v.name+"-repair-e.log", "/fe", out)
		g3, _ := os.ReadFile(target)
		c.logf("RESULT %s: install %d; corrupted same-size file restored by /fo=%v (%d), /fc=%v (%d), /fe=%v (%d)", v.name, r.Exit, bytes.Equal(g1, data), rr.Exit, bytes.Equal(g2, data), rr2.Exit, bytes.Equal(g3, data), rr3.Exit)
		runMsiexec(c, v.name+"-uninstall.log", "/x", out)
	}
}
