//go:build linux

package util

import (
	"fmt"
	"os"

	"github.com/syndtr/gocapability/capability"
)

func TracePrivilegeStatus(appBinName string, _ bool) TracePrivilegeCheck {
	if os.Getuid() == 0 {
		return TracePrivilegeCheck{}
	}

	caps, err := capability.NewPid2(0)
	if err != nil {
		return TracePrivilegeCheck{Message: fmt.Sprintf("读取进程能力信息失败: %v", err)}
	}
	if err := caps.Load(); err != nil {
		return TracePrivilegeCheck{Message: fmt.Sprintf("加载进程能力信息失败: %v", err)}
	}
	if caps.Get(capability.EFFECTIVE, capability.CAP_NET_RAW) && caps.Get(capability.EFFECTIVE, capability.CAP_NET_ADMIN) {
		return TracePrivilegeCheck{}
	}

	return TracePrivilegeCheck{
		Message: fmt.Sprintf(
			"您正在以普通用户权限运行 NextTrace，但 NextTrace 未被赋予监听网络套接字的ICMP消息包、修改IP头信息（TTL）等路由跟踪所需的权限\n"+
				"请使用管理员用户执行 `sudo setcap cap_net_raw,cap_net_admin+eip ${your_nexttrace_path}/%s` 命令，赋予相关权限后再运行~\n"+
				"什么？为什么 ping 普通用户执行不要 root 权限？因为这些工具在管理员安装时就已经被赋予了一些必要的权限，具体请使用 `getcap /usr/bin/ping` 查看",
			appBinName,
		),
	}
}
