package main

// Cabinet writer and reader written from MSISPEC.md section 5.

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
)

type cabFile struct {
	Name   string
	Data   []byte
	Date   uint16
	Time   uint16
	Attrib uint16
}

type cabOpts struct {
	Compress     int  // 0 none, 1 MSZIP
	NoDictCarry  bool // compress each block without history (valid, larger)
	WrongDict    bool // compress against a shifted history (invalid on purpose)
	SetID        uint16
	ZeroChecksum bool
	Level        int
}

func cabChecksum(b []byte, seed uint32) uint32 {
	acc := seed
	n := len(b) / 4
	for i := 0; i < n; i++ {
		acc ^= binary.LittleEndian.Uint32(b[4*i:])
	}
	var tail uint32
	for _, c := range b[4*n:] {
		tail = tail<<8 | uint32(c)
	}
	return acc ^ tail
}

func writeCab(files []cabFile, o cabOpts) ([]byte, error) {
	var folder []byte
	for _, f := range files {
		folder = append(folder, f.Data...)
	}
	type block struct {
		data  []byte
		uncmp int
	}
	var blocks []block
	for k := 0; k*32768 < len(folder); k++ {
		lo, hi := k*32768, (k+1)*32768
		if hi > len(folder) {
			hi = len(folder)
		}
		chunk := folder[lo:hi]
		var ab []byte
		if o.Compress == 1 {
			dictLo := lo - 32768
			if dictLo < 0 {
				dictLo = 0
			}
			dict := folder[dictLo:lo]
			if o.NoDictCarry {
				dict = nil
			}
			if o.WrongDict && lo > 0 {
				// history shifted by 1 KiB: matches point at the wrong bytes
				s := dictLo - 1024
				if s < 0 {
					s = 0
				}
				dict = folder[s : lo-1024]
			}
			var buf bytes.Buffer
			lvl := o.Level
			if lvl == 0 {
				lvl = 9
			}
			w, err := flate.NewWriterDict(&buf, lvl, dict)
			if err != nil {
				return nil, err
			}
			w.Write(chunk)
			w.Close()
			ab = append([]byte("CK"), buf.Bytes()...)
			if buf.Len() > len(chunk)+5 {
				st := []byte{'C', 'K', 1, 0, 0, 0, 0}
				binary.LittleEndian.PutUint16(st[3:], uint16(len(chunk)))
				binary.LittleEndian.PutUint16(st[5:], uint16(len(chunk))^0xFFFF)
				ab = append(st, chunk...)
			}
		} else {
			ab = chunk
		}
		blocks = append(blocks, block{ab, len(chunk)})
	}
	coffFiles := 36 + 8
	dataStart := coffFiles
	for _, f := range files {
		dataStart += 17 + len(f.Name)
	}
	total := dataStart
	for _, b := range blocks {
		total += 8 + len(b.data)
	}
	le := binary.LittleEndian
	out := make([]byte, 0, total)
	h := make([]byte, 36)
	copy(h, "MSCF")
	le.PutUint32(h[8:], uint32(total))
	le.PutUint32(h[16:], uint32(coffFiles))
	h[0x18], h[0x19] = 3, 1
	le.PutUint16(h[0x1A:], 1)
	le.PutUint16(h[0x1C:], uint16(len(files)))
	le.PutUint16(h[0x20:], o.SetID)
	out = append(out, h...)
	fo := make([]byte, 8)
	le.PutUint32(fo, uint32(dataStart))
	le.PutUint16(fo[4:], uint16(len(blocks)))
	le.PutUint16(fo[6:], uint16(o.Compress))
	out = append(out, fo...)
	off := 0
	for _, f := range files {
		e := make([]byte, 16)
		le.PutUint32(e, uint32(len(f.Data)))
		le.PutUint32(e[4:], uint32(off))
		le.PutUint16(e[10:], f.Date)
		le.PutUint16(e[12:], f.Time)
		le.PutUint16(e[14:], f.Attrib)
		out = append(out, e...)
		out = append(out, f.Name...)
		out = append(out, 0)
		off += len(f.Data)
	}
	for _, b := range blocks {
		d := make([]byte, 8)
		le.PutUint16(d[4:], uint16(len(b.data)))
		le.PutUint16(d[6:], uint16(b.uncmp))
		if !o.ZeroChecksum {
			cs := cabChecksum(b.data, 0) ^ (uint32(len(b.data)) | uint32(b.uncmp)<<16)
			le.PutUint32(d, cs)
		}
		out = append(out, d...)
		out = append(out, b.data...)
	}
	return out, nil
}

// readCab extracts files from a single-folder MSZIP or stored cabinet.
func readCab(c []byte) ([]cabFile, error) {
	le := binary.LittleEndian
	if len(c) < 36 || string(c[:4]) != "MSCF" {
		return nil, fmt.Errorf("not a cabinet")
	}
	nFolders := int(le.Uint16(c[0x1A:]))
	nFiles := int(le.Uint16(c[0x1C:]))
	coffFiles := int(le.Uint32(c[0x10:]))
	var folders [][]byte
	for fi := 0; fi < nFolders; fi++ {
		fo := c[36+8*fi:]
		start := int(le.Uint32(fo))
		nData := int(le.Uint16(fo[4:]))
		typ := le.Uint16(fo[6:]) & 0xF
		var out []byte
		p := start
		for k := 0; k < nData; k++ {
			csum := le.Uint32(c[p:])
			cb := int(le.Uint16(c[p+4:]))
			un := int(le.Uint16(c[p+6:]))
			ab := c[p+8 : p+8+cb]
			if csum != 0 {
				if want := cabChecksum(ab, 0) ^ (uint32(cb) | uint32(un)<<16); want != csum {
					return nil, fmt.Errorf("folder %d block %d checksum %#x want %#x", fi, k, csum, want)
				}
			}
			switch typ {
			case 0:
				out = append(out, ab...)
			case 1:
				if string(ab[:2]) != "CK" {
					return nil, fmt.Errorf("missing CK")
				}
				hist := out
				if len(hist) > 32768 {
					hist = hist[len(hist)-32768:]
				}
				r := flate.NewReaderDict(bytes.NewReader(ab[2:]), hist)
				buf := make([]byte, un)
				if _, err := io.ReadFull(r, buf); err != nil {
					return nil, fmt.Errorf("block %d: %w", k, err)
				}
				out = append(out, buf...)
			default:
				return nil, fmt.Errorf("compression %d unsupported", typ)
			}
			p += 8 + cb
		}
		folders = append(folders, out)
	}
	var files []cabFile
	p := coffFiles
	for i := 0; i < nFiles; i++ {
		size := int(le.Uint32(c[p:]))
		off := int(le.Uint32(c[p+4:]))
		fi := int(le.Uint16(c[p+8:]))
		end := bytes.IndexByte(c[p+16:], 0)
		name := string(c[p+16 : p+16+end])
		f := cabFile{Name: name, Date: le.Uint16(c[p+10:]), Time: le.Uint16(c[p+12:]), Attrib: le.Uint16(c[p+14:])}
		if fi < len(folders) && off+size <= len(folders[fi]) {
			f.Data = folders[fi][off : off+size]
		}
		files = append(files, f)
		p += 17 + end
	}
	return files, nil
}
