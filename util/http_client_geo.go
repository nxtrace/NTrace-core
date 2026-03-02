package util

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewGeoHTTPClient 返回一个使用 Geo DNS 解析策略的 *http.Client。
//
// 内部 Transport.DialContext 会通过 LookupHostForGeo 解析目标 host，
// 然后按 IP 拨号，保持请求 URL Host 不变（即 TLS SNI 不受影响）。
func NewGeoHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				// addr 可能不含端口号；原样拨号
				return dialer.DialContext(ctx, network, addr)
			}

			// 用 Geo DNS 策略解析 host
			ips, err := LookupHostForGeo(ctx, host)
			if err != nil {
				return nil, err
			}

			// 依次尝试解析到的 IP，优先使用地址族匹配的
			var lastErr error
			for _, ip := range ips {
				target := net.JoinHostPort(ip.String(), port)
				conn, dialErr := dialer.DialContext(ctx, network, target)
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			return nil, lastErr
		},
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
