//go:build !linux && !darwin && !windows

package util

func TracePrivilegeStatus(string, bool) TracePrivilegeCheck {
	return TracePrivilegeCheck{}
}
