package runner

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	speedconfig "github.com/nxtrace/NTrace-core/internal/speedtest/config"
	"github.com/nxtrace/NTrace-core/internal/speedtest/netx"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/apple"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/cloudflare"
	"github.com/nxtrace/NTrace-core/internal/speedtest/result"
)

func TestResolveLocalBindIPPrefersExplicitSource(t *testing.T) {
	ip, err := resolveLocalBindIP(net.ParseIP("1.1.1.1"), "127.0.0.1", "missing0")
	if err != nil {
		t.Fatalf("resolveLocalBindIP() error = %v", err)
	}
	if !ip.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("resolveLocalBindIP() = %v, want 127.0.0.1", ip)
	}
}

func TestRunAppleWithEndpoint(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/gm/small":
			time.Sleep(10 * time.Millisecond)
			_, _ = io.WriteString(w, "1")
		case "/api/v1/gm/large":
			time.Sleep(20 * time.Millisecond)
			_, _ = io.Copy(w, io.LimitReader(zeroReader{}, 64<<10))
		case "/api/v1/gm/slurp":
			time.Sleep(20 * time.Millisecond)
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restoreURLs := apple.SetBaseForTest(srv.URL)
	defer restoreURLs()
	restoreRoots := netx.SetExtraRootCAsForTest(srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs)
	defer restoreRoots()

	u, _ := url.Parse(srv.URL)
	host := u.Hostname()
	res := Run(context.Background(), &speedconfig.Config{
		Provider:     "apple",
		Max:          "64KiB",
		MaxBytes:     64 << 10,
		TimeoutMs:    1500,
		Threads:      2,
		LatencyCount: 2,
		EndpointIP:   host,
		NoMetadata:   true,
		Language:     "en",
		NoColor:      true,
	}, nil, false)

	if res.ExitCode != 0 && res.ExitCode != 2 {
		t.Fatalf("ExitCode = %d, want 0 or 2", res.ExitCode)
	}
	if len(res.Rounds) != 4 {
		t.Fatalf("len(Rounds) = %d, want 4", len(res.Rounds))
	}
	if res.SelectedEndpoint.IP != host {
		t.Fatalf("SelectedEndpoint.IP = %q, want %q", res.SelectedEndpoint.IP, host)
	}
	if res.TotalBytes == 0 {
		t.Fatal("TotalBytes = 0, want > 0")
	}
}

func TestRunCloudflareDiscoveryAndMetadata(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/__down":
			w.Header().Set("cf-meta-ip", "198.51.100.10")
			w.Header().Set("cf-meta-colo", "HKG")
			w.Header().Set("cf-meta-country", "HK")
			w.Header().Set("cf-meta-city", "Hong Kong")
			w.Header().Set("cf-meta-asn", "13335")
			time.Sleep(10 * time.Millisecond)
			if r.URL.Query().Get("bytes") == "0" {
				_, _ = io.WriteString(w, "0")
				return
			}
			_, _ = io.Copy(w, io.LimitReader(zeroReader{}, 64<<10))
		case "/__up":
			time.Sleep(20 * time.Millisecond)
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restoreBase := cloudflare.SetBaseForTest(srv.URL)
	defer restoreBase()
	restoreRoots := netx.SetExtraRootCAsForTest(srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs)
	defer restoreRoots()
	restoreMetadata := overrideMetadataFetchersForTest(t)
	defer restoreMetadata()

	res := Run(context.Background(), &speedconfig.Config{
		Provider:       "cloudflare",
		Max:            "64KiB",
		MaxBytes:       64 << 10,
		TimeoutMs:      1500,
		Threads:        2,
		LatencyCount:   2,
		NonInteractive: true,
		Language:       "en",
		NoColor:        true,
	}, nil, false)

	if res.ExitCode != 0 && res.ExitCode != 2 {
		t.Fatalf("ExitCode = %d, want 0 or 2", res.ExitCode)
	}
	if len(res.Candidates) == 0 {
		t.Fatal("Candidates empty, want at least one discovered endpoint")
	}
	if res.ConnectionInfo.Server.ProviderMeta["colo"] != "HKG" {
		t.Fatalf("server provider meta = %#v, want colo HKG", res.ConnectionInfo.Server.ProviderMeta)
	}
	if res.ConnectionInfo.Client.IP != "198.51.100.10" {
		t.Fatalf("client IP = %q, want 198.51.100.10", res.ConnectionInfo.Client.IP)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func overrideMetadataFetchersForTest(t *testing.T) func() {
	t.Helper()
	prevDesc := fetchIPDescFn
	prevPeer := fetchPeerInfoFn
	fetchIPDescFn = func(ctx context.Context, ip string, cfg *speedconfig.Config) string {
		return "test-desc"
	}
	fetchPeerInfoFn = func(ctx context.Context, target string, cfg *speedconfig.Config) result.PeerInfo {
		switch target {
		case "", "198.51.100.10":
			return result.PeerInfo{Status: "ok", IP: "198.51.100.10", ISP: "TestISP", ASN: "AS64500", Location: "Test City"}
		default:
			return result.PeerInfo{Status: "ok", IP: target, ISP: "EdgeISP", ASN: "AS13335", Location: "Hong Kong"}
		}
	}
	return func() {
		fetchIPDescFn = prevDesc
		fetchPeerInfoFn = prevPeer
	}
}

func TestPerformRequestUsesProviderMetadataParser(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cf-meta-colo", "SIN")
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	restoreRoots := netx.SetExtraRootCAsForTest(srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs)
	defer restoreRoots()

	u, _ := url.Parse(srv.URL)
	client := netx.NewClient(netx.Options{
		PinHost: u.Hostname(),
		PinIP:   u.Hostname(),
		Timeout: time.Second,
	})
	ms, meta, err := performRequest(context.Background(), client, provider.RequestSpec{
		Method: http.MethodGet,
		URL:    srv.URL,
	}, cloudflare.New("m"))
	if err != nil {
		t.Fatalf("performRequest() error = %v", err)
	}
	if ms <= 0 {
		t.Fatalf("ms = %f, want > 0", ms)
	}
	if meta["colo"] != "SIN" {
		t.Fatalf("meta = %#v, want provider parser result", meta)
	}
}
