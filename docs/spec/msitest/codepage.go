package main

import (
	"syscall"
	"unsafe"
)

var pWideCharToMultiByte = syscall.NewLazyDLL("kernel32.dll").NewProc("WideCharToMultiByte")

// cpEncode converts s to the given Windows code page, failing on unmappable text.
func cpEncode(s string, cp int) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	u, _ := syscall.UTF16FromString(s)
	u = u[:len(u)-1]
	var used int32
	n, _, err := pWideCharToMultiByte.Call(uintptr(cp), 0, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), 0, 0, 0, uintptr(unsafe.Pointer(&used)))
	if n == 0 {
		return nil, err
	}
	out := make([]byte, n)
	pWideCharToMultiByte.Call(uintptr(cp), 0, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&out[0])), n, 0, uintptr(unsafe.Pointer(&used)))
	if used != 0 {
		return out, syscall.Errno(1113) // ERROR_NO_UNICODE_TRANSLATION
	}
	return out, nil
}
