package service

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

type serviceJSONContracts struct {
	Trace     TraceResponse     `json:"trace"`
	MTRReport MTRReportResponse `json:"mtr_report"`
	MTRRaw    MTRRawResponse    `json:"mtr_raw"`
}

func TestServiceJSONContractsGolden(t *testing.T) {
	contracts := serviceJSONContracts{
		Trace: TraceResponse{
			Target:       "escape <&>\u2028 target",
			ResolvedIP:   "2001:db8::10",
			Protocol:     "udp",
			DataProvider: ipgeo.NextTraceAPIProvider,
			Language:     "en",
			Hops: []Hop{{
				TTL: 1,
				Attempts: []Attempt{
					{Success: true, IP: "192.0.2.1", Hostname: "edge.example", RTTMs: 1.25},
					{Success: false, Error: "timeout\nsecond line"},
				},
			}},
			DurationMs: 1250,
			Parameters: ParameterBoundaries{Supported: []string{"target", "protocol"}},
		},
		MTRReport: MTRReportResponse{
			Target:     "example.test",
			ResolvedIP: "192.0.2.9",
			Protocol:   "icmp",
			Stats: []trace.MTRHopStat{{
				TTL:      1,
				Host:     "edge.example",
				IP:       "192.0.2.1",
				Loss:     33.333333333333336,
				Snt:      3,
				Last:     1.25,
				Avg:      1.5,
				Best:     1.25,
				Wrst:     1.75,
				StDev:    0.25,
				Received: 2,
				Dropped:  1, GMean: 1.479019945774904,
				Jitter: 0.5, JitterAvg: 0.25, JitterMax: 0.5, JitterInterarrival: 0.5,
				MPLS: []string{"[MPLS: Lbl 16000, TC 0, S 1, TTL 63]"},
				Response: &trace.MTRProbeResponse{
					Kind:        trace.MTRResponseTransit,
					Description: "ICMP Time Exceeded",
				},
			}},
			DurationMs: 30000,
			Parameters: ParameterBoundaries{
				Supported:       []string{"target", "hop_interval_ms"},
				NotApplicable:   []string{"queries"},
				NotYetSupported: []string{"future_option"},
			},
			PathEnd: &TraceStopReason{
				Hop:       3,
				Reason:    trace.StopReasonUnreachable,
				Responses: []string{"ICMP Host Unreachable"},
				Markers:   []string{"!H"},
			},
		},
		MTRRaw: MTRRawResponse{
			Target:     "raw.example",
			ResolvedIP: "2001:db8::20",
			Protocol:   "tcp",
			Records: []trace.MTRRawRecord{{
				Iteration: 2,
				TTL:       4,
				Success:   true,
				IP:        "2001:db8::4",
				Host:      "router <&>\u2028 name",
				RTTMs:     12.75,
				ASN:       "AS64512",
				Country:   "ZZ",
				City:      "Example City",
				Owner:     "Example Network",
				Lat:       -0.125,
				Lng:       179.999,
				Response: &trace.MTRProbeResponse{
					Kind:   trace.MTRResponseDestination,
					Marker: "!X",
				},
			}},
			DurationMs: 5000,
			Warnings:   []string{"first line\nsecond line", ""},
			Parameters: ParameterBoundaries{Supported: []string{"target", "duration_ms"}},
		},
	}

	got, err := json.MarshalIndent(contracts, "", "  ")
	if err != nil {
		t.Fatalf("marshal service JSON contracts: %v", err)
	}
	got = append(got, '\n')
	assertServiceJSONGolden(t, got, "testdata/json_responses.golden.json")
}

func assertServiceJSONGolden(t *testing.T, got []byte, path string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON golden: %v", err)
	}
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode generated JSON: %v", err)
	}
	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("decode JSON golden: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON semantics changed\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("JSON bytes changed while semantics remain equal\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
