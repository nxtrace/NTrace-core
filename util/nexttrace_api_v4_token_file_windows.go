//go:build windows

package util

import (
	"os"

	"golang.org/x/sys/windows"
)

func strictNextTraceAPIV4TokenPerms() bool {
	return false
}

func checkNextTraceAPIV4TokenDirOwner(info os.FileInfo) error {
	return nil
}

func replaceNextTraceAPIV4TokenFile(tmpPath, path string) error {
	if err := rejectNextTraceAPIV4Symlink(path); err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(tmpPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
