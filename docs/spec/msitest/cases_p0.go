package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

func init() {
	cases["VEC"] = caseVectors
	cases["ENV-01"] = caseENV01
	cases["INST-01"] = caseINST01
	cases["MSIDB-01"] = caseMSIDB01
	cases["SUM-01"] = caseSUM01
	cases["CAB-01"] = caseCAB01
	cases["MSZIP-01"] = caseMSZIP01
	cases["MSIDB-03"] = caseMSIDB03
	cases["FILE-02"] = caseFILE02
}

func u16(s string) []uint16 { return utf16.Encode([]rune(s)) }

func caseVectors() {
	c := newCase("VEC", "spec-vectors")
	defer c.close()
	// A.1 stream names
	for _, v := range []struct {
		name  string
		table bool
		want  string
	}{
		{"Property", true, "4840 4559 44F2 4568 4737"},
		{"_StringPool", true, "4840 3F3F 4577 446C 3E6A 44B2 482F"},
		{"_Tables", true, "4840 3F7F 4164 422F 4836"},
		{"Binary.WixCA", false, "430B 4131 4735 403E 46EC 3A8C"},
		{"cab1.cab", false, "4126 3865 41BE 4164"},
		{"My-File", false, "4716 002D 430F 422F"},
	} {
		var parts []string
		for _, u := range encodeStreamName(v.name, v.table) {
			parts = append(parts, fmt.Sprintf("%04X", u))
		}
		got := strings.Join(parts, " ")
		c.logf("A.1 %-14s %s (want %s) %v", v.name, got, v.want, got == v.want)
	}
	// A.2 compound file
	small := bytes.Repeat([]byte("0123456789"), 10)
	large := make([]byte, 5000)
	for i := range large {
		large[i] = byte(i)
	}
	for _, m := range []int{3, 4} {
		b, err := writeCFB([]cfbStream{{u16("Small"), small}, {u16("Large"), large}}, cfbWriteOpts{Major: m, CLSID: msiCLSID})
		if err != nil {
			c.logf("A.2 v%d error %v", m, err)
			continue
		}
		want := map[int]string{3: "37d8668bf78bc6da2c9b3c6fa3da9c0ffb6824baee21bf13ff9b02dd74e08aed", 4: "acb3c3db476f1d8aebfac49e89b86895756faa24288228a61f617d3c4d54956a"}[m]
		c.logf("A.2 v%d %d bytes sha %s match=%v", m, len(b), sha(b), sha(b) == want)
		os.WriteFile(c.path(fmt.Sprintf("a2-v%d.cfb", m)), b, 0o644)
		f, err := readCFB(b)
		if err == nil {
			c.logf("    reread: Small ok=%v Large ok=%v", bytes.Equal(f.ByName["Small"].Data, small), bytes.Equal(f.ByName["Large"].Data, large))
		}
	}
	// A.4 checksum
	d := []byte("Hello, cabinet!\r\nMSI\r\n")
	cs := cabChecksum(d, 0) ^ (uint32(len(d)) | uint32(len(d))<<16)
	c.logf("A.4 checksum %#08x want 0x25411F35 match=%v", cs, cs == 0x25411F35)
	// A.4 stored cabinet vector (chapter 04 Â§16.2)
	date := uint16((2024-1980)<<9 | 6<<5 | 15)
	tm := uint16(13<<11 | 45<<5 | 30>>1)
	cab, _ := writeCab([]cabFile{{"fil_a", []byte("Hello, cabinet!\r\n"), date, tm, 0x20}, {"fil_b", []byte("MSI\r\n"), date, tm, 0x21}}, cabOpts{SetID: 0x1234})
	c.logf("A.4 stored cab %d bytes sha %s match=%v", len(cab), sha(cab), sha(cab) == "81e6018c16afa762ca19768c9c3ce0eed5aa44b5ef26ea555da92a248bbe4ace")
	os.WriteFile(c.path("stored.cab"), cab, 0o644)
	// A.6 MsiFileHash
	p := c.path("hash.txt")
	data := []byte("Hello from a clean-room MSI.\r\n")
	os.WriteFile(p, data, 0o644)
	h, err := fileHash(p)
	m := md5.Sum(data)
	c.logf("A.6 MsiGetFileHashW %v err=%v; md5 %x", h, err, m)
	want := [4]int32{-215670612, -989844572, -2054495708, 245970701}
	c.logf("A.6 native matches spec vector: %v", h == want)
	var fromMD5 [4]int32
	for i := 0; i < 4; i++ {
		fromMD5[i] = int32(binary.LittleEndian.Uint32(m[4*i:]))
	}
	c.logf("A.6 MD5 as 4 LE int32 = %v, equals native: %v", fromMD5, fromMD5 == h)
	// A.9-ish reproducible product code not needed natively.
}

func caseENV01() {
	c := newCase("ENV-01", "inventory")
	defer c.close()
	c.logf("OS: %s", strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-Command", "$o=Get-CimInstance Win32_OperatingSystem; \"$($o.Caption) $($o.Version) build $($o.BuildNumber) $($o.OSArchitecture)\"")))
	c.logf("UBR: %s", strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-Command", "(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion').UBR")))
	c.logf("Locale: %s / UI %s", strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-Command", "(Get-WinSystemLocale).Name")), strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-Command", "(Get-UICulture).Name")))
	c.logf("Privilege: %s", strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-Command", "([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)")))
	for _, f := range []string{`C:\Windows\System32\msi.dll`, `C:\Windows\System32\msiexec.exe`, `C:\Windows\System32\cabinet.dll`, `C:\Windows\SysWOW64\msi.dll`, `C:\Windows\System32\makecab.exe`} {
		v := strings.TrimSpace(cmdOut("powershell", "-NoProfile", "-Command", "[Diagnostics.FileVersionInfo]::GetVersionInfo('"+f+"').FileVersion"))
		c.logf("%s version %s sha256 %s", f, v, shaFile(f))
	}
	c.logf("Go: %s", strings.TrimSpace(cmdOut("go", "version")))
	// control package via native API
	p := &pkg{Name: "env01", Is64: true}
	out := control(c, p, "control.msi")
	if out == "" {
		return
	}
	db, err := openDB(out, dbReadOnly)
	if err != nil {
		c.logf("open: %v", err)
		return
	}
	defer closeHandle(db)
	rows, err := query(db, "SELECT `Name` FROM `_Tables`")
	c.logf("native _Tables (%v): %v", err, rows)
	for _, r := range rows {
		t := r[0]
		rr, err := query(db, "SELECT * FROM `"+t+"`")
		c.logf("  %s: %d rows err=%v", t, len(rr), err)
	}
}

func caseINST01() {
	for _, v := range []struct {
		name string
		is64 bool
	}{{"x64-substitute", true}, {"intel-substitute", false}} {
		c := newCase("INST-01", v.name)
		c.logf("Published test-vectors/minimal.msi is not in the repository; this is a labelled substitute built with the native importer.")
		p := &pkg{Name: "run01" + v.name, Is64: v.is64}
		out := control(c, p, "control.msi")
		if out == "" {
			c.close()
			continue
		}
		dir := pf32()
		if v.is64 {
			dir = pf64()
		}
		target := filepath.Join(dir, "MsiSpecTest", "readme.txt")
		_, _, ok := installCycle(c, out, map[string][]byte{target: p.Files[0].Data})
		c.logf("RESULT install+uninstall ok=%v target=%s", ok, target)
		c.close()
	}
}

func caseMSIDB01() {
	c := newCase("MSIDB-01", "native-written-catalog")
	defer c.close()
	p := &pkg{Name: "db01", Is64: true}
	out := control(c, p, "control.msi")
	if out == "" {
		return
	}
	raw, _ := os.ReadFile(out)
	rdb, err := openRawMSI(raw)
	if err != nil {
		c.logf("raw reader: %v", err)
		return
	}
	c.logf("CFB major %d, sector %d, codepage %d, longRefs %v, pool entries %d", rdb.f.Major, rdb.f.SS, rdb.CodePage, rdb.LongRefs, len(rdb.Pool)-1)
	free := 0
	for i, e := range rdb.Pool {
		if i > 0 && e.Len == 0 {
			free++
		}
	}
	c.logf("free pool slots: %d", free)
	// pool order: sorted?
	var prev string
	sortedPool := true
	for i, e := range rdb.Pool {
		if i == 0 || e.Len == 0 {
			continue
		}
		if string(e.Data) < prev {
			sortedPool = false
		}
		prev = string(e.Data)
	}
	c.logf("native pool sorted by bytes: %v (first ids: %q)", sortedPool, firstStrings(rdb, 12))
	// refcounts
	rc := rdb.refcounts()
	mism := 0
	for id, e := range rdb.Pool {
		if id == 0 || e.Len == 0 {
			continue
		}
		if rc[uint32(id)] != e.Ref {
			mism++
			if mism <= 5 {
				c.logf("  refcount mismatch id %d %q stored %d counted %d", id, string(e.Data), e.Ref, rc[uint32(id)])
			}
		}
	}
	c.logf("refcount mismatches (counting _Tables and _Columns): %d", mism)
	c.logf("raw _Tables order: %v", rdb.Tables)
	db, _ := openDB(out, dbReadOnly)
	defer closeHandle(db)
	nt, _ := query(db, "SELECT `Name` FROM `_Tables`")
	var native []string
	for _, r := range nt {
		native = append(native, r[0])
	}
	c.logf("native _Tables order: %v", native)
	allEqual := true
	for _, t := range rdb.Tables {
		rows, cols, err := rdb.rows(t)
		if err != nil {
			c.logf("  %s raw decode error %v", t, err)
			allEqual = false
			continue
		}
		var names []string
		for _, col := range cols {
			names = append(names, "`"+col.Name+"`")
		}
		nrows, err := query(db, "SELECT "+strings.Join(names, ",")+" FROM `"+t+"`")
		if err != nil {
			c.logf("  %s native query error %v", t, err)
			allEqual = false
			continue
		}
		same := len(nrows) == len(rows)
		if same {
			for i := range rows {
				for j := range cols {
					rv := rows[i][j].Text
					if rows[i][j].Null {
						rv = nullMark
					}
					nv := nrows[i][j]
					if strings.HasPrefix(nv, "<stream") && rv == "<binary>" {
						continue
					}
					if rv != nv {
						same = false
						c.logf("  %s row %d col %s raw %q native %q", t, i, cols[j].Name, rv, nv)
					}
				}
			}
		}
		var types []string
		for _, col := range cols {
			types = append(types, fmt.Sprintf("%s=%#04x", col.Name, col.Type))
		}
		c.logf("  %-24s rows=%d sorted=%v equal-to-native=%v types=%s", t, len(rows), keysSorted(rows, cols), same, strings.Join(types, " "))
		if !same {
			allEqual = false
		}
	}
	// streams
	var sn []string
	for _, e := range rdb.f.Entries {
		if e.Type == 2 && e.Parent == 0 {
			n, tbl := decodeStreamName(e.Name)
			if tbl {
				n = "[T]" + n
			}
			sn = append(sn, fmt.Sprintf("%s(%d)", strings.ReplaceAll(n, "\x05", "\\u0005"), e.Size))
		}
	}
	c.logf("streams: %s", strings.Join(sn, " "))
	c.logf("RESULT raw decode equals native query for every table: %v", allEqual)
}

func firstStrings(db *msiDB, n int) []string {
	var out []string
	for i := 1; i < len(db.Pool) && len(out) < n; i++ {
		out = append(out, string(db.Pool[i].Data))
	}
	return out
}

func caseSUM01() {
	c := newCase("SUM-01", "native-summary")
	defer c.close()
	p := &pkg{Name: "sum01", Is64: true}
	out := control(c, p, "control.msi")
	if out == "" {
		return
	}
	si, err := openSummary(0, out, 0)
	if err != nil {
		c.logf("open summary: %v", err)
		return
	}
	for _, pid := range []uint32{1, 2, 3, 4, 7, 9, 12, 13, 14, 15, 18, 19, 5, 6, 8} {
		v, err := getSummary(si, pid)
		c.logf("native PID %2d VT %2d int %d ft %08x%08x text %q err=%v", pid, v.VT, v.Int, v.FT[1], v.FT[0], v.Text, err)
	}
	closeHandle(si)
	raw, _ := os.ReadFile(out)
	f, _ := readCFB(raw)
	e := f.ByName["\x05SummaryInformation"]
	if e == nil {
		c.logf("no summary stream")
		return
	}
	d := e.Data
	os.WriteFile(c.path("summary.bin"), d, 0o644)
	le := binary.LittleEndian
	c.logf("raw: byteorder %#x version %d sysid %#08x nsets %d fmtid % x off %d size %d", le.Uint16(d), le.Uint16(d[2:]), le.Uint32(d[4:]), le.Uint32(d[0x18:]), d[0x1C:0x2C], le.Uint32(d[0x2C:]), len(d))
	sec := d[0x30:]
	n := int(le.Uint32(sec[4:]))
	c.logf("section size %d, %d properties", le.Uint32(sec), n)
	var pids []uint32
	for i := 0; i < n; i++ {
		pid := le.Uint32(sec[8+8*i:])
		off := le.Uint32(sec[12+8*i:])
		vt := le.Uint16(sec[off:])
		var val string
		switch vt {
		case 2:
			val = fmt.Sprint(le.Uint16(sec[off+4:]))
		case 3:
			val = fmt.Sprint(int32(le.Uint32(sec[off+4:])))
		case 30:
			l := le.Uint32(sec[off+4:])
			val = fmt.Sprintf("%q (count %d)", string(sec[off+8:off+8+l]), l)
		case 64:
			val = fmt.Sprintf("%08x%08x", le.Uint32(sec[off+8:]), le.Uint32(sec[off+4:]))
		}
		pids = append(pids, pid)
		c.logf("  raw PID %2d off %3d VT %2d %s", pid, off, vt, val)
	}
	c.logf("PIDs in stored order: %v", pids)
}

func caseCAB01() {
	data := []byte(strings.Repeat("CAB-01 payload line with some repetition 0123456789\r\n", 400))
	for _, mode := range []string{"uncompressed", "uncompressed-resolvesource", "uncompressed-longonly-dir", "makecab-none", "makecab-mszip", "go-stored", "go-mszip"} {
		c := newCase("CAB-01", mode)
		p := &pkg{Name: "cab01" + mode, Is64: true, CabMode: mode, Files: []payload{{Key: "f_data", Name: "data.txt", Data: data}}}
		if strings.HasPrefix(mode, "uncompressed") {
			p.CabMode = ""
			p.SkipCab = true
			p.LongOnlyDir = mode == "uncompressed-longonly-dir"
			if mode == "uncompressed-resolvesource" {
				p.ExtraSequence = [][]string{{"ResolveSource", "NOT Installed", "850"}}
			}
			p.WordCount = -1 // written as 0: long names, uncompressed source
		}
		out := control(c, p, "pkg.msi")
		if out == "" {
			c.close()
			continue
		}
		if strings.HasPrefix(mode, "uncompressed") {
			// source layout: SourceDir\ProgramFiles64Folder? '.' => INSTALLDIR long name under root
			src := filepath.Join(filepath.Dir(out), "MsiSpecTest")
			os.MkdirAll(src, 0o755)
			os.WriteFile(filepath.Join(src, "data.txt"), data, 0o644)
		} else {
			raw, _ := os.ReadFile(out)
			rdb, _ := openRawMSI(raw)
			if e := rdb.Streams["cab1.cab"]; e != nil {
				os.WriteFile(c.path("cab1.cab"), e.Data, 0o644)
				files, err := readCab(e.Data)
				c.logf("independent reader: %d files err=%v", len(files), err)
				for _, f := range files {
					c.logf("  %s %d bytes equal=%v date %#04x time %#04x attr %#02x", f.Name, len(f.Data), bytes.Equal(f.Data, data), f.Date, f.Time, f.Attrib)
				}
				x := c.path("expand")
				os.MkdirAll(x, 0o755)
				c.logf("native expand.exe: %s", strings.TrimSpace(lastLine(cmdOut("expand.exe", c.path("cab1.cab"), "-F:*", x))))
				got, err := readExpanded(x, "f_data")
				c.logf("  FDI extracted equal=%v err=%v", bytes.Equal(got, data), err)
			}
		}
		target := filepath.Join(pf64(), "MsiSpecTest", "data.txt")
		_, _, ok := installCycle(c, out, map[string][]byte{target: data})
		c.logf("RESULT %s install ok=%v", mode, ok)
		c.close()
	}
}

func lastLine(s string) string {
	ls := strings.Split(strings.TrimSpace(s), "\n")
	return ls[len(ls)-1]
}

func crossBlockData() []byte {
	// 100 KB with repeated 1 KB patterns that recur across the 32 KiB boundaries.
	pat := make([]byte, 1024)
	seed := uint32(12345)
	for i := range pat {
		seed = seed*1103515245 + 12345
		pat[i] = byte(seed >> 16)
	}
	var b []byte
	for len(b) < 100*1024 {
		b = append(b, pat...)
		b = append(b, []byte(fmt.Sprintf("<%d>", len(b)))...)
	}
	return b[:100*1024]
}

func caseMSZIP01() {
	data := crossBlockData()
	for _, mode := range []string{"makecab-mszip", "go-mszip", "go-nodict", "go-wrongdict"} {
		c := newCase("MSZIP-01", mode)
		p := &pkg{Name: "cab02" + mode, Is64: true, CabMode: mode, Files: []payload{{Key: "f_big", Name: "big.bin", Data: data}}}
		out := control(c, p, "pkg.msi")
		if out == "" {
			c.close()
			continue
		}
		raw, _ := os.ReadFile(out)
		rdb, _ := openRawMSI(raw)
		cab := rdb.Streams["cab1.cab"].Data
		os.WriteFile(c.path("cab1.cab"), cab, 0o644)
		nData := binary.LittleEndian.Uint16(cab[36+4:])
		c.logf("cab %d bytes, %d CFDATA blocks, typeCompress %#x", len(cab), nData, binary.LittleEndian.Uint16(cab[36+6:]))
		files, err := readCab(cab)
		if err == nil && len(files) == 1 {
			c.logf("independent reader (history carried): equal=%v", bytes.Equal(files[0].Data, data))
		} else {
			c.logf("independent reader error: %v", err)
		}
		x := c.path("expand")
		os.MkdirAll(x, 0o755)
		c.logf("native expand.exe: %s", strings.TrimSpace(lastLine(cmdOut("expand.exe", c.path("cab1.cab"), "-F:*", x))))
		got, err := readExpanded(x, "f_big")
		c.logf("  FDI extracted equal=%v err=%v %s", bytes.Equal(got, data), err, diffSummary(got, data))
		target := filepath.Join(pf64(), "MsiSpecTest", "big.bin")
		inst := runMsiexec(c, "install.log", "/i", out)
		got2, err := os.ReadFile(target)
		c.logf("  installed equal=%v err=%v %s", bytes.Equal(got2, data), err, diffSummary(got2, data))
		if inst.Exit == 0 || exists(target) {
			runMsiexec(c, "uninstall.log", "/x", out)
		}
		c.close()
	}
}

func diffSummary(got, want []byte) string {
	if got == nil {
		return ""
	}
	n := 0
	first := -1
	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			n++
			if first < 0 {
				first = i
			}
		}
	}
	if n == 0 && len(got) == len(want) {
		return ""
	}
	return fmt.Sprintf("(len %d vs %d, %d differing bytes, first at %d)", len(got), len(want), n, first)
}

func caseMSIDB03() {
	c := newCase("MSIDB-03", "property-row-order")
	defer c.close()
	p := &pkg{Name: "db03", Is64: true}
	out := control(c, p, "sorted.msi")
	if out == "" {
		return
	}
	raw, _ := os.ReadFile(out)
	rdb, _ := openRawMSI(raw)
	rows, cols, _ := rdb.rows("Property")
	c.logf("control Property rows sorted=%v (%d rows)", keysSorted(rows, cols), len(rows))
	// swap first and last row
	sh := append([][]rawCell(nil), rows...)
	sh[0], sh[len(sh)-1] = sh[len(sh)-1], sh[0]
	data := rdb.encodeRows("Property", sh)
	mut := rdb.f.patchStream(rdb.TableStr["Property"], data)
	sp := c.path("shuffled.msi")
	os.WriteFile(sp, mut, 0o644)
	sp, _ = filepath.Abs(sp)
	c.logf("shuffled.msi sha256 %s (Property rows 0 and %d swapped, nothing else)", sha(mut), len(sh)-1)
	for _, v := range []struct{ name, path string }{{"sorted", out}, {"shuffled", sp}} {
		db, err := openDB(v.path, dbReadOnly)
		if err != nil {
			c.logf("%s: open %v", v.name, err)
			continue
		}
		r, err := query(db, "SELECT `Value` FROM `Property` WHERE `Property`='ProductCode'")
		c.logf("%s: ProductCode query -> %v err=%v", v.name, r, err)
		all, err := query(db, "SELECT `Property` FROM `Property`")
		c.logf("%s: all Property rows -> %d err=%v", v.name, len(all), err)
		closeHandle(db)
	}
	r := runMsiexec(c, "install-shuffled.log", "/i", sp)
	c.logf("RESULT shuffled install exit %d (spec expects 1605 with Note 2211)", r.Exit)
	if r.Exit == 0 {
		runMsiexec(c, "uninstall-shuffled.log", "/x", sp)
	}
}

func caseFILE02() {
	c := newCase("FILE-02", "file-hash")
	defer c.close()
	inputs := map[string][]byte{
		"empty.bin":  {},
		"ascii.txt":  []byte("Hello from a clean-room MSI.\r\n"),
		"binary.bin": crossBlockData()[:4099],
	}
	for name, d := range inputs {
		p := c.path(name)
		os.WriteFile(p, d, 0o644)
		h, err := fileHash(p)
		m := md5.Sum(d)
		var le [4]int32
		for i := 0; i < 4; i++ {
			le[i] = int32(binary.LittleEndian.Uint32(m[4*i:]))
		}
		c.logf("%-10s native %v err=%v | md5 %x | md5 as LE int32 %v equal=%v", name, h, err, m, le, le == h)
	}
}

// expand.exe names the output after the cabinet when it holds one file.
func readExpanded(dir, key string) ([]byte, error) {
	if b, err := os.ReadFile(filepath.Join(dir, key)); err == nil {
		return b, nil
	}
	return os.ReadFile(filepath.Join(dir, "cab1.cab"))
}
