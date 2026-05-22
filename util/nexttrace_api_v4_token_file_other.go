//go:build !(aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package util

import "os"

func strictNextTraceAPIV4TokenPerms() bool {
	return false
}

func checkNextTraceAPIV4TokenDirOwner(info os.FileInfo) error {
	return nil
}
