// Package summary writes and reads the \u0005SummaryInformation stream of a
// Windows Installer package: a one-section OLE property set
// (docs/spec/MSISPEC.md section 4).
package summary

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jkroepke/go-msi/codepage"
)

// StreamName is the literal name of the summary stream.
const StreamName = "\u0005SummaryInformation"

// Property IDs.
const (
	PIDCodepage     = 1
	PIDTitle        = 2
	PIDSubject      = 3
	PIDAuthor       = 4
	PIDKeywords     = 5
	PIDComments     = 6
	PIDTemplate     = 7
	PIDLastAuthor   = 8
	PIDRevision     = 9
	PIDLastPrinted  = 11
	PIDCreated      = 12
	PIDLastSaved    = 13
	PIDPageCount    = 14
	PIDWordCount    = 15
	PIDCharCount    = 16
	PIDAppName      = 18
	PIDSecurity     = 19
	maxPID          = 19
	maxStreamSize   = 262144
	systemIdentifer = 0x0002000A // what the native writer stores (DB-14)
)

// Variant types.
const (
	vtI2       = 0x0002
	vtI4       = 0x0003
	vtLPSTR    = 0x001E
	vtFILETIME = 0x0040
)

var fmtid = [16]byte{0xE0, 0x85, 0x9F, 0xF2, 0xF9, 0x4F, 0x68, 0x10, 0xAB, 0x91, 0x08, 0x00, 0x2B, 0x27, 0xB3, 0xD9}

// expectedType is the variant type each PID takes.
var expectedType = map[int]uint16{
	PIDCodepage: vtI2, PIDTitle: vtLPSTR, PIDSubject: vtLPSTR, PIDAuthor: vtLPSTR,
	PIDKeywords: vtLPSTR, PIDComments: vtLPSTR, PIDTemplate: vtLPSTR, PIDLastAuthor: vtLPSTR,
	PIDRevision: vtLPSTR, PIDLastPrinted: vtFILETIME, PIDCreated: vtFILETIME, PIDLastSaved: vtFILETIME,
	PIDPageCount: vtI4, PIDWordCount: vtI4, PIDCharCount: vtI4, PIDAppName: vtLPSTR, PIDSecurity: vtI4,
}

// Property is one summary property. Value is int (I2 for PID 1, I4
// otherwise), string or time.Time (UTC).
type Property struct {
	ID    int
	Value any
}

// Word Count flags (PID 15).
const (
	WordShortNames  = 0x1
	WordCompressed  = 0x2
	WordAdminImage  = 0x4
	WordNoElevation = 0x8
)

const epochDelta = 116444736000000000 // 100 ns units between 1601 and 1970

// FileTime converts t to a Windows FILETIME value.
func FileTime(t time.Time) uint64 {
	return uint64(t.UTC().UnixNano()/100) + epochDelta
}

// FromFileTime converts a FILETIME to a UTC time.
func FromFileTime(ft uint64) time.Time {
	return time.Unix(0, int64(ft-epochDelta)*100).UTC()
}

// Marshal serializes the properties. Strings are written in cp, which must
// be an ANSI code page and should equal the PID 1 value.
func Marshal(props []Property, cp int) ([]byte, error) {
	if !codepage.IsANSI(cp) {
		return nil, fmt.Errorf("summary: code page %d is not an ANSI code page", cp)
	}

	sorted := append([]Property(nil), props...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	hasCodepage := false

	for i, p := range sorted {
		if p.ID <= 0 || p.ID > maxPID || p.ID == 10 || p.ID == 17 {
			return nil, fmt.Errorf("summary: PID %d is not valid", p.ID)
		}

		if i > 0 && sorted[i-1].ID == p.ID {
			return nil, fmt.Errorf("summary: PID %d given twice", p.ID)
		}

		if p.ID == PIDCodepage {
			hasCodepage = true
		}
	}

	if !hasCodepage {
		return nil, errors.New("summary: PID 1 (code page) is required")
	}

	le := binary.LittleEndian

	var values [][]byte

	for _, p := range sorted {
		vt := expectedType[p.ID]
		v := make([]byte, 4)
		le.PutUint16(v, vt)

		switch vt {
		case vtI2:
			n, ok := p.Value.(int)
			if !ok || n < 0 || n > 0xFFFF {
				return nil, fmt.Errorf("summary: PID %d wants a 16-bit integer, got %v", p.ID, p.Value)
			}

			v = append(v, byte(n), byte(n>>8), 0, 0)
		case vtI4:
			n, ok := p.Value.(int)
			if !ok || n < -2147483648 || n > 2147483647 {
				return nil, fmt.Errorf("summary: PID %d wants a 32-bit integer, got %v", p.ID, p.Value)
			}

			v = le.AppendUint32(v, uint32(int32(n)))
		case vtLPSTR:
			s, ok := p.Value.(string)
			if !ok {
				return nil, fmt.Errorf("summary: PID %d wants a string, got %v", p.ID, p.Value)
			}

			if s == "" {
				return nil, fmt.Errorf("summary: PID %d is empty; omit it instead", p.ID)
			}

			b, err := codepage.Encode(s, cp)
			if err != nil {
				return nil, fmt.Errorf("summary: PID %d: %w", p.ID, err)
			}

			v = le.AppendUint32(v, uint32(len(b)+1))
			v = append(v, b...)
			v = append(v, 0)

			for len(v)%4 != 0 {
				v = append(v, 0)
			}
		case vtFILETIME:
			t, ok := p.Value.(time.Time)
			if !ok {
				return nil, fmt.Errorf("summary: PID %d wants a time, got %v", p.ID, p.Value)
			}

			v = le.AppendUint64(v, FileTime(t))
		}

		values = append(values, v)
	}

	n := len(sorted)
	sectionSize := 8 + 8*n

	for _, v := range values {
		sectionSize += len(v)
	}

	out := make([]byte, 0x30, 0x30+sectionSize)
	le.PutUint16(out[0x00:], 0xFFFE)
	le.PutUint32(out[0x04:], systemIdentifer)
	le.PutUint32(out[0x18:], 1)
	copy(out[0x1C:], fmtid[:])
	le.PutUint32(out[0x2C:], 0x30)

	out = le.AppendUint32(out, uint32(sectionSize))
	out = le.AppendUint32(out, uint32(n))

	offset := 8 + 8*n

	for i, p := range sorted {
		out = le.AppendUint32(out, uint32(p.ID))
		out = le.AppendUint32(out, uint32(offset))
		offset += len(values[i])
	}

	for _, v := range values {
		out = append(out, v...)
	}

	if len(out) > maxStreamSize {
		return nil, errors.New("summary: stream exceeds 262144 bytes")
	}

	return out, nil
}

// Unmarshal parses a summary stream. Strings are decoded with the PID 1 code
// page.
func Unmarshal(data []byte) ([]Property, error) {
	le := binary.LittleEndian

	if len(data) < 0x30+8 || le.Uint16(data) != 0xFFFE {
		return nil, errors.New("summary: not a property set")
	}

	if le.Uint32(data[0x18:]) < 1 || [16]byte(data[0x1C:0x2C]) != fmtid {
		return nil, errors.New("summary: not a SummaryInformation property set")
	}

	sec := int(le.Uint32(data[0x2C:]))
	if sec+8 > len(data) {
		return nil, errors.New("summary: section offset out of range")
	}

	size := int(le.Uint32(data[sec:]))
	count := int(le.Uint32(data[sec+4:]))

	if size > len(data)-sec || count > (size-8)/8 {
		return nil, errors.New("summary: section size or count out of range")
	}

	section := data[sec : sec+size]

	type ent struct{ pid, off int }

	ents := make([]ent, count)
	for i := range ents {
		ents[i] = ent{int(le.Uint32(section[8+8*i:])), int(le.Uint32(section[12+8*i:]))}
	}

	cp := codepage.Windows1252

	for _, e := range ents {
		if e.pid == PIDCodepage && e.off+8 <= len(section) {
			cp = int(le.Uint16(section[e.off+4:]))
		}
	}

	var props []Property

	for _, e := range ents {
		if e.off+4 > len(section) {
			return nil, fmt.Errorf("summary: PID %d value out of range", e.pid)
		}

		vt := le.Uint16(section[e.off:])
		v := section[e.off+4:]

		switch vt {
		case vtI2:
			if len(v) < 2 {
				return nil, fmt.Errorf("summary: PID %d truncated", e.pid)
			}

			props = append(props, Property{e.pid, int(le.Uint16(v))})
		case vtI4:
			if len(v) < 4 {
				return nil, fmt.Errorf("summary: PID %d truncated", e.pid)
			}

			props = append(props, Property{e.pid, int(int32(le.Uint32(v)))})
		case vtLPSTR:
			if len(v) < 4 {
				return nil, fmt.Errorf("summary: PID %d truncated", e.pid)
			}

			n := int(le.Uint32(v))
			if n < 1 || n > len(v)-4 {
				return nil, fmt.Errorf("summary: PID %d string length %d out of range", e.pid, n)
			}

			s, err := codepage.Decode(v[4:4+n-1], cp)
			if err != nil {
				return nil, fmt.Errorf("summary: PID %d: %w", e.pid, err)
			}

			props = append(props, Property{e.pid, s})
		case vtFILETIME:
			if len(v) < 8 {
				return nil, fmt.Errorf("summary: PID %d truncated", e.pid)
			}

			props = append(props, Property{e.pid, FromFileTime(le.Uint64(v))})
		case 0, 1: // VT_EMPTY, VT_NULL: absent
		default:
			return nil, fmt.Errorf("summary: PID %d has variant type %#x", e.pid, vt)
		}
	}

	return props, nil
}

// Get returns the value of a PID, or nil.
func Get(props []Property, pid int) any {
	for _, p := range props {
		if p.ID == pid {
			return p.Value
		}
	}

	return nil
}
