//go:build !flavor_tiny && !flavor_ntr

package service

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/jsdelivr/globalping-cli/globalping"
)

func TestBuildGlobalpingCreateUsesLocationsAndOptions(t *testing.T) {
	req, err := buildGlobalpingCreate(GlobalpingTraceRequest{
		Target:    "example.com",
		Locations: []string{"Japan", "AS13335"},
		Limit:     2,
		Protocol:  "tcp",
		Port:      443,
		Packets:   5,
		IPVersion: 6,
	})
	if err != nil {
		t.Fatalf("buildGlobalpingCreate returned error: %v", err)
	}
	if req.Type != "mtr" {
		t.Fatalf("Type = %q, want mtr", req.Type)
	}
	if len(req.Locations) != 2 || req.Locations[0].Magic != "Japan" || req.Locations[1].Magic != "AS13335" {
		t.Fatalf("Locations = %#v", req.Locations)
	}
	if req.Limit != 2 {
		t.Fatalf("Limit = %d, want 2", req.Limit)
	}
	if req.Options.Protocol != "TCP" || req.Options.Port != 443 || req.Options.Packets != 5 || req.Options.IPVersion != globalping.IPVersion6 {
		t.Fatalf("Options = %#v", req.Options)
	}
}

func TestBuildGlobalpingCreateDefaultsPorts(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		wantPort uint16
	}{
		{name: "omitted protocol defaults to icmp", protocol: "", wantPort: 80},
		{name: "icmp", protocol: "icmp", wantPort: 80},
		{name: "tcp", protocol: "tcp", wantPort: 80},
		{name: "udp", protocol: "udp", wantPort: 33494},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := buildGlobalpingCreate(GlobalpingTraceRequest{
				Target:   "example.com",
				Protocol: tt.protocol,
			})
			if err != nil {
				t.Fatalf("buildGlobalpingCreate returned error: %v", err)
			}
			if req.Options.Port != tt.wantPort {
				t.Fatalf("Port = %d, want %d", req.Options.Port, tt.wantPort)
			}
		})
	}
}

func TestBuildGlobalpingCreateSerializesNonZeroDefaultPorts(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		wantPort uint16
	}{
		{name: "omitted protocol", protocol: "", wantPort: 80},
		{name: "icmp", protocol: "ICMP", wantPort: 80},
		{name: "tcp", protocol: "TCP", wantPort: 80},
		{name: "udp", protocol: "UDP", wantPort: 33494},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := buildGlobalpingCreate(GlobalpingTraceRequest{
				Target:   "example.com",
				Protocol: tt.protocol,
			})
			if err != nil {
				t.Fatalf("buildGlobalpingCreate returned error: %v", err)
			}
			payload, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			var decoded struct {
				Options struct {
					Port uint16 `json:"port"`
				} `json:"measurementOptions"`
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			if got := decoded.Options.Port; got != tt.wantPort {
				t.Fatalf("serialized port = %v, want %v; payload=%s", got, tt.wantPort, payload)
			}
		})
	}
}

func TestBuildGlobalpingCreatePreservesExplicitPort(t *testing.T) {
	req, err := buildGlobalpingCreate(GlobalpingTraceRequest{
		Target:   "example.com",
		Protocol: "udp",
		Port:     5300,
	})
	if err != nil {
		t.Fatalf("buildGlobalpingCreate returned error: %v", err)
	}
	if req.Options.Port != 5300 {
		t.Fatalf("Port = %d, want explicit 5300", req.Options.Port)
	}
}

func TestBuildGlobalpingCreateRejectsOutOfRangePorts(t *testing.T) {
	tests := []int{-1, 65536}
	for _, port := range tests {
		t.Run("port "+strconv.Itoa(port), func(t *testing.T) {
			_, err := buildGlobalpingCreate(GlobalpingTraceRequest{
				Target: "example.com",
				Port:   port,
			})
			if err == nil {
				t.Fatalf("buildGlobalpingCreate with port %d returned nil error", port)
			}
		})
	}
}

func TestTranslateGlobalpingMeasurementPreservesAllProbeResults(t *testing.T) {
	hopsRaw, err := json.Marshal([]globalping.MTRHop{
		{
			ResolvedAddress:  "192.0.2.1",
			ResolvedHostname: "router.example",
			ASN:              []int{64500},
			Timings:          []globalping.MTRTiming{{RTT: 12.5}},
			Stats:            globalping.MTRStats{Min: 12.5, Avg: 12.5, Max: 12.5, Total: 1, Rcv: 1},
		},
	})
	if err != nil {
		t.Fatalf("marshal hops: %v", err)
	}
	measurement := &globalping.Measurement{
		ID:          "m-1",
		Type:        "mtr",
		Status:      globalping.StatusFinished,
		Target:      "example.com",
		ProbesCount: 2,
		Results: []globalping.ProbeMeasurement{
			{
				Probe: globalping.ProbeDetails{Country: "JP", City: "Tokyo", ASN: 64500, Network: "ExampleNet", Tags: []string{"cloud"}},
				Result: globalping.ProbeResult{
					Status:          globalping.StatusFinished,
					ResolvedAddress: "93.184.216.34",
					HopsRaw:         hopsRaw,
					RawOutput:       "tokyo raw",
				},
			},
			{
				Probe: globalping.ProbeDetails{Country: "DE", City: "Frankfurt", ASN: 64501, Network: "ExampleDE"},
				Result: globalping.ProbeResult{
					Status:          globalping.StatusFinished,
					ResolvedAddress: "93.184.216.34",
					HopsRaw:         hopsRaw,
					RawOutput:       "frankfurt raw",
				},
			},
		},
	}

	got, err := translateGlobalpingMeasurement(measurement)
	if err != nil {
		t.Fatalf("translateGlobalpingMeasurement returned error: %v", err)
	}
	if got.MeasurementID != "m-1" || got.ProbesCount != 2 || len(got.Results) != 2 {
		t.Fatalf("response = %+v", got)
	}
	if got.Results[0].Probe.Country != "JP" || got.Results[1].Probe.Country != "DE" {
		t.Fatalf("probe locations not preserved: %+v", got.Results)
	}
	if got.Results[0].RawOutput != "tokyo raw" || got.Results[1].RawOutput != "frankfurt raw" {
		t.Fatalf("raw output not preserved: %+v", got.Results)
	}
	if len(got.Results[0].Hops) != 1 || got.Results[0].Hops[0].ResolvedAddress != "192.0.2.1" || got.Results[0].Hops[0].TimingsMs[0] != 12.5 {
		t.Fatalf("hops not translated: %+v", got.Results[0].Hops)
	}
}
