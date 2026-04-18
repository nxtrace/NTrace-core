package netx

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"testing"
	"time"
)

type stubConn struct{ net.Conn }

func TestResolveAllIPsUsesInjectedResolver(t *testing.T) {
	prev := resolveAllIPsFn
	resolveAllIPsFn = func(ctx context.Context, host, dotServer string) ([]net.IP, error) {
		if host != "example.com" || dotServer != "aliyun" {
			t.Fatalf("unexpected resolver args host=%q dot=%q", host, dotServer)
		}
		return []net.IP{net.ParseIP("1.1.1.1")}, nil
	}
	defer func() { resolveAllIPsFn = prev }()

	ips, err := ResolveAllIPs(context.Background(), "example.com", "aliyun")
	if err != nil {
		t.Fatalf("ResolveAllIPs() error = %v", err)
	}
	if len(ips) != 1 || ips[0].String() != "1.1.1.1" {
		t.Fatalf("ResolveAllIPs() = %v, want [1.1.1.1]", ips)
	}
}

func TestBuildDialerSetsLocalAddress(t *testing.T) {
	dialer := buildDialer(Options{LocalIP: net.ParseIP("127.0.0.1")})
	if dialer.LocalAddr == nil {
		t.Fatal("LocalAddr = nil, want non-nil")
	}
	addr, ok := dialer.LocalAddr.(*net.TCPAddr)
	if !ok || !addr.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("LocalAddr = %#v, want TCPAddr{127.0.0.1}", dialer.LocalAddr)
	}
}

func TestNewClientPinsDialTarget(t *testing.T) {
	prev := dialContextFn
	defer func() { dialContextFn = prev }()

	var gotAddr string
	dialContextFn = func(d *net.Dialer, ctx context.Context, network, addr string) (net.Conn, error) {
		gotAddr = addr
		server, client := net.Pipe()
		go server.Close()
		return client, nil
	}

	client := NewClient(Options{
		PinHost: "example.com",
		PinIP:   "203.0.113.9",
		Timeout: time.Second,
	})
	tr := client.Transport.(*http.Transport)
	conn, err := tr.DialContext(context.Background(), "tcp", "example.com:443")
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	_ = conn.Close()
	if gotAddr != "203.0.113.9:443" {
		t.Fatalf("dial target = %q, want 203.0.113.9:443", gotAddr)
	}
}

func TestBuildTLSConfigUsesPinnedHostAndInjectedRootCAs(t *testing.T) {
	pool := x509.NewCertPool()
	restore := SetExtraRootCAsForTest(pool)
	defer restore()

	cfg := buildTLSConfig(Options{PinHost: "speed.cloudflare.com"})
	if cfg.ServerName != "speed.cloudflare.com" {
		t.Fatalf("ServerName = %q, want speed.cloudflare.com", cfg.ServerName)
	}
	if cfg.RootCAs != pool {
		t.Fatal("RootCAs not applied from test override")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %v, want TLS1.2", cfg.MinVersion)
	}
}
