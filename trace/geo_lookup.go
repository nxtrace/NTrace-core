package trace

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

// LookupIPGeo reuses traceroute's shared GeoIP cache/retry path for direct IP metadata lookups.
func LookupIPGeo(ctx context.Context, source ipgeo.Source, lang string, maptrace bool, numMeasurements int, query string) (*ipgeo.IPGeoData, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty geo lookup target")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("nil geo source")
	}
	if ip := net.ParseIP(query); ip == nil {
		return nil, fmt.Errorf("geo lookup target %q is not an IP address", query)
	}
	if geo, ok := ipgeo.Filter(query); ok {
		return geo, nil
	}
	return lookupGeoWithRetry(Config{
		Context:         ctx,
		IPGeoSource:     source,
		Lang:            lang,
		Maptrace:        maptrace,
		NumMeasurements: numMeasurements,
	}, query, query, false)
}
