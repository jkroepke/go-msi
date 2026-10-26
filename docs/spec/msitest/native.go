package main

// Thin wrappers over the native msi.dll database API (syscall only, no cgo).

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	msiDLL                    = syscall.NewLazyDLL("msi.dll")
	pMsiOpenDatabaseW         = msiDLL.NewProc("MsiOpenDatabaseW")
	pMsiDatabaseOpenViewW     = msiDLL.NewProc("MsiDatabaseOpenViewW")
	pMsiViewExecute           = msiDLL.NewProc("MsiViewExecute")
	pMsiViewFetch             = msiDLL.NewProc("MsiViewFetch")
	pMsiViewClose             = msiDLL.NewProc("MsiViewClose")
	pMsiViewModify            = msiDLL.NewProc("MsiViewModify")
	pMsiCloseHandle           = msiDLL.NewProc("MsiCloseHandle")
	pMsiRecordGetFieldCount   = msiDLL.NewProc("MsiRecordGetFieldCount")
	pMsiRecordGetStringW      = msiDLL.NewProc("MsiRecordGetStringW")
	pMsiRecordIsNull          = msiDLL.NewProc("MsiRecordIsNull")
	pMsiRecordDataSize        = msiDLL.NewProc("MsiRecordDataSize")
	pMsiRecordReadStream      = msiDLL.NewProc("MsiRecordReadStream")
	pMsiCreateRecord          = msiDLL.NewProc("MsiCreateRecord")
	pMsiRecordSetStringW      = msiDLL.NewProc("MsiRecordSetStringW")
	pMsiRecordSetStreamW      = msiDLL.NewProc("MsiRecordSetStreamW")
	pMsiDatabaseCommit        = msiDLL.NewProc("MsiDatabaseCommit")
	pMsiDatabaseImportW       = msiDLL.NewProc("MsiDatabaseImportW")
	pMsiDatabaseExportW       = msiDLL.NewProc("MsiDatabaseExportW")
	pMsiGetSummaryInformation = msiDLL.NewProc("MsiGetSummaryInformationW")
	pMsiSummaryInfoSetProp    = msiDLL.NewProc("MsiSummaryInfoSetPropertyW")
	pMsiSummaryInfoGetProp    = msiDLL.NewProc("MsiSummaryInfoGetPropertyW")
	pMsiSummaryInfoPersist    = msiDLL.NewProc("MsiSummaryInfoPersist")
	pMsiGetFileHashW          = msiDLL.NewProc("MsiGetFileHashW")
	pMsiGetLastErrorRecord    = msiDLL.NewProc("MsiGetLastErrorRecord")
	pMsiFormatRecordW         = msiDLL.NewProc("MsiFormatRecordW")
	pMsiSetInternalUI         = msiDLL.NewProc("MsiSetInternalUI")
)

const (
	dbReadOnly     = 0
	dbTransact     = 1
	dbDirect       = 2
	dbCreate       = 3
	dbCreateDirect = 4

	errNoMoreItems = 259

	vtI2       = 2
	vtI4       = 3
	vtLPSTR    = 30
	vtFILETIME = 64
)

type msiError struct {
	Op   string
	Code uintptr
	Info string
}

func (e *msiError) Error() string {
	if e.Info != "" {
		return fmt.Sprintf("%s: %d (%s)", e.Op, e.Code, e.Info)
	}
	return fmt.Sprintf("%s: %d", e.Op, e.Code)
}

func codeOf(err error) uintptr {
	if e, ok := err.(*msiError); ok {
		return e.Code
	}
	if err == nil {
		return 0
	}
	return ^uintptr(0)
}

func lastErrorInfo() string {
	r, _, _ := pMsiGetLastErrorRecord.Call()
	if r == 0 {
		return ""
	}
	defer closeHandle(uint32(r))
	n := uint32(0)
	buf := make([]uint16, 1)
	pMsiFormatRecordW.Call(0, r, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	n++
	buf = make([]uint16, n)
	pMsiFormatRecordW.Call(0, r, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	return syscall.UTF16ToString(buf)
}

// call runs p and turns a non-zero result into an *msiError. Convert pointers
// to uintptr only inside its argument list: uintptrescapes then keeps them on
// the heap and alive for the call, as it does for (*syscall.LazyProc).Call.
//
//go:uintptrescapes
func call(op string, p *syscall.LazyProc, args ...uintptr) error {
	r, _, _ := p.Call(args...)
	if r != 0 {
		return &msiError{Op: op, Code: r, Info: lastErrorInfo()}
	}
	return nil
}

func wstr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func closeHandle(h uint32) { pMsiCloseHandle.Call(uintptr(h)) }

func openDB(path string, mode uintptr) (uint32, error) {
	var h uint32
	err := call("MsiOpenDatabaseW", pMsiOpenDatabaseW, uintptr(unsafe.Pointer(wstr(path))), mode, uintptr(unsafe.Pointer(&h)))
	return h, err
}

func commit(db uint32) error { return call("MsiDatabaseCommit", pMsiDatabaseCommit, uintptr(db)) }

// exec runs a statement and returns all rows as strings (NULL as "\x00NULL").
func query(db uint32, sql string, params ...string) ([][]string, error) {
	var view uint32
	if err := call("MsiDatabaseOpenViewW", pMsiDatabaseOpenViewW, uintptr(db), uintptr(unsafe.Pointer(wstr(sql))), uintptr(unsafe.Pointer(&view))); err != nil {
		return nil, err
	}
	defer closeHandle(view)
	var rec uintptr
	if len(params) > 0 {
		rec, _, _ = pMsiCreateRecord.Call(uintptr(len(params)))
		defer closeHandle(uint32(rec))
		for i, p := range params {
			pMsiRecordSetStringW.Call(rec, uintptr(i+1), uintptr(unsafe.Pointer(wstr(p))))
		}
	}
	if err := call("MsiViewExecute", pMsiViewExecute, uintptr(view), rec); err != nil {
		return nil, err
	}
	defer pMsiViewClose.Call(uintptr(view))
	var rows [][]string
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		return nil, nil
	}
	for {
		var r uint32
		code, _, _ := pMsiViewFetch.Call(uintptr(view), uintptr(unsafe.Pointer(&r)))
		if code == errNoMoreItems {
			break
		}
		if code != 0 {
			return rows, &msiError{Op: "MsiViewFetch", Code: code, Info: lastErrorInfo()}
		}
		rows = append(rows, recordStrings(r))
		closeHandle(r)
	}
	return rows, nil
}

const nullMark = "\x00NULL"

func recordStrings(r uint32) []string {
	n, _, _ := pMsiRecordGetFieldCount.Call(uintptr(r))
	out := make([]string, n)
	for i := uintptr(1); i <= n; i++ {
		isNull, _, _ := pMsiRecordIsNull.Call(uintptr(r), i)
		if isNull != 0 {
			out[i-1] = nullMark
			continue
		}
		out[i-1] = recordString(r, i)
	}
	return out
}

func recordString(r uint32, i uintptr) string {
	sz := uint32(0)
	var dummy [1]uint16
	code, _, _ := pMsiRecordGetStringW.Call(uintptr(r), i, uintptr(unsafe.Pointer(&dummy[0])), uintptr(unsafe.Pointer(&sz)))
	if code != 0 && code != 234 {
		// binary stream field: report its size
		ds, _, _ := pMsiRecordDataSize.Call(uintptr(r), i)
		return fmt.Sprintf("<stream %d bytes>", ds)
	}
	sz++
	buf := make([]uint16, sz)
	pMsiRecordGetStringW.Call(uintptr(r), i, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&sz)))
	return syscall.UTF16ToString(buf)
}

// readStreamField fetches the binary field of the first row of a query.
func readStreamField(db uint32, sql string, field uintptr) ([]byte, error) {
	var view uint32
	if err := call("MsiDatabaseOpenViewW", pMsiDatabaseOpenViewW, uintptr(db), uintptr(unsafe.Pointer(wstr(sql))), uintptr(unsafe.Pointer(&view))); err != nil {
		return nil, err
	}
	defer closeHandle(view)
	if err := call("MsiViewExecute", pMsiViewExecute, uintptr(view), 0); err != nil {
		return nil, err
	}
	var r uint32
	if err := call("MsiViewFetch", pMsiViewFetch, uintptr(view), uintptr(unsafe.Pointer(&r))); err != nil {
		return nil, err
	}
	defer closeHandle(r)
	var out []byte
	buf := make([]byte, 65536)
	for {
		n := uint32(len(buf))
		if err := call("MsiRecordReadStream", pMsiRecordReadStream, uintptr(r), field, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); err != nil {
			return out, err
		}
		if n == 0 {
			break
		}
		out = append(out, buf[:n]...)
	}
	return out, nil
}

// insertStream inserts a row (name, file contents) into a two-column table whose
// second column is binary, e.g. _Streams or Binary.
func insertStream(db uint32, table, name, file string) error {
	var view uint32
	sql := fmt.Sprintf("SELECT * FROM `%s`", table)
	if err := call("MsiDatabaseOpenViewW", pMsiDatabaseOpenViewW, uintptr(db), uintptr(unsafe.Pointer(wstr(sql))), uintptr(unsafe.Pointer(&view))); err != nil {
		return err
	}
	defer closeHandle(view)
	if err := call("MsiViewExecute", pMsiViewExecute, uintptr(view), 0); err != nil {
		return err
	}
	rec, _, _ := pMsiCreateRecord.Call(2)
	defer closeHandle(uint32(rec))
	pMsiRecordSetStringW.Call(rec, 1, uintptr(unsafe.Pointer(wstr(name))))
	if err := call("MsiRecordSetStreamW", pMsiRecordSetStreamW, rec, 2, uintptr(unsafe.Pointer(wstr(file)))); err != nil {
		return err
	}
	const modifyInsert = 1
	return call("MsiViewModify", pMsiViewModify, uintptr(view), modifyInsert, rec)
}

func importIDT(db uint32, dir, file string) error {
	return call("MsiDatabaseImportW("+file+")", pMsiDatabaseImportW, uintptr(db), uintptr(unsafe.Pointer(wstr(dir))), uintptr(unsafe.Pointer(wstr(file))))
}

func exportIDT(db uint32, table, dir, file string) error {
	return call("MsiDatabaseExportW", pMsiDatabaseExportW, uintptr(db), uintptr(unsafe.Pointer(wstr(table))), uintptr(unsafe.Pointer(wstr(dir))), uintptr(unsafe.Pointer(wstr(file))))
}

type sumProp struct {
	PID  uint32
	VT   uint32
	Int  int32
	FT   [2]uint32
	Text string
}

func openSummary(db uint32, path string, updates uint32) (uint32, error) {
	var h uint32
	var p *uint16
	if path != "" {
		p = wstr(path)
	}
	err := call("MsiGetSummaryInformationW", pMsiGetSummaryInformation, uintptr(db), uintptr(unsafe.Pointer(p)), uintptr(updates), uintptr(unsafe.Pointer(&h)))
	return h, err
}

func setSummary(h uint32, p sumProp) error {
	var ft *[2]uint32
	if p.VT == vtFILETIME {
		ft = &p.FT
	}
	var s *uint16
	if p.VT == vtLPSTR {
		s = wstr(p.Text)
	}
	return call(fmt.Sprintf("MsiSummaryInfoSetPropertyW(%d)", p.PID), pMsiSummaryInfoSetProp, uintptr(h), uintptr(p.PID), uintptr(p.VT), uintptr(p.Int),
		uintptr(unsafe.Pointer(ft)), uintptr(unsafe.Pointer(s)))
}

func getSummary(h uint32, pid uint32) (sumProp, error) {
	p := sumProp{PID: pid}
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	err := call("MsiSummaryInfoGetPropertyW", pMsiSummaryInfoGetProp, uintptr(h), uintptr(pid),
		uintptr(unsafe.Pointer(&p.VT)), uintptr(unsafe.Pointer(&p.Int)), uintptr(unsafe.Pointer(&p.FT)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	p.Text = syscall.UTF16ToString(buf)
	return p, err
}

func persistSummary(h uint32) error {
	return call("MsiSummaryInfoPersist", pMsiSummaryInfoPersist, uintptr(h))
}

func fileHash(path string) ([4]int32, error) {
	var info struct {
		Size uint32
		Data [4]int32
	}
	info.Size = 20
	err := call("MsiGetFileHashW", pMsiGetFileHashW, uintptr(unsafe.Pointer(wstr(path))), 0, uintptr(unsafe.Pointer(&info)))
	return info.Data, err
}
