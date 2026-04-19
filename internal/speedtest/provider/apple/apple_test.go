package apple

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestAppleRequestsUseExpectedDefaults(t *testing.T) {
	p := New()
	idle, err := p.IdleLatencyRequest(context.Background())
	if err != nil {
		t.Fatalf("IdleLatencyRequest() error = %v", err)
	}
	if idle.Method != http.MethodGet {
		t.Fatalf("idle.Method = %q, want GET", idle.Method)
	}
	if idle.URL != defaultLatencyURL {
		t.Fatalf("idle.URL = %q, want %q", idle.URL, defaultLatencyURL)
	}

	down, err := p.DownloadRequest(context.Background(), 123)
	if err != nil {
		t.Fatalf("DownloadRequest() error = %v", err)
	}
	if down.URL != defaultDownloadURL {
		t.Fatalf("down.URL = %q, want %q", down.URL, defaultDownloadURL)
	}
	if down.ResponseLimit != 123 {
		t.Fatalf("down.ResponseLimit = %d, want 123", down.ResponseLimit)
	}

	up, err := p.UploadRequest(context.Background(), 123)
	if err != nil {
		t.Fatalf("UploadRequest() error = %v", err)
	}
	if up.Method != http.MethodPut {
		t.Fatalf("up.Method = %q, want PUT", up.Method)
	}
	if up.URL != defaultUploadURL {
		t.Fatalf("up.URL = %q, want %q", up.URL, defaultUploadURL)
	}
	if up.Headers["Upload-Draft-Interop-Version"] != "6" {
		t.Fatalf("upload header missing Upload-Draft-Interop-Version: %#v", up.Headers)
	}
	if up.Headers["Upload-Complete"] != "?1" {
		t.Fatalf("upload header missing Upload-Complete: %#v", up.Headers)
	}
}

func TestAppleParseMetadata(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Via", "edge")
	resp.Header.Set("X-Cache", "hit")
	resp.Header.Set("CDNUUID", "uuid")

	meta := New().ParseMetadata(resp, nil)
	if meta["via"] != "edge" || meta["x_cache"] != "hit" || meta["cdn_uuid"] != "uuid" {
		t.Fatalf("ParseMetadata() = %#v, want apple response headers", meta)
	}
}

func TestAppleHostUsesLatencyEndpoint(t *testing.T) {
	if host := New().Host(); host == "" || !strings.Contains(defaultDownloadURL, host) {
		t.Fatalf("Host() = %q, want host parsed from %q", host, defaultDownloadURL)
	}
}
