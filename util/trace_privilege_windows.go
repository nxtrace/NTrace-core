//go:build windows

package util

func TracePrivilegeStatus(_ string, requireWindowsAdmin bool) TracePrivilegeCheck {
	if !requireWindowsAdmin || HasAdminPrivileges() {
		return TracePrivilegeCheck{}
	}
	return TracePrivilegeCheck{
		Message: "Windows 下 --mtu 需要管理员权限。当前实现依赖 WinDivert 或原始 ICMP 套接字；普通权限下无法可靠工作。请使用“以管理员身份运行”的终端重试。",
		Fatal:   true,
	}
}
