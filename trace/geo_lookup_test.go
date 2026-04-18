package trace

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

func TestLookupIPGeoCachesResults(t *testing.T) {
	ClearCaches()
	t.Cleanup(ClearCaches)

	var calls atomic.Int32
	source := func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
		calls.Add(1)
		return &ipgeo.IPGeoData{
			IP:        ip,
			Asnumber:  "13335",
			CountryEn: "Canada",
			ProvEn:    "Ontario",
			CityEn:    "Toronto",
			Owner:     "Cloudflare, Inc.",
		}, nil
	}

	for i := 0; i < 2; i++ {
		geo, err := LookupIPGeo(context.Background(), source, "en", false, 3, "1.1.1.1")
		if err != nil {
			t.Fatalf("LookupIPGeo() error = %v", err)
		}
		if geo == nil || geo.Asnumber != "13335" {
			t.Fatalf("LookupIPGeo() geo = %+v, want ASN 13335", geo)
		}
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("geo source calls = %d, want 1 due to shared cache", got)
	}
}

func TestLookupIPGeoRejectsNonIPTargets(t *testing.T) {
	source := func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
		t.Fatal("geo source should not be called for non-IP target")
		return nil, nil
	}

	if _, err := LookupIPGeo(context.Background(), source, "en", false, 3, "example.com"); err == nil {
		t.Fatal("LookupIPGeo(non-ip) error = nil, want error")
	}
}
