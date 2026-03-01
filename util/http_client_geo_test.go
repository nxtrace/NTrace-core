package util

import (
	"net/http"
	"testing"
	"time"
)

func TestNewGeoHTTPClient_ReturnsValidClient(t *testing.T) {
	c := NewGeoHTTPClient(3 * time.Second)
	if c == nil {
		t.Fatal("NewGeoHTTPClient returned nil")
	}
	if c.Timeout != 3*time.Second {
		t.Errorf("Timeout = %v, want 3s", c.Timeout)
	}
}

func TestNewGeoHTTPClient_HasCustomTransport(t *testing.T) {
	c := NewGeoHTTPClient(2 * time.Second)
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatal("Transport is not *http.Transport or is nil")
	}
	if tr.DialContext == nil {
		t.Error("Transport.DialContext is nil, expected custom function")
	}
}

func TestNewGeoHTTPClient_DifferentTimeouts(t *testing.T) {
	for _, d := range []time.Duration{
		500 * time.Millisecond,
		2 * time.Second,
		10 * time.Second,
	} {
		c := NewGeoHTTPClient(d)
		if c.Timeout != d {
			t.Errorf("NewGeoHTTPClient(%v).Timeout = %v", d, c.Timeout)
		}
	}
}
