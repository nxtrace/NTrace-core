//go:build !flavor_tiny && !flavor_ntr

package trace

import (
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"github.com/jsdelivr/globalping-cli/globalping"
	_config "github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/util"
)

func GlobalpingTraceroute(opts *GlobalpingOptions, config *Config) (*Result, *globalping.Measurement, error) {
	client := newGlobalpingClient()
	measurement, err := createGlobalpingMeasurement(client, buildGlobalpingMeasurement(opts))
	if err != nil {
		return nil, nil, err
	}
	gpHops, err := decodeGlobalpingMeasurementHops(measurement)
	if err != nil {
		return nil, measurement, err
	}
	limit := resolveGlobalpingHopLimit(opts, config, len(gpHops))
	return buildGlobalpingResult(gpHops, limit, config), measurement, nil
}

func newGlobalpingClient() globalping.Client {
	cfg := globalping.Config{
		UserAgent: "NextTrace/" + _config.Version,
	}
	if util.GlobalpingToken != "" {
		cfg.AuthToken = &globalping.Token{
			AccessToken: util.GlobalpingToken,
			Expiry:      time.Now().Add(math.MaxInt64),
		}
	}
	return globalping.NewClient(cfg)
}

func buildGlobalpingMeasurement(opts *GlobalpingOptions) *globalping.MeasurementCreate {
	req := &globalping.MeasurementCreate{
		Type:   "mtr",
		Target: opts.Target,
		Limit:  1,
		Locations: []globalping.Locations{{
			Magic: opts.From,
		}},
		Options: &globalping.MeasurementOptions{
			Port:     uint16(opts.Port),
			Packets:  opts.Packets,
			Protocol: globalpingProtocol(opts),
		},
	}
	assignGlobalpingIPVersion(req.Options, opts)
	return req
}

func globalpingProtocol(opts *GlobalpingOptions) string {
	switch {
	case opts.TCP:
		return "TCP"
	case opts.UDP:
		return "UDP"
	default:
		return "ICMP"
	}
}

func assignGlobalpingIPVersion(options *globalping.MeasurementOptions, opts *GlobalpingOptions) {
	switch {
	case opts.IPv4 && !opts.IPv6:
		options.IPVersion = globalping.IPVersion4
	case opts.IPv6 && !opts.IPv4:
		options.IPVersion = globalping.IPVersion6
	}
}

func createGlobalpingMeasurement(client globalping.Client, req *globalping.MeasurementCreate) (*globalping.Measurement, error) {
	res, err := client.CreateMeasurement(req)
	if err != nil {
		return nil, err
	}
	return client.AwaitMeasurement(res.ID)
}

func decodeGlobalpingMeasurementHops(measurement *globalping.Measurement) ([]globalping.MTRHop, error) {
	if measurement.Status != globalping.StatusFinished {
		return nil, fmt.Errorf("measurement did not complete successfully: %s", measurement.Status)
	}
	if len(measurement.Results) == 0 {
		return nil, fmt.Errorf("globalping measurement returned no probe results")
	}
	firstResult := measurement.Results[0]
	if len(firstResult.Result.HopsRaw) == 0 {
		return nil, fmt.Errorf("globalping measurement results did not include hop data")
	}
	return globalping.DecodeMTRHops(firstResult.Result.HopsRaw)
}

func resolveGlobalpingHopLimit(opts *GlobalpingOptions, config *Config, total int) int {
	limit := opts.MaxHops
	if limit <= 0 && config != nil && config.MaxHops > 0 {
		limit = config.MaxHops
	}
	if limit <= 0 || limit > total {
		return total
	}
	return limit
}

func buildGlobalpingResult(gpHops []globalping.MTRHop, limit int, config *Config) *Result {
	result := &Result{}
	geoMap := map[string]*ipgeo.IPGeoData{}
	maxTimings := maxGlobalpingTimings(gpHops, limit)
	for i := 0; i < limit; i++ {
		result.Hops = append(result.Hops, buildGlobalpingTTLHops(i+1, &gpHops[i], maxTimings, geoMap, config))
	}
	return result
}

func maxGlobalpingTimings(gpHops []globalping.MTRHop, limit int) int {
	maxTimings := 1
	for i := 0; i < limit; i++ {
		if count := len(gpHops[i].Timings); count > maxTimings {
			maxTimings = count
		}
	}
	return maxTimings
}

func buildGlobalpingTTLHops(ttl int, gpHop *globalping.MTRHop, maxTimings int, geoMap map[string]*ipgeo.IPGeoData, config *Config) []Hop {
	hops := make([]Hop, 0, maxTimings)
	for j := 0; j < maxTimings; j++ {
		hops = append(hops, mapGlobalpingHop(ttl, gpHop, globalpingTimingAt(gpHop, j), geoMap, config))
	}
	return hops
}

func globalpingTimingAt(gpHop *globalping.MTRHop, index int) *globalping.MTRTiming {
	if index >= len(gpHop.Timings) {
		return nil
	}
	return &gpHop.Timings[index]
}

func mapGlobalpingHop(ttl int, gpHop *globalping.MTRHop, timing *globalping.MTRTiming, geoMap map[string]*ipgeo.IPGeoData, config *Config) Hop {
	resolvedHostname := ""
	if config != nil && config.RDNS {
		if raw := strings.TrimSpace(gpHop.ResolvedHostname); raw != "" {
			trimmed := strings.TrimSuffix(raw, ".")
			if net.ParseIP(trimmed) == nil {
				resolvedHostname = CanonicalHostname(trimmed)
			}
		}
	}

	hop := Hop{
		Hostname: resolvedHostname,
		TTL:      ttl,
	}
	if config != nil {
		hop.Lang = config.Lang
	}

	if gpHop.ResolvedAddress != "" {
		hop.Address = &net.IPAddr{
			IP: net.ParseIP(gpHop.ResolvedAddress),
		}
		if geo, ok := geoMap[gpHop.ResolvedAddress]; ok {
			hop.Geo = geo
		} else if config != nil {
			_ = hop.fetchIPData(*config)
			geoMap[gpHop.ResolvedAddress] = hop.Geo
		}
	}

	if timing == nil {
		return hop
	}

	hop.Success = true
	hop.RTT = time.Duration(timing.RTT * float64(time.Millisecond))

	return hop
}

func hasGlobalpingProbeLocation(probe globalping.ProbeDetails) bool {
	return probe.City != "" ||
		probe.State != "" ||
		probe.Country != "" ||
		probe.Continent != "" ||
		probe.Network != "" ||
		probe.ASN != 0
}

func formatGlobalpingCity(probe globalping.ProbeDetails) string {
	if probe.City != "" && probe.State != "" {
		return probe.City + " (" + probe.State + ")"
	}
	if probe.City != "" {
		return probe.City
	}
	return probe.State
}

func formatGlobalpingNetwork(probe globalping.ProbeDetails) string {
	network := strings.TrimSpace(probe.Network)
	if network != "" && probe.ASN != 0 {
		return network + " (AS" + fmt.Sprint(probe.ASN) + ")"
	}
	if network != "" {
		return network
	}
	if probe.ASN != 0 {
		return "(AS" + fmt.Sprint(probe.ASN) + ")"
	}
	return ""
}

func appendGlobalpingPart(parts []string, value string) []string {
	if value == "" {
		return parts
	}
	return append(parts, value)
}

func GlobalpingFormatLocation(m *globalping.ProbeMeasurement) string {
	if m == nil {
		return ""
	}

	probe := m.Probe
	if !hasGlobalpingProbeLocation(probe) {
		return ""
	}

	var parts []string
	parts = appendGlobalpingPart(parts, formatGlobalpingCity(probe))
	parts = appendGlobalpingPart(parts, probe.Country)
	parts = appendGlobalpingPart(parts, probe.Continent)
	parts = appendGlobalpingPart(parts, formatGlobalpingNetwork(probe))

	return strings.Join(parts, ", ")
}
