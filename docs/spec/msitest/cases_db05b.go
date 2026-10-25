package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
)

func init() { cases["DB-05B"] = caseDB05B }

// caseDB05B patches the refcount word of one pool entry and reads it natively.
func caseDB05B() {
	c := newCase("DB-05", "refcount-read")
	defer c.close()
	src, _ := filepath.Abs(filepath.Join(resultsDir, "DB-05", "refcount-overflow", "ref32767.msi"))
	raw := mustRead(src)
	rdb, err := openRawMSI(raw)
	if err != nil {
		c.logf("%v", err)
		return
	}
	id := rdb.idOf("same")
	sp := rdb.TableStr["_StringPool"]
	for _, ref := range []uint16{1, 32767, 32768, 32769, 65535, 0} {
		d := append([]byte(nil), sp.Data...)
		binary.LittleEndian.PutUint16(d[4+4*(int(id)-1)+2:], ref)
		path := writeVariant(c, fmt.Sprintf("ref-%d.msi", ref), rdb.f.patchStream(sp, d))
		db, err := openDB(path, dbReadOnly)
		if err != nil {
			c.logf("ref %d: open %v", ref, err)
			continue
		}
		r, err := query(db, "SELECT `V` FROM `Bulk` WHERE `K`=1")
		closeHandle(db)
		// commit through native and see what it writes back
		db, _ = openDB(path, dbTransact)
		_, e1 := query(db, "INSERT INTO `Bulk` (`K`,`V`) VALUES (999999, 'other')")
		e2 := commit(db)
		closeHandle(db)
		after, _ := openRawMSI(mustRead(path))
		got := "missing"
		if i := after.idOf("same"); i != 0 {
			got = fmt.Sprint(after.Pool[i].Ref)
		}
		c.logf("RESULT stored refcount %d: native read K=1 -> %q err=%v; after native insert+commit (%v/%v) refcount -> %s", ref, r, err, e1, e2, got)
	}
}
