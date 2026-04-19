package runner

import (
	"context"
	"fmt"
	"strings"

	speedconfig "github.com/nxtrace/NTrace-core/internal/speedtest/config"
	"github.com/nxtrace/NTrace-core/internal/speedtest/result"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

const (
	defaultSpeedGeoSourceProvider   = "LeoMoeAPI"
	defaultSpeedGeoLookupRetryCount = 3
)

var (
	fetchIPDescFn   = fetchIPDescription
	fetchPeerInfoFn = fetchPeerInfo
	lookupGeoDataFn = lookupGeoData
)

func fetchIPDescription(ctx context.Context, ip string, cfg *speedconfig.Config) string {
	geo, err := lookupGeoDataFn(ctx, ip, cfg)
	if err != nil {
		return localizedText(cfg, "lookup failed", "查询失败")
	}
	desc := formatGeoDescription(ip, geo, cfg.Language)
	if strings.TrimSpace(desc) == "" {
		return localizedText(cfg, "unknown location", "未知位置")
	}
	return desc
}

func fetchPeerInfo(ctx context.Context, target string, cfg *speedconfig.Config) result.PeerInfo {
	target = strings.TrimSpace(target)
	if target == "" {
		return result.PeerInfo{Status: "unavailable"}
	}

	geo, err := lookupGeoDataFn(ctx, target, cfg)
	if err != nil {
		return result.PeerInfo{Status: "unavailable"}
	}

	peer := result.PeerInfo{
		Status:   "ok",
		IP:       firstNonEmpty(strings.TrimSpace(geo.IP), target),
		ISP:      ownerOrISP(geo),
		ASN:      normalizeASN(geo.Asnumber),
		Location: formatGeoLocation(geo, cfg.Language),
	}
	if peer.Location == "" {
		peer.Location = localizedText(cfg, "unknown", "未知")
	}
	return peer
}

func lookupGeoData(ctx context.Context, target string, cfg *speedconfig.Config) (*ipgeo.IPGeoData, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil speed config")
	}
	source := ipgeo.GetSourceWithGeoDNS(defaultSpeedGeoSourceProvider, cfg.DotServer)
	return trace.LookupIPGeo(ctx, source, cfg.Language, false, defaultSpeedGeoLookupRetryCount, target)
}

func formatGeoDescription(ip string, geo *ipgeo.IPGeoData, lang string) string {
	if geo == nil || isEmptyGeo(geo) {
		return ""
	}
	return strings.TrimSpace(printer.FormatIPGeoData(ip, localizedGeoCopy(geo, lang)))
}

func formatGeoLocation(geo *ipgeo.IPGeoData, lang string) string {
	if geo == nil {
		return ""
	}
	localized := localizedGeoCopy(geo, lang)
	parts := make([]string, 0, 3)
	parts = appendUniqueLocationPart(parts, localized.City)
	parts = appendUniqueLocationPart(parts, localized.Prov)
	parts = appendUniqueLocationPart(parts, localized.Country)
	return strings.Join(parts, ", ")
}

func appendUniqueLocationPart(parts []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return parts
	}
	for _, part := range parts {
		if part == value {
			return parts
		}
	}
	return append(parts, value)
}

func localizedGeoCopy(geo *ipgeo.IPGeoData, lang string) *ipgeo.IPGeoData {
	if geo == nil {
		return nil
	}
	cp := *geo
	if strings.EqualFold(lang, "en") {
		cp.Country = firstNonEmpty(strings.TrimSpace(geo.CountryEn), strings.TrimSpace(geo.Country))
		cp.Prov = firstNonEmpty(strings.TrimSpace(geo.ProvEn), strings.TrimSpace(geo.Prov))
		cp.City = firstNonEmpty(strings.TrimSpace(geo.CityEn), strings.TrimSpace(geo.City))
		return &cp
	}
	cp.Country = firstNonEmpty(strings.TrimSpace(geo.Country), strings.TrimSpace(geo.CountryEn))
	cp.Prov = firstNonEmpty(strings.TrimSpace(geo.Prov), strings.TrimSpace(geo.ProvEn))
	cp.City = firstNonEmpty(strings.TrimSpace(geo.City), strings.TrimSpace(geo.CityEn))
	return &cp
}

func normalizeASN(asn string) string {
	asn = strings.TrimSpace(asn)
	if asn == "" {
		return ""
	}
	upper := strings.ToUpper(asn)
	if strings.HasPrefix(upper, "AS") {
		asn = strings.TrimSpace(asn[2:])
	}
	return "AS" + asn
}

func ownerOrISP(geo *ipgeo.IPGeoData) string {
	if geo == nil {
		return ""
	}
	if owner := strings.TrimSpace(geo.Owner); owner != "" {
		return owner
	}
	return strings.TrimSpace(geo.Isp)
}

func isEmptyGeo(geo *ipgeo.IPGeoData) bool {
	if geo == nil {
		return true
	}
	return strings.TrimSpace(geo.Asnumber) == "" &&
		strings.TrimSpace(geo.Country) == "" &&
		strings.TrimSpace(geo.CountryEn) == "" &&
		strings.TrimSpace(geo.Prov) == "" &&
		strings.TrimSpace(geo.ProvEn) == "" &&
		strings.TrimSpace(geo.City) == "" &&
		strings.TrimSpace(geo.CityEn) == "" &&
		strings.TrimSpace(geo.District) == "" &&
		strings.TrimSpace(geo.Owner) == "" &&
		strings.TrimSpace(geo.Isp) == "" &&
		strings.TrimSpace(geo.Whois) == ""
}

func localizedText(cfg *speedconfig.Config, en, zh string) string {
	if cfg == nil {
		return en
	}
	if strings.EqualFold(cfg.Language, "en") {
		return en
	}
	return zh
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
