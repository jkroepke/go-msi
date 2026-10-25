package msidb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jkroepke/go-msi/cfb"
	"github.com/jkroepke/go-msi/codepage"
	"github.com/jkroepke/go-msi/summary"
)

type poolEntry struct {
	text     string
	length   int
	refcount int
	long     bool
}

// Unmarshal reads a package. It accepts what other writers emit (free pool
// slots, unsorted pool, empty table streams) and records soft problems such
// as refcount mismatches and unsorted rows in Database.Problems.
func Unmarshal(data []byte) (*Database, error) {
	f, err := cfb.Read(data)
	if err != nil {
		return nil, err
	}

	if f.CLSID != cfb.CLSIDMSI {
		return nil, fmt.Errorf("msidb: root CLSID %x is not the .msi CLSID", f.CLSID)
	}

	tableStreams := make(map[string][]byte)
	otherStreams := make(map[string][]byte)

	var order []string

	for _, s := range f.Streams() {
		name, isTable := DecodeStreamName(s.Name)
		if isTable {
			tableStreams[name] = s.Data
		} else {
			otherStreams[name] = s.Data
			order = append(order, name)
		}
	}

	poolStream, ok := tableStreams[TableStringPool]
	if !ok {
		return nil, errors.New("msidb: no _StringPool stream")
	}

	dataStream := tableStreams[TableStringData]

	if len(poolStream) < 4 {
		return nil, errors.New("msidb: _StringPool is truncated")
	}

	le := binary.LittleEndian
	hdr := le.Uint32(poolStream)
	db := &Database{Codepage: int(hdr & 0x7FFFFFFF)}
	longRefs := hdr&0x80000000 != 0

	if !codepage.Supported(db.Codepage) {
		return nil, fmt.Errorf("msidb: database code page %d is not supported", db.Codepage)
	}

	entries := []poolEntry{{}} // ID 0 is NULL
	pos := 0

	for i := 4; i+4 <= len(poolStream); {
		l := int(le.Uint16(poolStream[i:]))
		r := int(le.Uint16(poolStream[i+2:]))
		e := poolEntry{length: l, refcount: r}
		i += 4

		if l == 0 && r != 0 {
			if i+4 > len(poolStream) {
				return nil, errors.New("msidb: truncated long string pool entry")
			}

			e.long = true
			e.length = int(le.Uint32(poolStream[i:]))
			i += 4
		}

		if pos+e.length > len(dataStream) {
			return nil, fmt.Errorf("msidb: string pool entry %d overruns _StringData", len(entries))
		}

		if e.length > 0 {
			s, err := codepage.Decode(dataStream[pos:pos+e.length], db.Codepage)
			if err != nil {
				return nil, fmt.Errorf("msidb: string %d: %w", len(entries), err)
			}

			e.text = s
		}

		pos += e.length
		entries = append(entries, e)
	}

	if pos != len(dataStream) {
		return nil, fmt.Errorf("msidb: string lengths sum to %d but _StringData has %d bytes", pos, len(dataStream))
	}

	if (len(entries)-1 >= longRefThresh) != longRefs {
		db.Problems = append(db.Problems, fmt.Sprintf("long-reference flag is %v with highest string ID %d", longRefs, len(entries)-1))
	}

	str := func(id uint32) (string, error) {
		if id == 0 {
			return "", nil
		}

		if int(id) >= len(entries) || entries[id].length == 0 {
			return "", fmt.Errorf("msidb: string ID %d is not in the pool", id)
		}

		return entries[id].text, nil
	}

	refWidth := 2
	if longRefs {
		refWidth = 3
	}

	readRef := func(b []byte) uint32 {
		v := uint32(le.Uint16(b))
		if refWidth == 3 {
			v |= uint32(b[2]) << 16
		}

		return v
	}

	refs := make(map[uint32]int)

	// _Tables.
	tablesData := tableStreams[TableTables]
	if len(tablesData)%refWidth != 0 {
		return nil, errors.New("msidb: _Tables stream has a bad length")
	}

	tables := make(map[string]*Table)

	var tableOrder []string

	for i := 0; i+refWidth <= len(tablesData); i += refWidth {
		id := readRef(tablesData[i:])
		refs[id]++

		name, err := str(id)
		if err != nil {
			return nil, fmt.Errorf("msidb: _Tables: %w", err)
		}

		if _, dup := tables[name]; dup || name == "" {
			return nil, fmt.Errorf("msidb: _Tables lists %q twice or lists NULL", name)
		}

		tables[name] = &Table{Name: name}
		tableOrder = append(tableOrder, name)
	}

	// _Columns: column-major with 4 columns: Table (ref), Number (2), Name (ref), Type (2).
	colsData := tableStreams[TableColumns]
	rowSize := 2*refWidth + 4

	if len(colsData)%rowSize != 0 {
		return nil, errors.New("msidb: _Columns stream has a bad length")
	}

	n := len(colsData) / rowSize

	type colRow struct {
		table  string
		column Column
		number int
	}

	colRows := make([]colRow, n)

	for r := range n {
		tid := readRef(colsData[r*refWidth:])
		num := int(le.Uint16(colsData[n*refWidth+r*2:])) - 0x8000
		nid := readRef(colsData[n*(refWidth+2)+r*refWidth:])
		typ := le.Uint16(colsData[n*(2*refWidth+2)+r*2:]) - 0x8000
		refs[tid]++
		refs[nid]++

		tname, err := str(tid)
		if err != nil {
			return nil, fmt.Errorf("msidb: _Columns: %w", err)
		}

		cname, err := str(nid)
		if err != nil {
			return nil, fmt.Errorf("msidb: _Columns: %w", err)
		}

		t, err := TypeFromWord(typ)
		if err != nil {
			return nil, fmt.Errorf("msidb: _Columns: table %s column %s: %w", tname, cname, err)
		}

		colRows[r] = colRow{table: tname, column: Column{Name: cname, Type: t}, number: num}
	}

	slices.SortStableFunc(colRows, func(a, b colRow) int {
		if a.table != b.table {
			return strings.Compare(a.table, b.table)
		}

		return a.number - b.number
	})

	for _, cr := range colRows {
		t, ok := tables[cr.table]
		if !ok {
			return nil, fmt.Errorf("msidb: _Columns describes %s, which is not in _Tables", cr.table)
		}

		if cr.number != len(t.Columns)+1 {
			return nil, fmt.Errorf("msidb: table %s: column numbers are not contiguous at %d", cr.table, cr.number)
		}

		t.Columns = append(t.Columns, cr.column)
	}

	for _, name := range tableOrder {
		t := tables[name]
		if len(t.Columns) == 0 {
			return nil, fmt.Errorf("msidb: table %s has no columns", name)
		}

		if err := decodeRows(t, tableStreams[name], longRefs, readRef, str, refs, otherStreams, db); err != nil {
			return nil, err
		}

		db.Tables = append(db.Tables, t)
	}

	for id, e := range entries {
		if id == 0 || e.length == 0 {
			continue
		}

		if refs[uint32(id)] != e.refcount {
			db.Problems = append(db.Problems, fmt.Sprintf("string %q has refcount %d but %d references", e.text, e.refcount, refs[uint32(id)]))
		}
	}

	for _, name := range order {
		if name == summary.StreamName {
			props, err := summary.Unmarshal(otherStreams[name])
			if err != nil {
				return nil, err
			}

			db.Summary = props

			continue
		}

		if _, used := otherStreams[name]; used {
			db.Streams = append(db.Streams, cfb.Stream{Name: name, Data: otherStreams[name]})
		}
	}

	return db, nil
}

func decodeRows(t *Table, data []byte, longRefs bool, readRef func([]byte) uint32, str func(uint32) (string, error),
	refs map[uint32]int, otherStreams map[string][]byte, db *Database,
) error {
	rowSize := 0
	for _, c := range t.Columns {
		rowSize += c.Type.storageWidth(longRefs)
	}

	if len(data) == 0 {
		return nil
	}

	if len(data)%rowSize != 0 {
		return fmt.Errorf("msidb: table %s: stream of %d bytes is not a multiple of the row size %d", t.Name, len(data), rowSize)
	}

	le := binary.LittleEndian
	n := len(data) / rowSize
	keys := t.Keys()
	stored := make([][]uint32, n)
	rows := make([][]any, n)

	for r := range rows {
		rows[r] = make([]any, len(t.Columns))
		stored[r] = make([]uint32, len(t.Columns))
	}

	off := 0

	for ci, c := range t.Columns {
		w := c.Type.storageWidth(longRefs)

		for r := range n {
			b := data[off+r*w:]

			var v uint32

			switch w {
			case 2:
				v = uint32(le.Uint16(b))
			case 3:
				v = readRef(b)
			case 4:
				v = le.Uint32(b)
			}

			stored[r][ci] = v

			if v == 0 {
				if !c.Type.Nullable {
					db.Problems = append(db.Problems, fmt.Sprintf("table %s row %d: NULL in non-nullable column %s", t.Name, r, c.Name))
				}

				continue
			}

			switch c.Type.Kind {
			case KindString:
				refs[v]++

				s, err := str(v)
				if err != nil {
					return fmt.Errorf("msidb: table %s column %s: %w", t.Name, c.Name, err)
				}

				rows[r][ci] = s
			case KindInt16:
				rows[r][ci] = int(v) - 0x8000
			case KindInt32:
				rows[r][ci] = int(int32(v ^ 0x80000000)) //nolint:gosec // the stored form is the biased two's complement
			}
		}

		off += n * w
	}

	// Binary cells and the stored-key order.
	for ci, c := range t.Columns {
		if c.Type.Kind != KindBinary {
			continue
		}

		for r := range rows {
			if stored[r][ci] == 0 {
				continue
			}

			name := BinaryStreamName(t.Name, rows[r][:keys])

			b, ok := otherStreams[name]
			if !ok {
				db.Problems = append(db.Problems, fmt.Sprintf("table %s row %d: binary stream %q is missing", t.Name, r, name))

				continue
			}

			rows[r][ci] = append([]byte(nil), b...)

			delete(otherStreams, name)
		}
	}

	for r := 1; r < n; r++ {
		cmp := 0

		for k := 0; k < keys && cmp == 0; k++ {
			switch {
			case stored[r-1][k] < stored[r][k]:
				cmp = -1
			case stored[r-1][k] > stored[r][k]:
				cmp = 1
			}
		}

		if cmp > 0 {
			db.Problems = append(db.Problems, fmt.Sprintf("table %s: rows %d and %d are not in key order", t.Name, r-1, r))
		} else if cmp == 0 {
			db.Problems = append(db.Problems, fmt.Sprintf("table %s: rows %d and %d share a primary key", t.Name, r-1, r))
		}
	}

	t.Rows = rows

	return nil
}
