package trace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

var (
	ErrEmptyGeoQuery = errors.New("empty geo lookup target")
	ErrNilGeoSource  = errors.New("nil geo source")
	ErrNotIPGeoQuery = errors.New("geo lookup target is not an IP address")
)

// LookupIPGeo reuses traceroute's shared GeoIP cache/retry path for direct IP metadata lookups.
func LookupIPGeo(ctx context.Context, source ipgeo.Source, lang string, maptrace bool, numMeasurements int, query string) (*ipgeo.IPGeoData, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyGeoQuery
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, ErrNilGeoSource
	}
	if ip := net.ParseIP(query); ip == nil {
		return nil, fmt.Errorf("%w: %q", ErrNotIPGeoQuery, query)
	}
	if geo, ok := ipgeo.Filter(query); ok {
		geoCache.Store(query, geo)
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
