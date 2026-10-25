package msidb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/jkroepke/go-msi/cfb"
	"github.com/jkroepke/go-msi/codepage"
	"github.com/jkroepke/go-msi/summary"
)

// System table and stream names.
const (
	TableTables     = "_Tables"
	TableColumns    = "_Columns"
	TableStringPool = "_StringPool"
	TableStringData = "_StringData"
	TableValidation = "_Validation"
)

// Options control Marshal.
type Options struct {
	// CFBVersion is 3 or 4; 0 means 4.
	CFBVersion int
}

// pool assigns string IDs and counts references.
type pool struct {
	cp       int
	bytes    map[string][]byte
	ids      map[string]uint32
	refs     map[string]int
	ordered  []string
	longRefs bool
}

func newPool(cp int) *pool {
	return &pool{cp: cp, bytes: map[string][]byte{}, ids: map[string]uint32{}, refs: map[string]int{}}
}

// use records one reference to s.
func (p *pool) use(s string) error {
	if _, ok := p.bytes[s]; !ok {
		b, err := codepage.Encode(s, p.cp)
		if err != nil {
			return err
		}

		if len(b) == 0 {
			return errors.New("msidb: empty string in the pool")
		}

		p.bytes[s] = b
	}

	p.refs[s]++

	return nil
}

// assign numbers the strings by their encoded bytes, from 1.
func (p *pool) assign() error {
	p.ordered = p.ordered[:0]
	for s := range p.bytes {
		p.ordered = append(p.ordered, s)
	}

	sort.Slice(p.ordered, func(i, j int) bool { return bytes.Compare(p.bytes[p.ordered[i]], p.bytes[p.ordered[j]]) < 0 })

	for i, s := range p.ordered {
		p.ids[s] = uint32(i + 1)

		if p.refs[s] > MaxRefcount {
			return fmt.Errorf("msidb: string %.40q is referenced %d times; msi.dll keeps only 15 bits of a refcount (DB-05), the limit is %d", s, p.refs[s], MaxRefcount)
		}
	}

	p.longRefs = len(p.ordered) >= longRefThresh

	return nil
}

func (p *pool) id(s string) uint32 { return p.ids[s] }

func (p *pool) marshal() (poolStream, dataStream []byte) {
	le := binary.LittleEndian

	hdr := uint32(p.cp)
	if p.longRefs {
		hdr |= 0x80000000
	}

	poolStream = le.AppendUint32(poolStream, hdr)

	for _, s := range p.ordered {
		b := p.bytes[s]
		ref := uint16(p.refs[s])

		if len(b) > 0xFFFF {
			poolStream = le.AppendUint16(poolStream, 0)
			poolStream = le.AppendUint16(poolStream, ref)
			poolStream = le.AppendUint32(poolStream, uint32(len(b)))
		} else {
			poolStream = le.AppendUint16(poolStream, uint16(len(b)))
			poolStream = le.AppendUint16(poolStream, ref)
		}

		dataStream = append(dataStream, b...)
	}

	return poolStream, dataStream
}

// stored returns the raw stored value of a normalized cell.
func (p *pool) stored(c Column, v any) uint32 {
	if v == nil {
		return 0
	}

	switch c.Type.Kind {
	case KindString:
		return p.id(v.(string))
	case KindInt16:
		return uint32(int32(v.(int))+0x8000) & 0xFFFF
	case KindInt32:
		return uint32(int32(v.(int))) ^ 0x80000000
	default:
		return 1
	}
}

// Marshal serializes the database into a compound file.
func (db *Database) Marshal(o Options) ([]byte, error) {
	if !codepage.Supported(db.Codepage) {
		return nil, fmt.Errorf("msidb: database code page %d is not supported", db.Codepage)
	}

	if db.Codepage == 1200 || db.Codepage == 1201 {
		return nil, errors.New("msidb: UTF-16 database code pages are not valid")
	}

	seen := map[string]bool{}

	for _, t := range db.Tables {
		if err := t.Validate(); err != nil {
			return nil, err
		}

		if seen[t.Name] {
			return nil, fmt.Errorf("msidb: table %s defined twice", t.Name)
		}

		seen[t.Name] = true

		switch t.Name {
		case TableTables, TableColumns, TableStringPool, TableStringData, "_Streams", "_Storages":
			return nil, fmt.Errorf("msidb: table %s is a system table and cannot be defined", t.Name)
		}
	}

	tables := append([]*Table(nil), db.Tables...)
	sort.Slice(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })

	p := newPool(db.Codepage)

	for _, t := range tables {
		// _Tables row and the Table column of every _Columns row.
		if err := p.use(t.Name); err != nil {
			return nil, fmt.Errorf("msidb: table name %q: %w", t.Name, err)
		}

		for _, c := range t.Columns {
			if err := p.use(t.Name); err != nil {
				return nil, err
			}

			if err := p.use(c.Name); err != nil {
				return nil, fmt.Errorf("msidb: column name %q: %w", c.Name, err)
			}
		}

		for _, row := range t.Rows {
			for i, c := range t.Columns {
				if s, ok := row[i].(string); ok && c.Type.Kind == KindString {
					if err := p.use(s); err != nil {
						return nil, fmt.Errorf("msidb: table %s column %s: %w", t.Name, c.Name, err)
					}
				}
			}
		}
	}

	if err := p.assign(); err != nil {
		return nil, err
	}

	var streams []cfb.Stream

	addStream := func(decoded string, table bool, data []byte) error {
		name, err := EncodeStreamName(decoded, table)
		if err != nil {
			return err
		}

		streams = append(streams, cfb.Stream{Name: name, Data: data})

		return nil
	}

	poolStream, dataStream := p.marshal()

	if err := addStream(TableStringPool, true, poolStream); err != nil {
		return nil, err
	}

	if err := addStream(TableStringData, true, dataStream); err != nil {
		return nil, err
	}

	// _Tables: one string column, rows sorted by ID.
	tablesTable := &Table{Name: TableTables, Columns: []Column{Col("Name", "s64", true)}}
	for _, t := range tables {
		tablesTable.Add(t.Name)
	}

	// _Columns: Table, Number, Name, Type.
	columnsTable := &Table{Name: TableColumns, Columns: []Column{
		Col("Table", "s64", true), Col("Number", "i2", true), Col("Name", "s64", false), Col("Type", "i2", false),
	}}

	for _, t := range tables {
		for i, c := range t.Columns {
			columnsTable.Add(t.Name, i+1, c.Name, int(c.Type.Word()))
		}
	}

	for _, t := range []*Table{tablesTable, columnsTable} {
		if err := t.Validate(); err != nil {
			return nil, err
		}

		data, err := p.encodeTable(t)
		if err != nil {
			return nil, err
		}

		if err := addStream(t.Name, true, data); err != nil {
			return nil, err
		}
	}

	streamNames := map[string]bool{}

	for _, t := range tables {
		if len(t.Rows) == 0 {
			continue
		}

		data, err := p.encodeTable(t)
		if err != nil {
			return nil, err
		}

		if err := addStream(t.Name, true, data); err != nil {
			return nil, err
		}

		// Binary cells.
		keys := t.Keys()

		for bi, c := range t.Columns {
			if c.Type.Kind != KindBinary {
				continue
			}

			for _, row := range t.Rows {
				if row[bi] == nil {
					continue
				}

				name := BinaryStreamName(t.Name, row[:keys])
				if streamNames[name] {
					return nil, fmt.Errorf("msidb: binary stream %q written twice", name)
				}

				streamNames[name] = true

				if err := addStream(name, false, row[bi].([]byte)); err != nil {
					return nil, fmt.Errorf("msidb: table %s: %w", t.Name, err)
				}
			}
		}
	}

	for _, s := range db.Streams {
		if streamNames[s.Name] {
			return nil, fmt.Errorf("msidb: stream %q written twice", s.Name)
		}

		streamNames[s.Name] = true

		if err := addStream(s.Name, false, s.Data); err != nil {
			return nil, err
		}
	}

	if db.Summary != nil {
		cp := db.Codepage
		if !codepage.IsANSI(cp) {
			cp = codepage.Windows1252
		}

		if v, ok := summary.Get(db.Summary, summary.PIDCodepage).(int); ok {
			cp = v
		}

		data, err := summary.Marshal(db.Summary, cp)
		if err != nil {
			return nil, err
		}

		streams = append(streams, cfb.Stream{Name: summary.StreamName, Data: data})
	}

	return cfb.Write(streams, cfb.Options{Version: o.CFBVersion, CLSID: cfb.CLSIDMSI})
}

// encodeTable sorts the rows by their stored keys and writes the column-major
// stream.
func (p *pool) encodeTable(t *Table) ([]byte, error) {
	keys := t.Keys()

	type sortedRow struct {
		stored []uint32
	}

	rows := make([]sortedRow, len(t.Rows))

	for r, row := range t.Rows {
		rows[r].stored = make([]uint32, len(t.Columns))
		for i, c := range t.Columns {
			rows[r].stored[i] = p.stored(c, row[i])
		}
	}

	less := func(a, b []uint32) int {
		for i := 0; i < keys; i++ {
			if a[i] != b[i] {
				if a[i] < b[i] {
					return -1
				}

				return 1
			}
		}

		return 0
	}

	sort.SliceStable(rows, func(i, j int) bool { return less(rows[i].stored, rows[j].stored) < 0 })

	for i := 1; i < len(rows); i++ {
		if less(rows[i-1].stored, rows[i].stored) == 0 {
			return nil, fmt.Errorf("msidb: table %s has two rows with the same primary key %v", t.Name, describeKeys(t, rows[i].stored[:keys], p))
		}
	}

	var out []byte

	for ci, c := range t.Columns {
		w := c.Type.storageWidth(p.longRefs)

		for _, r := range rows {
			v := r.stored[ci]

			switch w {
			case 2:
				out = append(out, byte(v), byte(v>>8))
			case 3:
				out = append(out, byte(v), byte(v>>8), byte(v>>16))
			case 4:
				out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
			}
		}
	}

	return out, nil
}

func describeKeys(t *Table, stored []uint32, p *pool) []any {
	var out []any

	for i, v := range stored {
		c := t.Columns[i]

		switch {
		case v == 0:
			out = append(out, nil)
		case c.Type.Kind == KindString:
			out = append(out, p.ordered[v-1])
		case c.Type.Kind == KindInt16:
			out = append(out, int(v)-0x8000)
		default:
			out = append(out, int(int32(v^0x80000000)))
		}
	}

	return out
}
