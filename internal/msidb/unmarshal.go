package msidb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jkroepke/msi-builder/internal/cfb"
	"github.com/jkroepke/msi-builder/internal/codepage"
	"github.com/jkroepke/msi-builder/internal/summary"
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
	file, err := cfb.Read(data)
	if err != nil {
		return nil, fmt.Errorf("msidb: %w", err)
	}

	return decodeFile(file, true)
}

// Decode reads the database of an opened compound file. It loads the table
// streams, the summary and the binary streams that table cells reference,
// and lists every other stream (such as a cabinet) in Database.Sources by
// name with its size and a nil Open, leaving the bytes in the file for
// cfb.File.Reader. Unmarshal loads those too.
func Decode(file *cfb.File) (*Database, error) {
	return decodeFile(file, false)
}

// streamData returns a stream's bytes, loading them on demand.
func streamData(file *cfb.File, name string) ([]byte, error) {
	data, ok := file.Stream(name)
	if !ok {
		return nil, fmt.Errorf("msidb: cannot read stream %q", name)
	}

	return data, nil
}

func decodeFile(file *cfb.File, loadAll bool) (*Database, error) { //nolint:maintidx,cyclop,gocyclo,gocognit // one pass over the format
	if file.CLSID != cfb.CLSIDMSI {
		return nil, fmt.Errorf("msidb: root CLSID %x is not the .msi CLSID", file.CLSID)
	}

	tableStreams := make(map[string][]byte)
	otherStreams := make(map[string][]byte)
	sizes := make(map[string]uint64)
	stored := make(map[string]string) // decoded name -> name as stored in the file

	var order []string

	for _, ent := range file.Entries {
		if ent.Type != 2 || ent.Parent != 0 {
			continue
		}

		name, isTable := DecodeStreamName(ent.Name)

		switch {
		case isTable:
			data, err := streamData(file, ent.Name)
			if err != nil {
				return nil, err
			}

			tableStreams[name] = data
		case loadAll || name == summary.StreamName:
			data, err := streamData(file, ent.Name)
			if err != nil {
				return nil, err
			}

			otherStreams[name] = data
			order = append(order, name)
		default:
			sizes[name] = ent.Size
			stored[name] = ent.Name
			order = append(order, name)
		}
	}

	// binaryStream hands a table cell its stream, loading it on demand, and
	// takes it out of the extra streams.
	binaryStream := func(name string) ([]byte, bool) {
		if data, ok := otherStreams[name]; ok {
			delete(otherStreams, name)

			return data, true
		}

		encoded, ok := stored[name]
		if !ok {
			return nil, false
		}

		// The stored name, not a re-encoding: the encoding is not canonical.
		data, err := streamData(file, encoded)
		if err != nil {
			return nil, false
		}

		delete(sizes, name)

		return data, true
	}

	poolStream, ok := tableStreams[TableStringPool]
	if !ok {
		return nil, errors.New("msidb: no _StringPool stream")
	}

	dataStream := tableStreams[TableStringData]

	if len(poolStream) < 4 {
		return nil, errors.New("msidb: _StringPool is truncated")
	}

	endian := binary.LittleEndian
	hdr := endian.Uint32(poolStream)
	database := &Database{Codepage: int(hdr & 0x7FFFFFFF)}
	longRefs := hdr&0x80000000 != 0

	if !codepage.Supported(database.Codepage) {
		return nil, fmt.Errorf("msidb: database code page %d is not supported", database.Codepage)
	}

	entries := []poolEntry{{}} // ID 0 is NULL
	pos := 0

	for i := 4; i+4 <= len(poolStream); {
		l := int(endian.Uint16(poolStream[i:]))
		r := int(endian.Uint16(poolStream[i+2:]))
		ent := poolEntry{length: l, refcount: r}
		i += 4

		if l == 0 && r != 0 {
			if i+4 > len(poolStream) {
				return nil, errors.New("msidb: truncated long string pool entry")
			}

			ent.long = true
			ent.length = int(endian.Uint32(poolStream[i:]))
			i += 4
		}

		if pos+ent.length > len(dataStream) {
			return nil, fmt.Errorf("msidb: string pool entry %d overruns _StringData", len(entries))
		}

		if ent.length > 0 {
			s, err := codepage.Decode(dataStream[pos:pos+ent.length], database.Codepage)
			if err != nil {
				return nil, fmt.Errorf("msidb: string %d: %w", len(entries), err)
			}

			ent.text = s
		}

		pos += ent.length
		entries = append(entries, ent)
	}

	if pos != len(dataStream) {
		return nil, fmt.Errorf("msidb: string lengths sum to %d but _StringData has %d bytes", pos, len(dataStream))
	}

	if (len(entries)-1 >= longRefThresh) != longRefs {
		database.Problems = append(database.Problems, fmt.Sprintf("long-reference flag is %v with highest string ID %d", longRefs, len(entries)-1))
	}

	str := func(ref uint32) (string, error) {
		if ref == 0 {
			return "", nil
		}

		if int(ref) >= len(entries) || entries[ref].length == 0 {
			return "", fmt.Errorf("msidb: string ID %d is not in the pool", ref)
		}

		return entries[ref].text, nil
	}

	refWidth := 2
	if longRefs {
		refWidth = 3
	}

	readRef := func(b []byte) uint32 {
		v := uint32(endian.Uint16(b))
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

	count := len(colsData) / rowSize

	type colRow struct {
		table  string
		column Column
		number int
	}

	colRows := make([]colRow, count)

	for rowIdx := range count {
		tid := readRef(colsData[rowIdx*refWidth:])
		num := int(endian.Uint16(colsData[count*refWidth+rowIdx*2:])) - 0x8000
		nid := readRef(colsData[count*(refWidth+2)+rowIdx*refWidth:])
		typ := endian.Uint16(colsData[count*(2*refWidth+2)+rowIdx*2:]) - 0x8000
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

		colRows[rowIdx] = colRow{table: tname, column: Column{Name: cname, Type: t}, number: num}
	}

	slices.SortStableFunc(colRows, func(a, b colRow) int {
		if a.table != b.table {
			return strings.Compare(a.table, b.table)
		}

		return a.number - b.number
	})

	for _, col := range colRows {
		tbl, ok := tables[col.table]
		if !ok {
			return nil, fmt.Errorf("msidb: _Columns describes %s, which is not in _Tables", col.table)
		}

		if col.number != len(tbl.Columns)+1 {
			return nil, fmt.Errorf("msidb: table %s: column numbers are not contiguous at %d", col.table, col.number)
		}

		tbl.Columns = append(tbl.Columns, col.column)
	}

	for _, name := range tableOrder {
		tbl := tables[name]
		if len(tbl.Columns) == 0 {
			return nil, fmt.Errorf("msidb: table %s has no columns", name)
		}

		if err := decodeRows(tbl, tableStreams[name], longRefs, readRef, str, refs, binaryStream, database); err != nil {
			return nil, err
		}

		database.Tables = append(database.Tables, tbl)
	}

	for strID, ent := range entries {
		if strID == 0 || ent.length == 0 {
			continue
		}

		if refs[uint32(strID)] != ent.refcount {
			database.Problems = append(database.Problems, fmt.Sprintf("string %q has refcount %d but %d references", ent.text, ent.refcount, refs[uint32(strID)]))
		}
	}

	for _, name := range order {
		if name == summary.StreamName {
			props, err := summary.Unmarshal(otherStreams[name])
			if err != nil {
				return nil, fmt.Errorf("msidb: %w", err)
			}

			database.Summary = props

			continue
		}

		if _, used := otherStreams[name]; used {
			database.Streams = append(database.Streams, cfb.Stream{Name: name, Data: otherStreams[name]})
		}

		if size, left := sizes[name]; left {
			database.Sources = append(database.Sources, cfb.Source{Name: name, Size: int64(size)}) //nolint:gosec // checked against the file size
		}
	}

	return database, nil
}

func decodeRows( //nolint:cyclop,revive,gocognit // one pass over the format
	tbl *Table, data []byte, longRefs bool, readRef func([]byte) uint32, str func(uint32) (string, error),
	refs map[uint32]int, binaryStream func(string) ([]byte, bool), database *Database,
) error {
	rowSize := 0
	for _, c := range tbl.Columns {
		rowSize += c.Type.storageWidth(longRefs)
	}

	if len(data) == 0 {
		return nil
	}

	if len(data)%rowSize != 0 {
		return fmt.Errorf("msidb: table %s: stream of %d bytes is not a multiple of the row size %d", tbl.Name, len(data), rowSize)
	}

	order := binary.LittleEndian
	count := len(data) / rowSize
	keys := tbl.Keys()
	stored := make([][]uint32, count)
	rows := make([][]any, count)

	for r := range rows {
		rows[r] = make([]any, len(tbl.Columns))
		stored[r] = make([]uint32, len(tbl.Columns))
	}

	off := 0

	for colIdx, col := range tbl.Columns {
		width := col.Type.storageWidth(longRefs)

		for rowIdx := range count {
			cell := data[off+rowIdx*width:]

			var value uint32

			switch width {
			case 2:
				value = uint32(order.Uint16(cell))
			case 3:
				value = readRef(cell)
			case 4:
				value = order.Uint32(cell)
			}

			stored[rowIdx][colIdx] = value

			if value == 0 {
				if !col.Type.Nullable {
					database.Problems = append(database.Problems, fmt.Sprintf("table %s row %d: NULL in non-nullable column %s", tbl.Name, rowIdx, col.Name))
				}

				continue
			}

			switch col.Type.Kind { //nolint:exhaustive // the default covers the remaining kinds
			case KindString:
				refs[value]++

				s, err := str(value)
				if err != nil {
					return fmt.Errorf("msidb: table %s column %s: %w", tbl.Name, col.Name, err)
				}

				rows[rowIdx][colIdx] = s
			case KindInt16:
				rows[rowIdx][colIdx] = int(value) - 0x8000
			case KindInt32:
				rows[rowIdx][colIdx] = int(int32(value ^ 0x80000000)) //nolint:gosec // the stored form is the biased two's complement
			}
		}

		off += count * width
	}

	// Binary cells and the stored-key order.
	for colIdx, col := range tbl.Columns {
		if col.Type.Kind != KindBinary {
			continue
		}

		for rowIdx := range rows {
			if stored[rowIdx][colIdx] == 0 {
				continue
			}

			name := BinaryStreamName(tbl.Name, rows[rowIdx][:keys])

			data, ok := binaryStream(name)
			if !ok {
				database.Problems = append(database.Problems, fmt.Sprintf("table %s row %d: binary stream %q is missing", tbl.Name, rowIdx, name))

				continue
			}

			rows[rowIdx][colIdx] = append([]byte(nil), data...)
		}
	}

	for rowIdx := 1; rowIdx < count; rowIdx++ {
		cmp := 0

		for k := 0; k < keys && cmp == 0; k++ {
			switch {
			case stored[rowIdx-1][k] < stored[rowIdx][k]:
				cmp = -1
			case stored[rowIdx-1][k] > stored[rowIdx][k]:
				cmp = 1
			}
		}

		if cmp > 0 {
			database.Problems = append(database.Problems, fmt.Sprintf("table %s: rows %d and %d are not in key order", tbl.Name, rowIdx-1, rowIdx))
		} else if cmp == 0 {
			database.Problems = append(database.Problems, fmt.Sprintf("table %s: rows %d and %d share a primary key", tbl.Name, rowIdx-1, rowIdx))
		}
	}

	tbl.Rows = rows

	return nil
}
