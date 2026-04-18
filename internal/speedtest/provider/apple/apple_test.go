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
	if idle.URL != DefaultLatencyURL {
		t.Fatalf("idle.URL = %q, want %q", idle.URL, DefaultLatencyURL)
	}

	down, err := p.DownloadRequest(context.Background(), 123)
	if err != nil {
		t.Fatalf("DownloadRequest() error = %v", err)
	}
	if down.URL != DefaultDownloadURL {
		t.Fatalf("down.URL = %q, want %q", down.URL, DefaultDownloadURL)
	}

	up, err := p.UploadRequest(context.Background(), 123)
	if err != nil {
		t.Fatalf("UploadRequest() error = %v", err)
	}
	if up.Method != http.MethodPut {
		t.Fatalf("up.Method = %q, want PUT", up.Method)
	}
	if up.URL != DefaultUploadURL {
		t.Fatalf("up.URL = %q, want %q", up.URL, DefaultUploadURL)
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
	if host := New().Host(); host == "" || !strings.Contains(DefaultLatencyURL, host) {
		t.Fatalf("Host() = %q, want host parsed from %q", host, DefaultLatencyURL)
	}
}
