package runner

import (
	"context"
	"strings"
	"sync"
	"testing"

	speedconfig "github.com/nxtrace/NTrace-core/internal/speedtest/config"
	"github.com/nxtrace/NTrace-core/ipgeo"
)

var geoLookupMu sync.Mutex

func TestFetchIPDescriptionUsesGeoData(t *testing.T) {
	geoLookupMu.Lock()
	defer geoLookupMu.Unlock()

	prev := lookupGeoDataFn
	lookupGeoDataFn = func(ctx context.Context, target string, cfg *speedconfig.Config) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{
			Asnumber:  "13335",
			CountryEn: "Canada",
			ProvEn:    "Ontario",
			CityEn:    "Toronto",
			Owner:     "Cloudflare, Inc.",
		}, nil
	}
	defer func() { lookupGeoDataFn = prev }()

	got := fetchIPDescription(context.Background(), "172.66.0.218", &speedconfig.Config{Language: "en"})
	for _, want := range []string{"AS13335", "Canada", "Ontario", "Toronto", "Cloudflare, Inc."} {
		if !strings.Contains(got, want) {
			t.Fatalf("fetchIPDescription() = %q, want substring %q", got, want)
		}
	}
}

func TestFetchPeerInfoUsesLocalizedGeoData(t *testing.T) {
	geoLookupMu.Lock()
	defer geoLookupMu.Unlock()

	prev := lookupGeoDataFn
	lookupGeoDataFn = func(ctx context.Context, target string, cfg *speedconfig.Config) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{
			IP:       "198.51.100.10",
			Asnumber: "13335",
			Country:  "加拿大",
			Prov:     "安大略省",
			City:     "多伦多",
			Owner:    "Cloudflare, Inc.",
		}, nil
	}
	defer func() { lookupGeoDataFn = prev }()

	got := fetchPeerInfo(context.Background(), "198.51.100.10", &speedconfig.Config{Language: "cn"})
	if got.Status != "ok" {
		t.Fatalf("Status = %q, want ok", got.Status)
	}
	if got.ASN != "AS13335" {
		t.Fatalf("ASN = %q, want AS13335", got.ASN)
	}
	if got.ISP != "Cloudflare, Inc." {
		t.Fatalf("ISP = %q, want Cloudflare, Inc.", got.ISP)
	}
	// Location order is intentionally language-specific.
	if got.Location != "多伦多, 安大略省, 加拿大" {
		t.Fatalf("Location = %q, want localized location", got.Location)
	}
}

func TestFormatGeoLocationDeduplicatesRepeatedParts(t *testing.T) {
	got := formatGeoLocation(&ipgeo.IPGeoData{
		CountryEn: "Singapore",
		CityEn:    "Singapore",
	}, "en")
	if got != "Singapore" {
		t.Fatalf("formatGeoLocation() = %q, want Singapore", got)
	}
}

func TestFetchPeerInfoWithoutTargetReturnsUnavailable(t *testing.T) {
	got := fetchPeerInfo(context.Background(), "", &speedconfig.Config{Language: "en"})
	if got.Status != "unavailable" {
		t.Fatalf("Status = %q, want unavailable", got.Status)
	}
}
