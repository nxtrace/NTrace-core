package util

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

func newDoTResolver(serverName string, addrs string) *net.Resolver {

	d := &net.Dialer{
		// 设置超时时间
		Timeout: 1000 * time.Millisecond,
	}

	tlsConfig := &tls.Config{
		// 设置 TLS Server Name 以确保证书能和域名对应
		ServerName: serverName,
	}
	return &net.Resolver{
		// 指定使用 Go Build-in 的 DNS Resolver 来解析
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := d.DialContext(ctx, "tcp", addrs)
			if err != nil {
				return nil, err
			}
			return tls.Client(conn, tlsConfig), nil
		},
	}
}
