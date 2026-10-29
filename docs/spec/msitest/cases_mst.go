package main

// MSIDB-12: the stream layout of a transform. msi.dll's own
// MsiDatabaseGenerateTransformW writes transforms between two databases that
// differ in known ways, and this case dumps every stream of the result so
// that section 3 can describe the format from observed bytes. The transform
// is then applied with MSITRANSFORM_ERROR_VIEWTRANSFORM and _TransformView is
// logged, which names each change in the engine's own terms.

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"unsafe"
)

var pMsiDatabaseApplyTransformW = msiDLL.NewProc("MsiDatabaseApplyTransformW")

func init() { cases["MSIDB-12"] = caseMSIDB12 }

const transformViewTransform = 0x100 // MSITRANSFORM_ERROR_VIEWTRANSFORM

// mstWideColumns is the column count of MstWide, past the 16 bits of a
// record mask.
const mstWideColumns = 20

// mstCommon are the statements both databases run: tables that the
// transform then changes.
func mstCommon() []string {
	wide := make([]string, 0, mstWideColumns)
	names := make([]string, 0, mstWideColumns)
	values := make([]string, 0, mstWideColumns)
	for i := 1; i <= mstWideColumns; i++ {
		wide = append(wide, fmt.Sprintf("`C%02d` CHAR(20)", i))
		names = append(names, fmt.Sprintf("`C%02d`", i))
		values = append(values, fmt.Sprintf("'w%02d'", i))
	}
	wide[0] = "`C01` CHAR(20) NOT NULL"
	stmts := []string{
		"CREATE TABLE `MstKeep` (`Key` CHAR(72) NOT NULL, `Num` SHORT, `Big` LONG, `Text` CHAR(255) PRIMARY KEY `Key`)",
		"INSERT INTO `MstKeep` (`Key`, `Num`, `Big`, `Text`) VALUES ('a', 1, 100000, 'alpha')",
		"INSERT INTO `MstKeep` (`Key`, `Num`, `Big`, `Text`) VALUES ('b', 2, 200000, 'beta')",
		"INSERT INTO `MstKeep` (`Key`, `Num`, `Big`, `Text`) VALUES ('c', 3, 300000, 'gamma')",
		"CREATE TABLE `MstTwoKey` (`K1` CHAR(72) NOT NULL, `K2` SHORT NOT NULL, `Val` CHAR(72) PRIMARY KEY `K1`, `K2`)",
		"INSERT INTO `MstTwoKey` (`K1`, `K2`, `Val`) VALUES ('x', 1, 'one')",
		"INSERT INTO `MstTwoKey` (`K1`, `K2`, `Val`) VALUES ('x', 2, 'two')",
		"CREATE TABLE `MstWide` (" + strings.Join(wide, ", ") + " PRIMARY KEY `C01`)",
		"INSERT INTO `MstWide` (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(values, ", ") + ")",
		"CREATE TABLE `MstAlter` (`Key` CHAR(72) NOT NULL PRIMARY KEY `Key`)",
		"INSERT INTO `MstAlter` (`Key`) VALUES ('k')",
		"INSERT INTO `MstKeep` (`Key`, `Num`, `Big`, `Text`) VALUES ('e', 5, 500000, 'epsilon')",
	}
	// Rows v02 to v07 of MstWide, each changed in another way.
	for row := 2; row <= 7; row++ {
		vals := make([]string, 0, mstWideColumns)
		for i := 1; i <= mstWideColumns; i++ {
			vals = append(vals, fmt.Sprintf("'v%02d_%02d'", row, i))
		}
		vals[0] = fmt.Sprintf("'v%02d'", row)
		if row == 6 {
			continue // inserted by the variant
		}
		stmts = append(stmts, "INSERT INTO `MstWide` ("+strings.Join(names, ", ")+") VALUES ("+strings.Join(vals, ", ")+")")
	}
	return stmts
}

// mstWideInsert is row v06 of MstWide, which only the variant has.
func mstWideInsert() string {
	names := make([]string, 0, mstWideColumns)
	vals := make([]string, 0, mstWideColumns)
	for i := 1; i <= mstWideColumns; i++ {
		names = append(names, fmt.Sprintf("`C%02d`", i))
		vals = append(vals, fmt.Sprintf("'v06_%02d'", i))
	}
	vals[0] = "'v06'"
	return "INSERT INTO `MstWide` (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(vals, ", ") + ")"
}

// mstChange is one difference between the reference and the variant.
type mstChange struct {
	what      string
	reference []string
	variant   []string
}

var mstChanges = []mstChange{
	{what: "MstKeep a: Text alpha -> ALPHA (one string column)", variant: []string{"UPDATE `MstKeep` SET `Text`='ALPHA' WHERE `Key`='a'"}},
	{what: "MstKeep b: Num 2 -> NULL, Big 200000 -> -5 (two integer columns)", variant: []string{"UPDATE `MstKeep` SET `Num`=NULL, `Big`=-5 WHERE `Key`='b'"}},
	{what: "MstKeep c: deleted", variant: []string{"DELETE FROM `MstKeep` WHERE `Key`='c'"}},
	{what: "MstKeep d: inserted", variant: []string{"INSERT INTO `MstKeep` (`Key`, `Num`, `Big`, `Text`) VALUES ('d', 4, 400000, 'delta')"}},
	{what: "MstTwoKey x/2: Val two -> TWO; x/3 inserted", variant: []string{
		"UPDATE `MstTwoKey` SET `Val`='TWO' WHERE `K1`='x' AND `K2`=2",
		"INSERT INTO `MstTwoKey` (`K1`, `K2`, `Val`) VALUES ('x', 3, 'three')",
	}},
	{what: "MstWide w01: C18 w18 -> W18 (column 18, past the mask's 16 bits)", variant: []string{"UPDATE `MstWide` SET `C18`='W18' WHERE `C01`='w01'"}},
	{what: "MstWide v02: C05 changed (column 5 of a 20-column table)", variant: []string{"UPDATE `MstWide` SET `C05`='X02_05' WHERE `C01`='v02'"}},
	{what: "MstWide v03: C05 and C18 changed", variant: []string{"UPDATE `MstWide` SET `C05`='X03_05', `C18`='X03_18' WHERE `C01`='v03'"}},
	{what: "MstWide v04: C16 changed", variant: []string{"UPDATE `MstWide` SET `C16`='X04_16' WHERE `C01`='v04'"}},
	{what: "MstWide v05: C17 changed", variant: []string{"UPDATE `MstWide` SET `C17`='X05_17' WHERE `C01`='v05'"}},
	{what: "MstWide v06: inserted (20 columns)", variant: []string{mstWideInsert()}},
	{what: "MstWide v07: deleted", variant: []string{"DELETE FROM `MstWide` WHERE `C01`='v07'"}},
	{what: "MstKeep e: Text epsilon -> NULL", variant: []string{"UPDATE `MstKeep` SET `Text`=NULL WHERE `Key`='e'"}},
	{what: "MstAlter: column Extra added, row k gets Extra 'e'", variant: []string{
		"ALTER TABLE `MstAlter` ADD `Extra` CHAR(20)",
		"UPDATE `MstAlter` SET `Extra`='e' WHERE `Key`='k'",
	}},
	{what: "MstNew: table created with one row", variant: []string{
		"CREATE TABLE `MstNew` (`Key` CHAR(72) NOT NULL, `Num` SHORT PRIMARY KEY `Key`)",
		"INSERT INTO `MstNew` (`Key`, `Num`) VALUES ('n', 7)",
	}},
	{what: "MstOld: table only in the reference, dropped", reference: []string{
		"CREATE TABLE `MstOld` (`Key` CHAR(72) NOT NULL PRIMARY KEY `Key`)",
		"INSERT INTO `MstOld` (`Key`) VALUES ('o')",
	}},
	{what: "Property: ProductName changed, MstProp inserted", variant: []string{
		"UPDATE `Property` SET `Value`='MsiSpec Test msidb12 changed' WHERE `Property`='ProductName'",
		"INSERT INTO `Property` (`Property`, `Value`) VALUES ('MstProp', 'added')",
	}},
}

// mstDatabase copies base to path and runs the common statements and then
// the reference or variant ones.
func mstDatabase(base, path string, variant bool) error {
	data, err := os.ReadFile(base)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	db, err := openDB(path, dbTransact)
	if err != nil {
		return err
	}
	defer closeHandle(db)
	stmts := mstCommon()
	for _, ch := range mstChanges {
		if variant {
			stmts = append(stmts, ch.variant...)
		} else {
			stmts = append(stmts, ch.reference...)
		}
	}
	for _, s := range stmts {
		if _, err := query(db, s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return commit(db)
}

// dumpTransform logs every entry of a transform file and decodes its string
// pool, so the table streams can be read by string ID.
func dumpTransform(c *caseLog, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		c.logf("read %s: %v", path, err)
		return
	}
	f, err := readCFB(raw)
	if err != nil {
		c.logf("CFB %s: %v", path, err)
		return
	}
	c.logf("file %s: %d bytes, CFB major %d, sector %d", path, len(raw), f.Major, f.SS)
	byName := map[string][]byte{}
	for _, e := range f.Entries {
		name, table := decodeStreamName(e.Name)
		kind := "stream"
		if table {
			kind = "table"
		}
		switch e.Type {
		case 1:
			kind = "storage"
		case 5:
			kind = "root"
		}
		c.logf("entry SID %d parent %d type %d (%s) name %q decoded %q clsid %s size %d", e.SID, e.Parent, e.Type, kind,
			e.NameString(), name, hex.EncodeToString(e.CLSID[:]), e.Size)
		if e.Type == 2 {
			byName[name] = e.Data
			c.logf("  hex %s", hex.EncodeToString(e.Data))
		}
	}
	sp, sd := byName["_StringPool"], byName["_StringData"]
	if len(sp) < 4 {
		c.logf("no _StringPool")
		return
	}
	le := binary.LittleEndian
	hdr := le.Uint32(sp)
	c.logf("_StringPool header %#08x: code page %d, long references %v", hdr, hdr&0x7FFFFFFF, hdr&0x80000000 != 0)
	pos, id := 0, 1
	for i := 4; i+4 <= len(sp); id++ {
		l, r := int(le.Uint16(sp[i:])), le.Uint16(sp[i+2:])
		i += 4
		if l == 0 && r != 0 && i+4 <= len(sp) {
			l = int(le.Uint16(sp[i:])) | int(le.Uint16(sp[i+2:]))<<16
			i += 4
		}
		if pos+l > len(sd) {
			c.logf("string %d overruns _StringData", id)
			return
		}
		c.logf("  string %d (refcount %d): %q", id, r, sd[pos:pos+l])
		pos += l
	}
}

// logTransformSummary logs the summary properties of a transform file.
func logTransformSummary(c *caseLog, path string) {
	h, err := openSummary(0, path, 0)
	if err != nil {
		c.logf("summary %s: %v", path, err)
		return
	}
	defer closeHandle(h)
	for pid := uint32(1); pid <= 19; pid++ {
		p, err := getSummary(h, pid)
		if err != nil || p.VT == 0 || p.VT == 1 {
			continue
		}
		c.logf("  PID %d VT %d int %d (%#x) text %q filetime %v", pid, p.VT, p.Int, uint32(p.Int), p.Text, p.FT)
	}
}

// logTransformView applies mst to a copy of the reference with
// MSITRANSFORM_ERROR_VIEWTRANSFORM and logs _TransformView.
func logTransformView(c *caseLog, reference, mst string) {
	data, err := os.ReadFile(reference)
	if err != nil {
		c.logf("view: %v", err)
		return
	}
	scratch := c.path("view.msi")
	if err := os.WriteFile(scratch, data, 0o644); err != nil {
		c.logf("view: %v", err)
		return
	}
	db, err := openDB(scratch, dbReadOnly)
	if err != nil {
		c.logf("view open: %v", err)
		return
	}
	defer closeHandle(db)
	if err := call("MsiDatabaseApplyTransformW", pMsiDatabaseApplyTransformW, uintptr(db),
		uintptr(unsafe.Pointer(wstr(mst))), transformViewTransform); err != nil {
		c.logf("view apply: %v", err)
		return
	}
	rows, err := query(db, "SELECT `Table`, `Column`, `Row`, `Data`, `Current` FROM `_TransformView`")
	if err != nil {
		c.logf("view query: %v", err)
	}
	for _, r := range rows {
		c.logf("  view %q", r)
	}
}

func caseMSIDB12() {
	c := newCase("MSIDB-12", "transform-streams")
	defer c.close()
	base := control(c, &pkg{Name: "msidb12", Is64: true}, "base.msi")
	if base == "" {
		return
	}
	reference, variant := c.path("reference.msi"), c.path("variant.msi")
	if err := mstDatabase(base, reference, false); err != nil {
		c.logf("reference: %v", err)
		return
	}
	if err := mstDatabase(base, variant, true); err != nil {
		c.logf("variant: %v", err)
		return
	}
	for _, ch := range mstChanges {
		c.logf("change: %s", ch.what)
	}

	// Error conditions and validation as LANG-01 uses them (none), then
	// every error condition and the product code, language and version
	// checks, so that the summary shows where each lands.
	for _, v := range []struct {
		name               string
		errors, validation uintptr
	}{
		{"plain", 0, 0},
		{"flags", 0x3F, 0x0001 | 0x0002 | 0x0008 | 0x0100 | 0x0800},
	} {
		mst := c.path(v.name + ".mst")
		os.Remove(mst)
		db, err := openDB(variant, dbReadOnly)
		if err != nil {
			c.logf("open variant: %v", err)
			return
		}
		ref, err := openDB(reference, dbReadOnly)
		if err != nil {
			closeHandle(db)
			c.logf("open reference: %v", err)
			return
		}
		err = call("MsiDatabaseGenerateTransformW", pMsiDatabaseGenerateTransformW, uintptr(db), uintptr(ref),
			uintptr(unsafe.Pointer(wstr(mst))), 0, 0)
		if err == nil {
			err = call("MsiCreateTransformSummaryInfoW", pMsiCreateTransformSummaryInfo, uintptr(db), uintptr(ref),
				uintptr(unsafe.Pointer(wstr(mst))), v.errors, v.validation)
		}
		closeHandle(ref)
		closeHandle(db)
		if err != nil {
			c.logf("%s: %v", v.name, err)
			continue
		}
		c.logf("## %s.mst (error conditions %#x, validation %#x) sha256 %s", v.name, v.errors, v.validation, shaFile(mst))
		logTransformSummary(c, mst)
		if v.name == "plain" {
			dumpTransform(c, mst)
			logTransformView(c, reference, mst)
		}
	}

	mstCodepages(c, base)
}

// setCodepage sets the database code page through the _ForceCodepage
// pseudo table.
func setCodepage(db uint32, dir string, codepage int) error {
	file := "codepage.idt"
	if err := os.WriteFile(dir+`\`+file, []byte(fmt.Sprintf("\r\n\r\n%d\t_ForceCodepage\r\n", codepage)), 0o644); err != nil {
		return err
	}
	return importIDT(db, dir, file)
}

// codepageOf exports _ForceCodepage and returns its line.
func codepageOf(db uint32, dir string) string {
	if err := exportIDT(db, "_ForceCodepage", dir, "cp-out.idt"); err != nil {
		return err.Error()
	}
	b, err := os.ReadFile(dir + `\cp-out.idt`)
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(string(b))
}

// mstCodepages makes transforms whose variant holds non-ASCII text, once in
// the reference's code page 1252 and once in a variant set to 65001, then
// applies each to a copy of the reference and reads the text and code page
// back. It shows whether a transform carries or changes the code page.
func mstCodepages(c *caseLog, base string) {
	dir := c.path("")

	// A copy of base in code page 1252, the code page msi-builder writes
	// by default, as a second reference.
	ref1252 := c.path("ref1252.msi")
	if data, err := os.ReadFile(base); err == nil && os.WriteFile(ref1252, data, 0o644) == nil {
		if db, err := openDB(ref1252, dbTransact); err == nil {
			if err := setCodepage(db, dir, 1252); err != nil {
				c.logf("ref1252: %v", err)
			}
			c.logf("ref1252: %s, commit %v", codepageOf(db, dir), commit(db))
			closeHandle(db)
		}
	}

	for _, v := range []struct {
		name      string
		reference string
		codepage  int
		text      string
	}{
		{"cp1252", base, 0, "Zurück"},
		{"cp65001", base, 65001, "Назад Zurück 日本"},
		{"ref1252-cp1251", ref1252, 1251, "Назад"},
		{"ref1252-cp65001", ref1252, 65001, "Назад Zurück 日本"},
	} {
		base := v.reference
		variant := c.path(v.name + ".msi")
		data, err := os.ReadFile(base)
		if err == nil {
			err = os.WriteFile(variant, data, 0o644)
		}
		if err != nil {
			c.logf("%s: %v", v.name, err)
			continue
		}
		db, err := openDB(variant, dbTransact)
		if err != nil {
			c.logf("%s: %v", v.name, err)
			continue
		}
		if v.codepage != 0 {
			if err := setCodepage(db, dir, v.codepage); err != nil {
				c.logf("%s: set code page: %v", v.name, err)
			}
		}
		if _, err := query(db, "INSERT INTO `Property` (`Property`, `Value`) VALUES ('MstText', ?)", v.text); err != nil {
			c.logf("%s: insert: %v", v.name, err)
		}
		c.logf("## %s: variant %s, text %q", v.name, codepageOf(db, dir), v.text)
		err = commit(db)
		closeHandle(db)
		if err != nil {
			c.logf("%s: commit: %v", v.name, err)
			continue
		}

		mst := c.path(v.name + ".mst")
		os.Remove(mst)
		vdb, err := openDB(variant, dbReadOnly)
		if err != nil {
			c.logf("%s: %v", v.name, err)
			continue
		}
		rdb, err := openDB(base, dbReadOnly)
		if err == nil {
			err = call("MsiDatabaseGenerateTransformW", pMsiDatabaseGenerateTransformW, uintptr(vdb), uintptr(rdb),
				uintptr(unsafe.Pointer(wstr(mst))), 0, 0)
			if err == nil {
				err = call("MsiCreateTransformSummaryInfoW", pMsiCreateTransformSummaryInfo, uintptr(vdb), uintptr(rdb),
					uintptr(unsafe.Pointer(wstr(mst))), 0, 0)
			}
			closeHandle(rdb)
		}
		closeHandle(vdb)
		if err != nil {
			c.logf("%s: transform: %v", v.name, err)
			continue
		}
		logTransformSummary(c, mst)
		dumpTransform(c, mst)

		applied := c.path(v.name + "-applied.msi")
		data, _ = os.ReadFile(base)
		_ = os.WriteFile(applied, data, 0o644)
		adb, err := openDB(applied, dbTransact)
		if err != nil {
			c.logf("%s: %v", v.name, err)
			continue
		}
		if err := call("MsiDatabaseApplyTransformW", pMsiDatabaseApplyTransformW, uintptr(adb), uintptr(unsafe.Pointer(wstr(mst))), 0); err != nil {
			c.logf("%s: apply: %v", v.name, err)
		}
		rows, err := query(adb, "SELECT `Value` FROM `Property` WHERE `Property`='MstText'")
		c.logf("%s: applied to the reference: %s, MstText %q (%v), equal %v", v.name, codepageOf(adb, dir), rows, err,
			len(rows) == 1 && rows[0][0] == v.text)
		closeHandle(adb)
	}
}
