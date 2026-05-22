//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package util

import (
	"fmt"
	"os"
	"syscall"
)

func strictNextTraceAPIV4TokenPerms() bool {
	return true
}

func checkNextTraceAPIV4TokenDirOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot verify owner of NextTrace API v4 token directory")
	}
	if int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("NextTrace API v4 token directory is not owned by current user")
	}
	return nil
}
