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
	c := globalping.Config{
		UserAgent: "NextTrace/" + _config.Version,
	}
	if util.GlobalpingToken != "" {
		c.AuthToken = &globalping.Token{
			AccessToken: util.GlobalpingToken,
			Expiry:      time.Now().Add(math.MaxInt64),
		}
	}
	client := globalping.NewClient(c)

	o := &globalping.MeasurementCreate{
		Type:   "mtr",
		Target: opts.Target,
		Limit:  1,
		Locations: []globalping.Locations{
			{
				Magic: opts.From,
			},
		},
		Options: &globalping.MeasurementOptions{
			Port:    uint16(opts.Port),
			Packets: opts.Packets,
		},
	}

	if opts.TCP {
		o.Options.Protocol = "TCP"
	} else if opts.UDP {
		o.Options.Protocol = "UDP"
	} else {
		o.Options.Protocol = "ICMP"
	}

	switch {
	case opts.IPv4 && !opts.IPv6:
		o.Options.IPVersion = globalping.IPVersion4
	case opts.IPv6 && !opts.IPv4:
		o.Options.IPVersion = globalping.IPVersion6
	default:
		// 两者均未指定或同时为 true：不设 IPVersion，交由平台选路
	}

	res, err := client.CreateMeasurement(o)
	if err != nil {
		return nil, nil, err
	}

	measurement, err := client.AwaitMeasurement(res.ID)
	if err != nil {
		return nil, nil, err
	}

	if measurement.Status != globalping.StatusFinished {
		return nil, nil, fmt.Errorf("measurement did not complete successfully: %s", measurement.Status)
	}

	if len(measurement.Results) == 0 {
		return nil, measurement, fmt.Errorf("globalping measurement returned no probe results")
	}

	firstResult := measurement.Results[0]
	if len(firstResult.Result.HopsRaw) == 0 {
		return nil, measurement, fmt.Errorf("globalping measurement results did not include hop data")
	}

	gpHops, err := globalping.DecodeMTRHops(firstResult.Result.HopsRaw)
	if err != nil {
		return nil, nil, err
	}

	limit := opts.MaxHops
	if limit <= 0 && config != nil && config.MaxHops > 0 {
		limit = config.MaxHops
	}
	if limit <= 0 || limit > len(gpHops) {
		limit = len(gpHops)
	}

	result := &Result{}
	geoMap := map[string]*ipgeo.IPGeoData{}
	maxTimings := 1

	for i := 0; i < limit; i++ {
		if count := len(gpHops[i].Timings); count > maxTimings {
			maxTimings = count
		}
	}
	for i := 0; i < limit; i++ {
		hops := make([]Hop, 0, maxTimings)
		for j := 0; j < maxTimings; j++ {
			var timing *globalping.MTRTiming
			if j < len(gpHops[i].Timings) {
				timing = &gpHops[i].Timings[j]
			}
			hop := mapGlobalpingHop(i+1, &gpHops[i], timing, geoMap, config)
			hops = append(hops, hop)
		}
		result.Hops = append(result.Hops, hops)
	}

	return result, measurement, nil
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
