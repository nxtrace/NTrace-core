package mtu

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

func TestEnrichHopMetadataGeoSuccess(t *testing.T) {
	cfg := Config{
		IPGeoSource: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{
				Asnumber:  "13335",
				Country:   "中国香港",
				CountryEn: "Hong Kong",
				Owner:     "Cloudflare",
			}, nil
		},
		Lang: "cn",
	}

	hop, changed := enrichHopMetadata(context.Background(), cfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "1.1.1.1"})
	if !changed {
		t.Fatal("expected hop metadata to change")
	}
	if hop.Geo == nil || hop.Geo.Asnumber != "13335" {
		t.Fatalf("unexpected geo: %+v", hop.Geo)
	}
}

func TestEnrichHopMetadataDisableGeoIPReturnsNoGeo(t *testing.T) {
	cfg := Config{
		IPGeoSource: ipgeo.GetSource("disable-geoip"),
	}

	hop, changed := enrichHopMetadata(context.Background(), cfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "1.1.1.1"})
	if changed {
		t.Fatalf("expected no metadata change, got %+v", hop)
	}
	if hop.Geo != nil {
		t.Fatalf("expected nil geo, got %+v", hop.Geo)
	}
}

func TestEnrichHopMetadataRDNSOnly(t *testing.T) {
	origLookup := mtuLookupAddr
	mtuLookupAddr = func(context.Context, string) ([]string, error) {
		return []string{"one.one.one.one."}, nil
	}
	defer func() { mtuLookupAddr = origLookup }()

	cfg := Config{
		RDNS:           true,
		AlwaysWaitRDNS: true,
	}

	hop, changed := enrichHopMetadata(context.Background(), cfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "1.1.1.1"})
	if !changed {
		t.Fatal("expected hostname metadata change")
	}
	if hop.Hostname != "one.one.one.one" {
		t.Fatalf("hostname = %q, want %q", hop.Hostname, "one.one.one.one")
	}
}

func TestEnrichHopMetadataRDNSOnlyWithoutAlwaysWaitStillSetsHostname(t *testing.T) {
	origLookup := mtuLookupAddr
	mtuLookupAddr = func(context.Context, string) ([]string, error) {
		return []string{"resolver.example.com."}, nil
	}
	defer func() { mtuLookupAddr = origLookup }()

	cfg := Config{
		RDNS: true,
	}

	hop, changed := enrichHopMetadata(context.Background(), cfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "1.1.1.1"})
	if !changed {
		t.Fatal("expected hostname metadata change")
	}
	if hop.Hostname != "resolver.example.com" {
		t.Fatalf("hostname = %q, want %q", hop.Hostname, "resolver.example.com")
	}
}

func TestEnrichHopMetadataAlwaysWaitRDNSWaitsForPTR(t *testing.T) {
	origLookup := mtuLookupAddr
	mtuLookupAddr = func(context.Context, string) ([]string, error) {
		time.Sleep(20 * time.Millisecond)
		return []string{"resolver.example.com."}, nil
	}
	defer func() { mtuLookupAddr = origLookup }()

	baseCfg := Config{
		RDNS: true,
		IPGeoSource: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{CountryEn: "US"}, nil
		},
	}

	hopNoWait, _ := enrichHopMetadata(context.Background(), baseCfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "8.8.8.8"})
	if hopNoWait.Hostname != "" {
		t.Fatalf("expected no hostname without AlwaysWaitRDNS, got %q", hopNoWait.Hostname)
	}

	baseCfg.AlwaysWaitRDNS = true
	hopWait, _ := enrichHopMetadata(context.Background(), baseCfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "8.8.8.8"})
	if hopWait.Hostname != "resolver.example.com" {
		t.Fatalf("hostname = %q, want %q", hopWait.Hostname, "resolver.example.com")
	}
	if hopWait.Geo == nil || hopWait.Geo.CountryEn != "US" {
		t.Fatalf("unexpected geo with AlwaysWaitRDNS: %+v", hopWait.Geo)
	}
}

func TestEnrichHopMetadataGeoTimeout(t *testing.T) {
	cfg := Config{
		IPGeoSource: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return nil, errors.New("boom")
		},
	}

	hop, changed := enrichHopMetadata(context.Background(), cfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "1.1.1.1"})
	if !changed {
		t.Fatal("expected timeout geo metadata change")
	}
	if hop.Geo == nil || hop.Geo.Source != mtuTimeoutGeoSource {
		t.Fatalf("unexpected timeout geo: %+v", hop.Geo)
	}
}

func TestEnrichHopMetadataCancelStopsWaitingForPTR(t *testing.T) {
	origLookup := mtuLookupAddr
	blocked := make(chan struct{})
	mtuLookupAddr = func(ctx context.Context, ip string) ([]string, error) {
		close(blocked)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	defer func() { mtuLookupAddr = origLookup }()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		RDNS: true,
		IPGeoSource: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			time.Sleep(200 * time.Millisecond)
			return &ipgeo.IPGeoData{CountryEn: "US"}, nil
		},
	}

	done := make(chan struct{})
	var (
		hop     Hop
		changed bool
	)
	go func() {
		hop, changed = enrichHopMetadata(ctx, cfg, Hop{TTL: 1, Event: EventTimeExceeded, IP: "1.1.1.1"})
		close(done)
	}()

	<-blocked
	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("enrichHopMetadata did not stop promptly after cancel")
	}
	if changed {
		t.Fatalf("expected no metadata change after cancel, got %+v", hop)
	}
}
