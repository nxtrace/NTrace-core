//go:build !flavor_tiny && !flavor_ntr

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jsdelivr/globalping-cli/globalping"

	"github.com/nxtrace/NTrace-core/trace"
)

func (s *Service) GlobalpingTrace(ctx context.Context, req GlobalpingTraceRequest) (GlobalpingMeasurementResponse, error) {
	create, err := buildGlobalpingCreate(req)
	if err != nil {
		return GlobalpingMeasurementResponse{}, err
	}
	measurement, err := trace.CreateGlobalpingMeasurement(ctx, trace.NewGlobalpingClient(ctx), create)
	if err != nil {
		return GlobalpingMeasurementResponse{}, err
	}
	return translateGlobalpingMeasurement(measurement)
}

func (s *Service) GlobalpingLimits(ctx context.Context, _ GlobalpingLimitsRequest) (GlobalpingLimitsResponse, error) {
	limits, err := trace.NewGlobalpingClient(ctx).Limits()
	if err != nil {
		return GlobalpingLimitsResponse{}, err
	}
	return GlobalpingLimitsResponse{
		Measurements: GlobalpingMeasurementLimits{
			Create: GlobalpingCreateLimit{
				Type:      string(limits.RateLimits.Measurements.Create.Type),
				Limit:     limits.RateLimits.Measurements.Create.Limit,
				Remaining: limits.RateLimits.Measurements.Create.Remaining,
				Reset:     limits.RateLimits.Measurements.Create.Reset,
			},
		},
		Credits:    GlobalpingCreditLimits{Remaining: limits.Credits.Remaining},
		Parameters: globalpingLimitsParameterBoundaries(),
	}, nil
}

func (s *Service) GlobalpingGetMeasurement(ctx context.Context, req GlobalpingGetMeasurementRequest) (GlobalpingMeasurementResponse, error) {
	id := strings.TrimSpace(req.MeasurementID)
	if id == "" {
		return GlobalpingMeasurementResponse{}, errors.New("measurement_id is required")
	}
	measurement, err := trace.NewGlobalpingClient(ctx).GetMeasurement(id)
	if err != nil {
		return GlobalpingMeasurementResponse{}, err
	}
	out, err := translateGlobalpingMeasurement(measurement)
	if err != nil {
		return GlobalpingMeasurementResponse{}, err
	}
	out.Parameters = globalpingGetParameterBoundaries()
	return out, nil
}

func buildGlobalpingCreate(req GlobalpingTraceRequest) (*globalping.MeasurementCreate, error) {
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, errors.New("target is required")
	}
	protocol := strings.ToUpper(strings.TrimSpace(req.Protocol))
	if protocol == "" {
		protocol = "ICMP"
	}
	switch protocol {
	case "ICMP", "TCP", "UDP":
	default:
		return nil, fmt.Errorf("unsupported globalping protocol %q", req.Protocol)
	}
	if req.Port < 0 || req.Port > 65535 {
		return nil, errors.New("port must be within range 0-65535")
	}
	port := req.Port
	if port == 0 {
		port = defaultGlobalpingPort(protocol)
	}
	locations := make([]globalping.Locations, 0, len(req.Locations))
	for _, loc := range req.Locations {
		if trimmed := strings.TrimSpace(loc); trimmed != "" {
			locations = append(locations, globalping.Locations{Magic: trimmed})
		}
	}
	if len(locations) == 0 {
		locations = append(locations, globalping.Locations{Magic: "world"})
	}
	limit := req.Limit
	if limit <= 0 {
		limit = len(locations)
	}
	if limit <= 0 {
		limit = 1
	}
	options := &globalping.MeasurementOptions{
		Protocol: protocol,
		Port:     uint16(port),
		Packets:  positiveOrDefault(req.Packets, defaultQueries),
	}
	switch req.IPVersion {
	case 0:
	case 4:
		options.IPVersion = globalping.IPVersion4
	case 6:
		options.IPVersion = globalping.IPVersion6
	default:
		return nil, errors.New("ip_version must be 4, 6, or omitted")
	}
	return &globalping.MeasurementCreate{
		Type:      "mtr",
		Target:    target,
		Limit:     limit,
		Locations: locations,
		Options:   options,
	}, nil
}

func defaultGlobalpingPort(protocol string) int {
	switch protocol {
	case "TCP":
		return 80
	case "UDP":
		return 33494
	default:
		return 0
	}
}

func translateGlobalpingMeasurement(measurement *globalping.Measurement) (GlobalpingMeasurementResponse, error) {
	if measurement == nil {
		return GlobalpingMeasurementResponse{}, errors.New("globalping measurement is nil")
	}
	results := make([]GlobalpingProbeResult, 0, len(measurement.Results))
	for _, result := range measurement.Results {
		translated, err := translateGlobalpingProbe(measurement.Type, result)
		if err != nil {
			return GlobalpingMeasurementResponse{}, err
		}
		results = append(results, translated)
	}
	return GlobalpingMeasurementResponse{
		MeasurementID: measurement.ID,
		Type:          measurement.Type,
		Target:        measurement.Target,
		Status:        string(measurement.Status),
		ProbesCount:   measurement.ProbesCount,
		Results:       results,
		Parameters:    globalpingTraceParameterBoundaries(),
	}, nil
}

func translateGlobalpingProbe(measurementType string, measurement globalping.ProbeMeasurement) (GlobalpingProbeResult, error) {
	hops, err := decodeGlobalpingHops(measurementType, measurement.Result.HopsRaw)
	if err != nil {
		return GlobalpingProbeResult{}, err
	}
	return GlobalpingProbeResult{
		Probe: GlobalpingProbeInfo{
			Continent: measurement.Probe.Continent,
			Region:    measurement.Probe.Region,
			Country:   measurement.Probe.Country,
			City:      measurement.Probe.City,
			State:     measurement.Probe.State,
			ASN:       measurement.Probe.ASN,
			Network:   measurement.Probe.Network,
			Tags:      measurement.Probe.Tags,
		},
		Status:           string(measurement.Result.Status),
		ResolvedAddress:  measurement.Result.ResolvedAddress,
		ResolvedHostname: measurement.Result.ResolvedHostname,
		Hops:             hops,
		RawOutput:        measurement.Result.RawOutput,
	}, nil
}

func decodeGlobalpingHops(measurementType string, raw json.RawMessage) ([]GlobalpingHop, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(measurementType)) {
	case "traceroute":
		hops, err := globalping.DecodeTracerouteHops(raw)
		if err != nil {
			return nil, err
		}
		return translateTracerouteHops(hops), nil
	default:
		hops, err := globalping.DecodeMTRHops(raw)
		if err != nil {
			return nil, err
		}
		return translateMTRHops(hops), nil
	}
}

func translateMTRHops(hops []globalping.MTRHop) []GlobalpingHop {
	out := make([]GlobalpingHop, 0, len(hops))
	for i, hop := range hops {
		out = append(out, GlobalpingHop{
			TTL:              i + 1,
			ResolvedAddress:  hop.ResolvedAddress,
			ResolvedHostname: hop.ResolvedHostname,
			ASN:              hop.ASN,
			TimingsMs:        translateMTRTimings(hop.Timings),
			Stats: &GlobalpingMTRStats{
				Min:   hop.Stats.Min,
				Avg:   hop.Stats.Avg,
				Max:   hop.Stats.Max,
				StDev: hop.Stats.StDev,
				JMin:  hop.Stats.JMin,
				JAvg:  hop.Stats.JAvg,
				JMax:  hop.Stats.JMax,
				Total: hop.Stats.Total,
				Rcv:   hop.Stats.Rcv,
				Drop:  hop.Stats.Drop,
				Loss:  hop.Stats.Loss,
			},
		})
	}
	return out
}

func translateTracerouteHops(hops []globalping.TracerouteHop) []GlobalpingHop {
	out := make([]GlobalpingHop, 0, len(hops))
	for i, hop := range hops {
		out = append(out, GlobalpingHop{
			TTL:              i + 1,
			ResolvedAddress:  hop.ResolvedAddress,
			ResolvedHostname: hop.ResolvedHostname,
			TimingsMs:        translateTracerouteTimings(hop.Timings),
		})
	}
	return out
}

func translateMTRTimings(timings []globalping.MTRTiming) []float64 {
	if len(timings) == 0 {
		return nil
	}
	out := make([]float64, 0, len(timings))
	for _, timing := range timings {
		out = append(out, timing.RTT)
	}
	return out
}

func translateTracerouteTimings(timings []globalping.TracerouteTiming) []float64 {
	if len(timings) == 0 {
		return nil
	}
	out := make([]float64, 0, len(timings))
	for _, timing := range timings {
		out = append(out, timing.RTT)
	}
	return out
}
