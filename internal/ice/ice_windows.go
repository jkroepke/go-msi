//go:build windows

package ice

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

var (
	msiDLL                 = syscall.NewLazyDLL("msi.dll")           //nolint:gochecknoglobals // one DLL table for the file
	pMsiOpenDatabaseW      = msiDLL.NewProc("MsiOpenDatabaseW")      //nolint:gochecknoglobals // one DLL table for the file
	pMsiDatabaseMergeW     = msiDLL.NewProc("MsiDatabaseMergeW")     //nolint:gochecknoglobals // one DLL table for the file
	pMsiDatabaseOpenViewW  = msiDLL.NewProc("MsiDatabaseOpenViewW")  //nolint:gochecknoglobals // one DLL table for the file
	pMsiViewExecute        = msiDLL.NewProc("MsiViewExecute")        //nolint:gochecknoglobals // one DLL table for the file
	pMsiViewFetch          = msiDLL.NewProc("MsiViewFetch")          //nolint:gochecknoglobals // one DLL table for the file
	pMsiViewClose          = msiDLL.NewProc("MsiViewClose")          //nolint:gochecknoglobals // one DLL table for the file
	pMsiRecordGetStringW   = msiDLL.NewProc("MsiRecordGetStringW")   //nolint:gochecknoglobals // one DLL table for the file
	pMsiCloseHandle        = msiDLL.NewProc("MsiCloseHandle")        //nolint:gochecknoglobals // one DLL table for the file
	pMsiSetInternalUI      = msiDLL.NewProc("MsiSetInternalUI")      //nolint:gochecknoglobals // one DLL table for the file
	pMsiSetExternalUIW     = msiDLL.NewProc("MsiSetExternalUIW")     //nolint:gochecknoglobals // one DLL table for the file
	pMsiOpenPackageExW     = msiDLL.NewProc("MsiOpenPackageExW")     //nolint:gochecknoglobals // one DLL table for the file
	pMsiEvaluateConditionW = msiDLL.NewProc("MsiEvaluateConditionW") //nolint:gochecknoglobals // one DLL table for the file
	pMsiDoActionW          = msiDLL.NewProc("MsiDoActionW")          //nolint:gochecknoglobals // one DLL table for the file
	pMsiGetLastErrorRecord = msiDLL.NewProc("MsiGetLastErrorRecord") //nolint:gochecknoglobals // one DLL table for the file
	pMsiFormatRecordW      = msiDLL.NewProc("MsiFormatRecordW")      //nolint:gochecknoglobals // one DLL table for the file
)

const (
	errorFunctionFailed       = uintptr(1627)
	errorNoMoreItems          = uintptr(259)
	errorMoreData             = uintptr(234)
	msidbOpenReadOnly         = uintptr(0)
	msidbOpenTransact         = uintptr(1)
	installUILevelNone        = uintptr(2)
	ignoreMachineState        = uintptr(1) // MSIOPENPACKAGEFLAGS_IGNOREMACHINESTATE
	conditionFalse            = uintptr(0)
	conditionError            = uintptr(3)
	installMessageTypeMask    = uintptr(0xFF000000)
	installMessageError       = uintptr(0x01000000)
	installMessageWarning     = uintptr(0x02000000)
	installMessageUser        = uintptr(0x03000000)
	installLogModeErrWarnUser = uintptr(1<<1 | 1<<2 | 1<<3)
	idOK                      = uintptr(1)
)

// message is one message the external UI handler received.
type message struct {
	text string
	kind uintptr
}

// The external UI handler is process-wide, so one session runs at a time and
// the callback (created once: callbacks are never freed) feeds the active one.
var (
	session     sync.Mutex //nolint:gochecknoglobals // the external UI handler is process-wide
	sinkMu      sync.Mutex //nolint:gochecknoglobals // the external UI handler is process-wide
	sink        *[]message //nolint:gochecknoglobals // the external UI handler is process-wide
	uiCallback  uintptr    //nolint:gochecknoglobals // the external UI handler is process-wide
	callbackSet sync.Once  //nolint:gochecknoglobals // the external UI handler is process-wide
)

func handler(_, msgType uintptr, msg *uint16) uintptr {
	kind := msgType & installMessageTypeMask

	switch kind {
	case installMessageUser, installMessageError, installMessageWarning:
		if msg != nil {
			text := utf16PtrToString(msg)

			sinkMu.Lock()

			if sink != nil {
				*sink = append(*sink, message{kind: kind, text: text})
			}

			sinkMu.Unlock()
		}

		return idOK // handled: the ICE goes on
	default:
		return 0
	}
}

// Run validates the package at pkg with the evaluators in cub, for example
// Darice.cub. Neither file is changed: the merge happens in a temporary copy.
// It returns every message the ICE actions posted, in order. An ICE action
// that fails to run is reported as a Failure finding.
func Run(pkg, cub string) ([]Finding, error) {
	session.Lock()
	defer session.Unlock()

	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	tmp, err := os.MkdirTemp("", "go-msi-ice-")
	if err != nil {
		return nil, fmt.Errorf("ice: %w", err)
	}

	defer os.RemoveAll(tmp)

	work := filepath.Join(tmp, "package.msi")
	if err := copyFile(pkg, work); err != nil {
		return nil, err
	}

	database, err := openDatabase(work, msidbOpenTransact)
	if err != nil {
		return nil, err
	}

	defer closeHandle(database)

	cubDB, err := openDatabase(cub, msidbOpenReadOnly)
	if err != nil {
		return nil, err
	}

	defer closeHandle(cubDB)

	findings, err := merge(database, cubDB)
	if err != nil {
		return nil, err
	}

	sequence, err := query(database, "SELECT `Action`, `Condition` FROM `_ICESequence` ORDER BY `Sequence`")
	if err != nil {
		return nil, fmt.Errorf("ice: %s has no usable _ICESequence: %w", cub, err)
	}

	pkgHandle, release, err := openSession(database)
	if err != nil {
		return nil, err
	}

	defer release()

	for _, row := range sequence {
		findings = append(findings, runAction(pkgHandle, row[0], row[1])...)
	}

	return findings, nil
}

// merge merges the validation file into the package. Row conflicts are listed
// per table in _MergeErrors; the merge itself still goes through.
func merge(database, cubDB uint32) ([]Finding, error) {
	ret, _, _ := pMsiDatabaseMergeW.Call(uintptr(database), uintptr(cubDB), uintptr(unsafe.Pointer(wstr("_MergeErrors"))))

	switch ret {
	case 0:
		return nil, nil
	case errorFunctionFailed:
		rows, err := query(database, "SELECT `Table`, `NumRowMergeConflicts` FROM `_MergeErrors`")
		if err != nil {
			return nil, err
		}

		findings := make([]Finding, 0, len(rows))
		for _, row := range rows {
			findings = append(findings, Finding{ICE: "merge", Type: Failure, Description: row[1] + " row conflicts merging the validation file", Table: row[0]})
		}

		return findings, nil
	default:
		return nil, fmt.Errorf("ice: MsiDatabaseMergeW: error %d", ret)
	}
}

// messages collects what the handler receives during one session.
var messages []message //nolint:gochecknoglobals // guarded by session; the handler appends through sink

// openSession opens the merged database as a package that ignores the machine
// state, with the external UI handler installed. release undoes all of it.
func openSession(database uint32) (uint32, func(), error) {
	callbackSet.Do(func() { uiCallback = syscall.NewCallback(handler) })

	messages = messages[:0]

	sinkMu.Lock()
	sink = &messages
	sinkMu.Unlock()

	prevLevel, _, _ := pMsiSetInternalUI.Call(installUILevelNone, 0)
	prevUI, _, _ := pMsiSetExternalUIW.Call(uiCallback, installLogModeErrWarnUser, 0)

	restore := func() {
		_, _, _ = pMsiSetExternalUIW.Call(prevUI, 0, 0)
		_, _, _ = pMsiSetInternalUI.Call(prevLevel, 0)

		sinkMu.Lock()
		sink = nil
		sinkMu.Unlock()
	}

	var pkgHandle uint32

	name := wstr("#" + strconv.FormatUint(uint64(database), 10))

	ret, _, _ := pMsiOpenPackageExW.Call(uintptr(unsafe.Pointer(name)), ignoreMachineState, uintptr(unsafe.Pointer(&pkgHandle)))
	if ret != 0 {
		restore()

		return 0, nil, fmt.Errorf("ice: MsiOpenPackageExW: error %d%s", ret, lastError())
	}

	return pkgHandle, func() { closeHandle(pkgHandle); restore() }, nil
}

// runAction runs one ICE action when its condition holds and returns what it
// posted.
func runAction(pkgHandle uint32, action, condition string) []Finding {
	if condition != "" {
		cond, _, _ := pMsiEvaluateConditionW.Call(uintptr(pkgHandle), uintptr(unsafe.Pointer(wstr(condition))))

		switch cond {
		case conditionFalse:
			return nil
		case conditionError:
			return []Finding{{ICE: action, Type: Failure, Description: "condition does not parse: " + condition}}
		}
	}

	ret, _, _ := pMsiDoActionW.Call(uintptr(pkgHandle), uintptr(unsafe.Pointer(wstr(action))))

	var findings []Finding

	sinkMu.Lock()

	for _, msg := range messages {
		if finding, ok := Parse(msg.text); ok {
			findings = append(findings, finding)
		} else if msg.kind == installMessageError {
			findings = append(findings, Finding{ICE: action, Type: Failure, Description: msg.text})
		}
	}

	messages = messages[:0]

	sinkMu.Unlock()

	if ret != 0 && len(findings) == 0 {
		findings = append(findings, Finding{ICE: action, Type: Failure, Description: fmt.Sprintf("action returned %d%s", ret, lastError())})
	}

	return findings
}

func openDatabase(path string, mode uintptr) (uint32, error) {
	var handle uint32

	ret, _, _ := pMsiOpenDatabaseW.Call(uintptr(unsafe.Pointer(wstr(path))), mode, uintptr(unsafe.Pointer(&handle)))
	if ret != 0 {
		return 0, fmt.Errorf("ice: MsiOpenDatabaseW %s: error %d", path, ret)
	}

	return handle, nil
}

// query returns the first two columns of every row.
func query(database uint32, sql string) ([][]string, error) {
	var view uint32

	ret, _, _ := pMsiDatabaseOpenViewW.Call(uintptr(database), uintptr(unsafe.Pointer(wstr(sql))), uintptr(unsafe.Pointer(&view)))
	if ret != 0 {
		return nil, fmt.Errorf("ice: %s: error %d", sql, ret)
	}

	defer closeHandle(view)

	if ret, _, _ := pMsiViewExecute.Call(uintptr(view), 0); ret != 0 {
		return nil, fmt.Errorf("ice: %s: MsiViewExecute error %d", sql, ret)
	}

	defer func() { _, _, _ = pMsiViewClose.Call(uintptr(view)) }()

	var rows [][]string

	for {
		var rec uint32

		ret, _, _ := pMsiViewFetch.Call(uintptr(view), uintptr(unsafe.Pointer(&rec)))
		if ret == errorNoMoreItems {
			return rows, nil
		}

		if ret != 0 {
			return nil, fmt.Errorf("ice: %s: MsiViewFetch error %d", sql, ret)
		}

		rows = append(rows, []string{recordString(rec, 1), recordString(rec, 2)})
		closeHandle(rec)
	}
}

func recordString(rec uint32, field uintptr) string {
	size := uint32(256)

	for {
		buf := make([]uint16, size)

		ret, _, _ := pMsiRecordGetStringW.Call(uintptr(rec), field, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
		if ret == errorMoreData {
			size++ // the size returned excludes the terminator

			continue
		}

		return syscall.UTF16ToString(buf[:size])
	}
}

// lastError formats the extended error record of the last failed call.
func lastError() string {
	rec, _, _ := pMsiGetLastErrorRecord.Call()
	if rec == 0 {
		return ""
	}

	defer func() { _, _, _ = pMsiCloseHandle.Call(rec) }()

	size := uint32(1024)
	buf := make([]uint16, size)

	if ret, _, _ := pMsiFormatRecordW.Call(0, rec, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); ret != 0 {
		return ""
	}

	return " (" + syscall.UTF16ToString(buf[:size]) + ")"
}

func closeHandle(h uint32) {
	_, _, _ = pMsiCloseHandle.Call(uintptr(h)) //nolint:dogsled // MsiCloseHandle has nothing worth checking
}

func wstr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}

	return p
}

func utf16PtrToString(ptr *uint16) string {
	var units []uint16

	for p := unsafe.Pointer(ptr); ; p = unsafe.Add(p, 2) {
		unit := *(*uint16)(p)
		if unit == 0 {
			return syscall.UTF16ToString(units)
		}

		units = append(units, unit)
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("ice: %w", err)
	}

	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return fmt.Errorf("ice: %w", err)
	}

	return nil
}
