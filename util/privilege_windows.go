//go:build windows

package util

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// HasAdminPrivileges reports whether the current Windows process is elevated.
func HasAdminPrivileges() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer func() {
		_ = token.Close()
	}()

	type tokenElevation struct {
		TokenIsElevated uint32
	}
	var elev tokenElevation
	var outLen uint32
	if err := windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elev)),
		uint32(unsafe.Sizeof(elev)),
		&outLen,
	); err != nil {
		return false
	}
	return elev.TokenIsElevated != 0
}
