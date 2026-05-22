//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package util

import (
	"fmt"
	"os"
	"syscall"
)

func checkNextTraceAPIV4TokenDirOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if int(stat.Uid) != os.Getuid() {
		return fmt.Errorf("NextTrace API v4 token directory is not owned by current user")
	}
	return nil
}
