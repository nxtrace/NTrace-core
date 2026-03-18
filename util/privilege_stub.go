//go:build !windows

package util

func HasAdminPrivileges() bool {
	return true
}
