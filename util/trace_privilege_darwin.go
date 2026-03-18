//go:build darwin

package util

// macOS 上旧逻辑不会因为权限检查阻断执行，这里保持同样语义。
func TracePrivilegeStatus(string, bool) TracePrivilegeCheck {
	return TracePrivilegeCheck{}
}
