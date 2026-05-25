//go:build !(windows || aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package util

import "os"

func strictNextTraceAPIV4TokenPerms() bool {
	return false
}

func checkNextTraceAPIV4TokenDirOwner(info os.FileInfo) error {
	return nil
}

func replaceNextTraceAPIV4TokenFile(tmpPath, path string) error {
	return os.Rename(tmpPath, path)
}
