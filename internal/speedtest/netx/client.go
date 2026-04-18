package netx

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"github.com/nxtrace/NTrace-core/util"
)

type Options struct {
	PinHost string
	PinIP   string
	Timeout time.Duration
	LocalIP net.IP
}

var (
	resolveAllIPsFn = resolveAllIPs
	dialContextFn   = func(d *net.Dialer, ctx context.Context, network, addr string) (net.Conn, error) {
		return d.DialContext(ctx, network, addr)
	}
	rootCAMu     sync.RWMutex
	extraRootCAs *x509.CertPool
)

func ResolveAllIPs(ctx context.Context, host, dotServer string) ([]net.IP, error) {
	return resolveAllIPsFn(ctx, host, dotServer)
}

func resolveAllIPs(ctx context.Context, host, dotServer string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	return util.WithGeoDNSResolver(dotServer, func() ([]net.IP, error) {
		return util.LookupHostForGeo(ctx, host)
	})
}

func SetExtraRootCAsForTest(pool *x509.CertPool) func() {
	rootCAMu.Lock()
	prev := extraRootCAs
	extraRootCAs = pool
	rootCAMu.Unlock()
	return func() {
		rootCAMu.Lock()
		extraRootCAs = prev
		rootCAMu.Unlock()
	}
}

func buildDialer(opts Options) *net.Dialer {
	d := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if opts.LocalIP != nil {
		d.LocalAddr = &net.TCPAddr{IP: opts.LocalIP}
	}
	return d
}

func buildTLSConfig(opts Options) *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if opts.PinHost != "" {
		cfg.ServerName = opts.PinHost
	}
	rootCAMu.RLock()
	if extraRootCAs != nil {
		cfg.RootCAs = extraRootCAs
	}
	rootCAMu.RUnlock()
	return cfg
}

func NewClient(opts Options) *http.Client {
	dialer := buildDialer(opts)
	transport := &http.Transport{
		TLSClientConfig:     buildTLSConfig(opts),
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		targetAddr := addr
		if opts.PinHost != "" && opts.PinIP != "" {
			host, port, err := net.SplitHostPort(addr)
			if err == nil && normalizeHost(host) == normalizeHost(opts.PinHost) {
				targetAddr = net.JoinHostPort(opts.PinIP, port)
			}
		}
		return dialContextFn(dialer, ctx, network, targetAddr)
	}

	_ = http2.ConfigureTransport(transport)

	return &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
	}
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimSuffix(host, ".")
	return strings.ToLower(host)
}
