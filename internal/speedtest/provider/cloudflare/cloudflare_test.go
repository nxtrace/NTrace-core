package cloudflare

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
)

func TestCloudflareRequestsCarryExpectedQueryParameters(t *testing.T) {
	p := New("abc123")

	idle, err := p.IdleLatencyRequest(context.Background())
	if err != nil {
		t.Fatalf("IdleLatencyRequest() error = %v", err)
	}
	if !strings.Contains(idle.URL, "/__down?") || !strings.Contains(idle.URL, "bytes=0") || !strings.Contains(idle.URL, "measId=abc123") {
		t.Fatalf("idle.URL = %q, want bytes=0 and measId", idle.URL)
	}

	loaded, err := p.LoadedLatencyRequest(context.Background(), provider.LatencyLoadDownload)
	if err != nil {
		t.Fatalf("LoadedLatencyRequest() error = %v", err)
	}
	if !strings.Contains(loaded.URL, "during=download") {
		t.Fatalf("loaded.URL = %q, want during=download", loaded.URL)
	}

	down, err := p.DownloadRequest(context.Background(), 2048)
	if err != nil {
		t.Fatalf("DownloadRequest() error = %v", err)
	}
	if !strings.Contains(down.URL, "bytes=2048") {
		t.Fatalf("down.URL = %q, want bytes=2048", down.URL)
	}

	up, err := p.UploadRequest(context.Background(), 2048)
	if err != nil {
		t.Fatalf("UploadRequest() error = %v", err)
	}
	if up.Method != http.MethodPost {
		t.Fatalf("up.Method = %q, want POST", up.Method)
	}
	if up.ContentLength != 2048 {
		t.Fatalf("up.ContentLength = %d, want 2048", up.ContentLength)
	}
	if !strings.Contains(up.URL, "/__up") || !strings.Contains(up.URL, "measId=abc123") {
		t.Fatalf("up.URL = %q, want __up and measId", up.URL)
	}
}

func TestCloudflareParseMetadataPrefersHeadersAndFallsBackToBody(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("cf-meta-ip", "198.51.100.10")
	resp.Header.Set("cf-meta-colo", "HKG")
	resp.Header.Set("cf-meta-asn", "13335")
	resp.Header.Set("cf-ray", "abcd-HKG")

	meta := New("x").ParseMetadata(resp, []byte("ip=203.0.113.10\ncolo=SIN\nloc=SG\ntls=TLSv1.3\n"))
	if meta["client_ip"] != "198.51.100.10" {
		t.Fatalf("client_ip = %#v, want cf-meta-ip", meta["client_ip"])
	}
	if meta["colo"] != "HKG" {
		t.Fatalf("colo = %#v, want cf-meta-colo", meta["colo"])
	}
	if meta["asn"] != "13335" {
		t.Fatalf("asn = %#v, want 13335", meta["asn"])
	}
	if meta["tls_version"] != "TLSv1.3" {
		t.Fatalf("tls_version = %#v, want body fallback", meta["tls_version"])
	}
}

func TestCloudflareHostUsesBaseURL(t *testing.T) {
	if host := New("x").Host(); host == "" || !strings.Contains(defaultBaseURL, host) {
		t.Fatalf("Host() = %q, want host parsed from %q", host, defaultBaseURL)
	}
}
