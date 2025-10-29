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

type GlobalpingOptions struct {
	Target  string
	From    string
	IPv4    bool
	IPv6    bool
	TCP     bool
	UDP     bool
	Port    int
	Packets int
	MaxHops int

	DisableMaptrace bool
	DataOrigin      string

	TablePrint   bool
	ClassicPrint bool
	RawPrint     bool
	JSONPrint    bool
}

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
	if config.RDNS {
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
		Lang:     config.Lang,
	}

	if gpHop.ResolvedAddress != "" {
		hop.Address = &net.IPAddr{
			IP: net.ParseIP(gpHop.ResolvedAddress),
		}
		if geo, ok := geoMap[gpHop.ResolvedAddress]; ok {
			hop.Geo = geo
		} else {
			// 此处不处理错误
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

func GlobalpingFormatLocation(m *globalping.ProbeMeasurement) string {
	state := ""
	if m.Probe.State != "" {
		state = " (" + m.Probe.State + ")"
	}
	return m.Probe.City + state + ", " +
		m.Probe.Country + ", " +
		m.Probe.Continent + ", " +
		m.Probe.Network + " " +
		"(AS" + fmt.Sprint(m.Probe.ASN) + ")"
}
