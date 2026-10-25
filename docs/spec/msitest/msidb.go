package main

// Raw MSI database decoding written from MSISPEC.md section 3.

import (
	"encoding/binary"
	"fmt"
	"sort"
	"unicode/utf16"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz._"

func alphaIndex(r rune) int {
	for i, c := range alphabet {
		if c == r {
			return i
		}
	}
	return -1
}

func encodeStreamName(name string, table bool) []uint16 {
	var out []uint16
	if table {
		out = append(out, 0x4840)
	}
	rs := []rune(name)
	for i := 0; i < len(rs); i++ {
		a := alphaIndex(rs[i])
		if a < 0 {
			out = append(out, utf16.Encode([]rune{rs[i]})...)
			continue
		}
		if i+1 < len(rs) {
			if b := alphaIndex(rs[i+1]); b >= 0 {
				out = append(out, uint16(0x3800+a+(b<<6)))
				i++
				continue
			}
		}
		out = append(out, uint16(0x4800+a))
	}
	return out
}

func decodeStreamName(u []uint16) (name string, table bool) {
	if len(u) > 0 && u[0] == 0x4840 {
		table = true
		u = u[1:]
	}
	var rs []rune
	for _, c := range u {
		switch {
		case c >= 0x3800 && c < 0x4800:
			v := c - 0x3800
			rs = append(rs, rune(alphabet[v&0x3F]), rune(alphabet[(v>>6)&0x3F]))
		case c >= 0x4800 && c < 0x4840:
			rs = append(rs, rune(alphabet[c-0x4800]))
		default:
			rs = append(rs, rune(c))
		}
	}
	return string(rs), table
}

type poolEntry struct {
	Len, Ref uint32
	Long     bool
	Data     []byte
}

type msiDB struct {
	f        *cfbFile
	CodePage uint32
	LongRefs bool
	Pool     []poolEntry // index = string id; 0 unused
	Tables   []string
	Columns  map[string][]colDef
	Streams  map[string]*cfbEntry // decoded name -> entry (tables have table=true)
	TableStr map[string]*cfbEntry
	ids      map[string]uint32
}

type colDef struct {
	Number int
	Name   string
	Type   uint16
}

func (c colDef) width(longRefs bool) int {
	t := c.Type
	switch {
	case t&^0x1000 == 0x0900:
		return 2
	case t&0x0800 != 0:
		if longRefs {
			return 3
		}
		return 2
	case t&0xFF == 4:
		return 4
	case t&0xFF == 1 || t&0xFF == 2:
		return 2
	}
	return -1
}

func (c colDef) isKey() bool    { return c.Type&0x2000 != 0 }
func (c colDef) isString() bool { return c.Type&^0x1000 != 0x0900 && c.Type&0x0800 != 0 }

func openRawMSI(raw []byte) (*msiDB, error) {
	f, err := readCFB(raw)
	if err != nil {
		return nil, err
	}
	db := &msiDB{f: f, Columns: map[string][]colDef{}, Streams: map[string]*cfbEntry{}, TableStr: map[string]*cfbEntry{}}
	for _, e := range f.Entries {
		if e.Type != 2 || e.Parent != 0 {
			continue
		}
		n, isTable := decodeStreamName(e.Name)
		if isTable {
			db.TableStr[n] = e
		} else {
			db.Streams[n] = e
		}
	}
	sp, sd := db.TableStr["_StringPool"], db.TableStr["_StringData"]
	if sp == nil || sd == nil {
		return nil, fmt.Errorf("missing string pool")
	}
	le := binary.LittleEndian
	hdr := le.Uint32(sp.Data)
	db.CodePage = hdr & 0x7FFFFFFF
	db.LongRefs = hdr&0x80000000 != 0
	db.Pool = []poolEntry{{}}
	pos := 0
	for i := 4; i+4 <= len(sp.Data); {
		l := uint32(le.Uint16(sp.Data[i:]))
		r := uint32(le.Uint16(sp.Data[i+2:]))
		e := poolEntry{Len: l, Ref: r}
		i += 4
		if l == 0 && r != 0 {
			// long entry: 0, refcount, low16, high16
			if i+4 > len(sp.Data) {
				return nil, fmt.Errorf("truncated long pool entry")
			}
			e.Long = true
			e.Len = uint32(le.Uint16(sp.Data[i:])) | uint32(le.Uint16(sp.Data[i+2:]))<<16
			i += 4
		}
		if pos+int(e.Len) > len(sd.Data) {
			return nil, fmt.Errorf("pool overruns _StringData at id %d", len(db.Pool))
		}
		e.Data = sd.Data[pos : pos+int(e.Len)]
		pos += int(e.Len)
		db.Pool = append(db.Pool, e)
	}
	if pos != len(sd.Data) {
		return nil, fmt.Errorf("pool lengths sum to %d, _StringData is %d", pos, len(sd.Data))
	}
	refW := 2
	if db.LongRefs {
		refW = 3
	}
	readRef := func(b []byte) uint32 {
		v := uint32(le.Uint16(b))
		if refW == 3 {
			v |= uint32(b[2]) << 16
		}
		return v
	}
	if t := db.TableStr["_Tables"]; t != nil {
		for i := 0; i+refW <= len(t.Data); i += refW {
			db.Tables = append(db.Tables, db.str(readRef(t.Data[i:])))
		}
	}
	if c := db.TableStr["_Columns"]; c != nil {
		rowSize := refW + 2 + refW + 2
		n := len(c.Data) / rowSize
		d := c.Data
		for r := 0; r < n; r++ {
			tbl := db.str(readRef(d[r*refW:]))
			num := int(le.Uint16(d[n*refW+r*2:])) - 0x8000
			name := db.str(readRef(d[n*(refW+2)+r*refW:]))
			typ := le.Uint16(d[n*(2*refW+2)+r*2:]) - 0x8000
			db.Columns[tbl] = append(db.Columns[tbl], colDef{Number: num, Name: name, Type: typ})
		}
	}
	return db, nil
}

func (db *msiDB) str(id uint32) string {
	if id == 0 || int(id) >= len(db.Pool) {
		return ""
	}
	return string(db.Pool[id].Data)
}

type rawCell struct {
	Raw  uint32
	Text string
	Null bool
}

// rows decodes a table stream into raw cells per row.
func (db *msiDB) rows(table string) ([][]rawCell, []colDef, error) {
	cols := db.Columns[table]
	e := db.TableStr[table]
	if e == nil {
		return nil, cols, nil
	}
	rowSize := 0
	for _, c := range cols {
		rowSize += c.width(db.LongRefs)
	}
	if rowSize == 0 || len(e.Data)%rowSize != 0 {
		return nil, cols, fmt.Errorf("%s: stream %d bytes not a multiple of row size %d", table, len(e.Data), rowSize)
	}
	n := len(e.Data) / rowSize
	out := make([][]rawCell, n)
	le := binary.LittleEndian
	off := 0
	for _, c := range cols {
		w := c.width(db.LongRefs)
		for r := 0; r < n; r++ {
			b := e.Data[off+r*w:]
			var v uint32
			switch w {
			case 2:
				v = uint32(le.Uint16(b))
			case 3:
				v = uint32(le.Uint16(b)) | uint32(b[2])<<16
			case 4:
				v = le.Uint32(b)
			}
			cell := rawCell{Raw: v, Null: v == 0}
			if !cell.Null {
				switch {
				case c.isString():
					cell.Text = db.str(v)
				case c.Type&^0x1000 == 0x0900:
					cell.Text = "<binary>"
				case w == 4:
					cell.Text = fmt.Sprint(int32(v ^ 0x80000000))
				default:
					cell.Text = fmt.Sprint(int(v) - 0x8000)
				}
			}
			out[r] = append(out[r], cell)
		}
		off += n * w
	}
	return out, cols, nil
}

// encodeRows writes rows back in column-major layout (same widths).
func (db *msiDB) encodeRows(table string, rows [][]rawCell) []byte {
	cols := db.Columns[table]
	var out []byte
	for ci, c := range cols {
		w := c.width(db.LongRefs)
		for _, r := range rows {
			v := r[ci].Raw
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, v)
			out = append(out, b[:w]...)
		}
	}
	return out
}

// keysSorted reports whether rows are in ascending stored-key order.
func keysSorted(rows [][]rawCell, cols []colDef) bool {
	return sort.SliceIsSorted(rows, func(i, j int) bool { return keyLess(rows[i], rows[j], cols) })
}

func keyLess(a, b []rawCell, cols []colDef) bool {
	for i, c := range cols {
		if !c.isKey() {
			break
		}
		if a[i].Raw != b[i].Raw {
			return a[i].Raw < b[i].Raw
		}
	}
	return false
}

// refcounts recounts string references across every table stream incl. catalog.
func (db *msiDB) refcounts() map[uint32]uint32 {
	cnt := map[uint32]uint32{}
	for _, t := range db.Tables {
		cnt[db.idOf(t)]++ // _Tables row
	}
	for t, cols := range db.Columns {
		for _, c := range cols {
			cnt[db.idOf(t)]++
			cnt[db.idOf(c.Name)]++
		}
	}
	for _, t := range db.Tables {
		rows, cols, err := db.rows(t)
		if err != nil {
			continue
		}
		for _, r := range rows {
			for i, c := range cols {
				if c.isString() && !r[i].Null {
					cnt[r[i].Raw]++
				}
			}
		}
	}
	delete(cnt, 0)
	return cnt
}

func (db *msiDB) idOf(s string) uint32 {
	if db.ids == nil {
		db.ids = map[string]uint32{}
		for i, e := range db.Pool {
			if i > 0 && e.Len > 0 {
				db.ids[string(e.Data)] = uint32(i)
			}
		}
	}
	return db.ids[s]
}
