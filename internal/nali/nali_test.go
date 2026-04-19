package nali

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

func TestAnnotateLineIPv4AndCache(t *testing.T) {
	calls := 0
	a := New(Config{
		Lang: "en",
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			calls++
			if ip != "8.8.8.8" {
				t.Fatalf("lookup ip = %q, want 8.8.8.8", ip)
			}
			return &ipgeo.IPGeoData{
				Asnumber:  "15169",
				Country:   "美国",
				CountryEn: "United States",
				ProvEn:    "California",
				CityEn:    "Mountain View",
				Owner:     "Google",
			}, nil
		},
	})

	got := a.AnnotateLine(context.Background(), "dns 8.8.8.8 and 8.8.8.8.")
	want := "dns 8.8.8.8 [AS15169, United States, California, Mountain View, Google] and 8.8.8.8 [AS15169, United States, California, Mountain View, Google]."
	if got != want {
		t.Fatalf("AnnotateLine() = %q, want %q", got, want)
	}
	if calls != 1 {
		t.Fatalf("lookup calls = %d, want 1", calls)
	}
}

func TestAnnotateLineIPv6AndMappedIPv6(t *testing.T) {
	a := New(Config{
		Lang: "cn",
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			switch ip {
			case "2606:4700:4700::1111":
				return &ipgeo.IPGeoData{Asnumber: "13335", Country: "美国", Owner: "Cloudflare"}, nil
			case "::ffff:104.26.11.119":
				return &ipgeo.IPGeoData{Asnumber: "13335", Country: "美国", Owner: "Cloudflare"}, nil
			default:
				t.Fatalf("unexpected lookup ip %q", ip)
			}
			return nil, nil
		},
	})

	got := a.AnnotateLine(context.Background(), "v6 2606:4700:4700::1111 mapped ::ffff:104.26.11.119")
	want := "v6 2606:4700:4700::1111 [AS13335, 美国, Cloudflare] mapped ::ffff:104.26.11.119 [AS13335, 美国, Cloudflare]"
	if got != want {
		t.Fatalf("AnnotateLine() = %q, want %q", got, want)
	}
}

func TestAnnotateLineBracketPortCIDRZoneAndPunctuation(t *testing.T) {
	a := New(Config{
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			if ip != "8.8.8.8" {
				t.Fatalf("unexpected lookup ip %q", ip)
			}
			return &ipgeo.IPGeoData{Asnumber: "15169", CountryEn: "United States", Owner: "Google"}, nil
		},
		Lang: "en",
	})

	got := a.AnnotateLine(context.Background(), "https://[2001:db8::1]:443 8.8.8.8:53 192.0.2.1/24 fe80::1%en0.")
	want := "https://[2001:db8::1] [RFC3849]:443 8.8.8.8 [AS15169, United States, Google]:53 192.0.2.1 [RFC5737]/24 fe80::1%en0 [RFC4291]."
	if got != want {
		t.Fatalf("AnnotateLine() = %q, want %q", got, want)
	}
}

func TestAnnotateLineIPv4MappedIPv6WithPortStaysPlain(t *testing.T) {
	a := New(Config{
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			t.Fatalf("unexpected lookup for invalid unbracketed IPv6 endpoint %q", ip)
			return nil, nil
		},
	})

	// parseCandidate only strips ports from pure IPv4 tokens through splitIPv4Port;
	// parseAddr rejects unbracketed IPv6 endpoints with a port suffix.
	got := a.AnnotateLine(context.Background(), "::ffff:1.2.3.4:53")
	if got != "::ffff:1.2.3.4:53" {
		t.Fatalf("AnnotateLine() = %q, want original", got)
	}
}

func TestAnnotateLineFamilyFilters(t *testing.T) {
	source := func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{CountryEn: "ok"}, nil
	}

	got4 := New(Config{Family: Family4, Source: source, Lang: "en"}).AnnotateLine(context.Background(), "8.8.8.8 2606:4700:4700::1111")
	if got4 != "8.8.8.8 [ok] 2606:4700:4700::1111" {
		t.Fatalf("Family4 AnnotateLine() = %q", got4)
	}

	got6 := New(Config{Family: Family6, Source: source, Lang: "en"}).AnnotateLine(context.Background(), "8.8.8.8 2606:4700:4700::1111")
	if got6 != "8.8.8.8 2606:4700:4700::1111 [ok]" {
		t.Fatalf("Family6 AnnotateLine() = %q", got6)
	}
}

func TestAnnotateLineFailureAndEmptyGeoKeepOriginal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source ipgeo.Source
	}{
		{
			name: "error",
			source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
				return nil, errors.New("boom")
			},
		},
		{
			name: "empty",
			source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
				return &ipgeo.IPGeoData{}, nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := New(Config{Source: tc.source}).AnnotateLine(context.Background(), "A 8.8.8.8")
			if got != "A 8.8.8.8" {
				t.Fatalf("AnnotateLine() = %q, want original", got)
			}
		})
	}
}

func TestAnnotateLineDoesNotCacheLookupErrors(t *testing.T) {
	calls := 0
	a := New(Config{
		Lang: "en",
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("temporary")
			}
			return &ipgeo.IPGeoData{CountryEn: "ok"}, nil
		},
	})

	if got := a.AnnotateLine(context.Background(), "A 8.8.8.8"); got != "A 8.8.8.8" {
		t.Fatalf("first AnnotateLine() = %q, want original", got)
	}
	if got := a.AnnotateLine(context.Background(), "A 8.8.8.8"); got != "A 8.8.8.8 [ok]" {
		t.Fatalf("second AnnotateLine() = %q, want annotated", got)
	}
	if calls != 2 {
		t.Fatalf("lookup calls = %d, want 2", calls)
	}
}

func TestFindIPSpans(t *testing.T) {
	line := "IP:1.1.1.1 [2001:db8::1]:443"
	spans := FindIPSpans(line)
	if len(spans) != 2 {
		t.Fatalf("len(spans) = %d, want 2: %+v", len(spans), spans)
	}
	if spans[0].Text != "1.1.1.1" || spans[0].InsertEnd != strings.Index(line, " [") {
		t.Fatalf("unexpected first span: %+v", spans[0])
	}
	if spans[1].Text != "2001:db8::1" || line[spans[1].InsertEnd-1] != ']' {
		t.Fatalf("unexpected bracket span: %+v", spans[1])
	}
}

func TestFindIPSpansRespectsLeftBoundary(t *testing.T) {
	a := New(Config{
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{CountryEn: "ok"}, nil
		},
		Lang: "en",
	})

	got := a.AnnotateLine(context.Background(), "src:2001:db8::1 embeddedx8.8.8.8 dst=8.8.4.4")
	want := "src:2001:db8::1 [RFC3849] embeddedx8.8.8.8 dst=8.8.4.4 [ok]"
	if got != want {
		t.Fatalf("AnnotateLine() = %q, want %q", got, want)
	}
}

func TestFormatGeo(t *testing.T) {
	if got := FormatGeo(&ipgeo.IPGeoData{Whois: "RFC5737"}, "en"); got != "RFC5737" {
		t.Fatalf("FormatGeo(whois) = %q", got)
	}
	if got := FormatGeo(&ipgeo.IPGeoData{Asnumber: "AS65000", CountryEn: "Test", Isp: "Example ISP"}, "en"); got != "AS65000, Test, Example ISP" {
		t.Fatalf("FormatGeo(fields) = %q", got)
	}
	if got := FormatGeo(&ipgeo.IPGeoData{Asnumber: "as 15169", CountryEn: "Test"}, "en"); got != "AS15169, Test" {
		t.Fatalf("FormatGeo(normalized ASN) = %q", got)
	}
	if got := FormatGeo(&ipgeo.IPGeoData{}, "cn"); got != "" {
		t.Fatalf("FormatGeo(empty) = %q, want empty", got)
	}
}

func TestCacheIsBounded(t *testing.T) {
	a := New(Config{
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{CountryEn: "Test"}, nil
		},
		Lang: "en",
	})

	for i := 0; i < maxCacheEntries+10; i++ {
		a.lookupLabel(context.Background(), "8.8."+strconv.Itoa(i/256)+"."+strconv.Itoa(i%256))
	}

	a.cacheMu.RLock()
	defer a.cacheMu.RUnlock()
	if len(a.cache) != maxCacheEntries {
		t.Fatalf("cache size = %d, want %d", len(a.cache), maxCacheEntries)
	}
	if len(a.cacheRing) != maxCacheEntries {
		t.Fatalf("cache ring size = %d, want %d", len(a.cacheRing), maxCacheEntries)
	}
	if _, ok := a.cache["8.8.0.0"]; ok {
		t.Fatal("oldest cache entry was not evicted")
	}
	if _, ok := a.cache["8.8.16.9"]; !ok {
		t.Fatal("newest cache entry missing")
	}
}

func TestRunStdinPreservesNewlineAndStopsOnExit(t *testing.T) {
	var out bytes.Buffer
	cfg := Config{
		Lang: "en",
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{CountryEn: "United States"}, nil
		},
	}
	err := Run(context.Background(), cfg, strings.NewReader("A 8.8.8.8\nquit\nB 8.8.4.4\n"), &out, "")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := out.String(), "A 8.8.8.8 [United States]\n"; got != want {
		t.Fatalf("Run() output = %q, want %q", got, want)
	}
}

func TestRunStdinReturnsOnContextCancelWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Config{}, reader, io.Discard, "")
	}()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

func TestRunTargetAddsNewline(t *testing.T) {
	var out bytes.Buffer
	cfg := Config{
		Lang: "en",
		Source: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{CountryEn: "United States"}, nil
		},
	}
	if err := Run(context.Background(), cfg, nil, &out, "8.8.8.8"); err != nil {
		t.Fatalf("Run(target) error = %v", err)
	}
	if got, want := out.String(), "8.8.8.8 [United States]\n"; got != want {
		t.Fatalf("Run(target) output = %q, want %q", got, want)
	}
}
