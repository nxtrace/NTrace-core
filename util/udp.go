package util

import (
	"net"
)

func newUDPResolver() *net.Resolver {
	return &net.Resolver{
		// 指定使用 Go Build-in 的 DNS Resolver 来解析
		PreferGo: true,
	}
}
