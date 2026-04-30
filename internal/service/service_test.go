package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	mtutrace "github.com/nxtrace/NTrace-core/trace/mtu"
	"github.com/nxtrace/NTrace-core/util"
)

func TestMTRParameterBoundariesMatchMCPBehavior(t *testing.T) {
	caps, err := New().Capabilities(context.Background(), CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("Capabilities returned error: %v", err)
	}
	tools := map[string]ParameterBoundaries{}
	for _, tool := range caps.Tools {
		tools[tool.Name] = tool.Parameters
	}
	report := requireToolBoundaries(t, tools, "nexttrace_mtr_report")
	raw := requireToolBoundaries(t, tools, "nexttrace_mtr_raw")

	for _, params := range []struct {
		name       string
		boundaries ParameterBoundaries
		supported  []string
		notApp     []string
	}{
		{
			name:       "report",
			boundaries: report,
			supported:  []string{"target", "hop_interval_ms", "max_per_hop"},
			notApp:     []string{"queries", "packet_interval", "ttl_interval"},
		},
		{
			name:       "raw",
			boundaries: raw,
			supported:  []string{"target", "hop_interval_ms", "max_per_hop", "duration_ms"},
			notApp:     []string{"queries", "packet_interval", "ttl_interval"},
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			for _, param := range params.supported {
				if !containsParam(params.boundaries.Supported, param) {
					t.Fatalf("%s supported missing %s: %+v", params.name, param, params.boundaries)
				}
			}
			for _, param := range params.notApp {
				if containsParam(params.boundaries.Supported, param) {
					t.Fatalf("%s supported includes non-applicable %s: %+v", params.name, param, params.boundaries)
				}
				if !containsParam(params.boundaries.NotApplicable, param) {
					t.Fatalf("%s not_applicable missing %s: %+v", params.name, param, params.boundaries)
				}
			}
		})
	}
}

func requireToolBoundaries(t *testing.T, tools map[string]ParameterBoundaries, name string) ParameterBoundaries {
	t.Helper()
	boundaries, ok := tools[name]
	if ok {
		return boundaries
	}
	names := make([]string, 0, len(tools))
	for toolName := range tools {
		names = append(names, toolName)
	}
	sort.Strings(names)
	t.Fatalf("Capabilities missing tool %s; got tools=%v", name, names)
	return ParameterBoundaries{}
}

func TestMTRRawReturnsParentCancellation(t *testing.T) {
	restore := stubServiceRuntimeForTests(t)
	defer restore()

	runMTRRawFn = func(context.Context, trace.Method, trace.Config, trace.MTRRawOptions, trace.MTRRawOnRecord) error {
		return context.Canceled
	}
	_, err := New().MTRRaw(context.Background(), MTRRawRequest{
		TraceRequest: TraceRequest{Target: "192.0.2.1", DataProvider: "disable-geoip"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("MTRRaw error = %v, want context.Canceled", err)
	}

	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	cancel()
	errStub := errors.New("stub error")
	runMTRRawFn = func(context.Context, trace.Method, trace.Config, trace.MTRRawOptions, trace.MTRRawOnRecord) error {
		return errStub
	}
	_, err = New().MTRRaw(parent, MTRRawRequest{
		TraceRequest: TraceRequest{Target: "192.0.2.1", DataProvider: "disable-geoip"},
		DurationMs:   1000,
	})
	if errors.Is(err, errStub) {
		t.Fatalf("MTRRaw deadline error = %v, want parent context deadline", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("MTRRaw deadline error = %v, want context.DeadlineExceeded", err)
	}
}

func TestMTRRawAllowsLocalDurationTimeout(t *testing.T) {
	restore := stubServiceRuntimeForTests(t)
	defer restore()

	runMTRRawFn = func(_ context.Context, _ trace.Method, _ trace.Config, _ trace.MTRRawOptions, onRecord trace.MTRRawOnRecord) error {
		onRecord(trace.MTRRawRecord{TTL: 1, Success: true, IP: "192.0.2.1"})
		return context.DeadlineExceeded
	}
	resp, err := New().MTRRaw(context.Background(), MTRRawRequest{
		TraceRequest: TraceRequest{Target: "192.0.2.1", DataProvider: "disable-geoip"},
		DurationMs:   1,
	})
	if err != nil {
		t.Fatalf("MTRRaw returned error: %v", err)
	}
	if len(resp.Records) != 1 || resp.Records[0].IP != "192.0.2.1" {
		t.Fatalf("MTRRaw records = %+v, want one preserved record", resp.Records)
	}
}

func TestMTRResponsesUseMTRParameterBoundaries(t *testing.T) {
	restore := stubServiceRuntimeForTests(t)
	defer restore()

	runMTRFn = func(_ context.Context, _ trace.Method, _ trace.Config, _ trace.MTROptions, onUpdate trace.MTROnSnapshot) error {
		onUpdate(1, []trace.MTRHopStat{{TTL: 1, IP: "192.0.2.1"}})
		return nil
	}
	runMTRRawFn = func(_ context.Context, _ trace.Method, _ trace.Config, _ trace.MTRRawOptions, onRecord trace.MTRRawOnRecord) error {
		onRecord(trace.MTRRawRecord{TTL: 1, Success: true, IP: "192.0.2.1"})
		return nil
	}

	report, err := New().MTRReport(context.Background(), MTRReportRequest{
		TraceRequest: TraceRequest{Target: "192.0.2.1", DataProvider: "disable-geoip"},
	})
	if err != nil {
		t.Fatalf("MTRReport returned error: %v", err)
	}
	assertMTRBoundaries(t, "report", report.Parameters, false)

	raw, err := New().MTRRaw(context.Background(), MTRRawRequest{
		TraceRequest: TraceRequest{Target: "192.0.2.1", DataProvider: "disable-geoip"},
		MaxPerHop:    1,
	})
	if err != nil {
		t.Fatalf("MTRRaw returned error: %v", err)
	}
	assertMTRBoundaries(t, "raw", raw.Parameters, true)
}

func TestMTUTraceInitializesDefaultLeoMoeRuntime(t *testing.T) {
	restore := stubServiceRuntimeForTests(t)
	defer restore()

	var ensureCalls int
	ensureLeoMoeConnectionFn = func(context.Context) {
		ensureCalls++
	}
	runMTUTraceFn = func(_ context.Context, cfg mtutrace.Config) (*mtutrace.Result, error) {
		return &mtutrace.Result{
			Target:     cfg.Target,
			ResolvedIP: cfg.DstIP.String(),
			Protocol:   "udp",
			IPVersion:  4,
			StartMTU:   1500,
			PathMTU:    1500,
		}, nil
	}

	resp, err := New().MTUTrace(context.Background(), MTUTraceRequest{Target: "192.0.2.1"})
	if err != nil {
		t.Fatalf("MTUTrace returned error: %v", err)
	}
	if ensureCalls != 1 {
		t.Fatalf("ensureLeoMoeConnection calls = %d, want 1", ensureCalls)
	}
	if resp.ResolvedIP != "192.0.2.1" {
		t.Fatalf("ResolvedIP = %q, want 192.0.2.1", resp.ResolvedIP)
	}
}

func TestAnnotateIPsAndGeoLookupInitializeDefaultLeoMoeRuntime(t *testing.T) {
	restore := stubServiceRuntimeForTests(t)
	defer restore()

	var ensureCalls int
	ensureLeoMoeConnectionFn = func(context.Context) {
		ensureCalls++
	}
	lookupIPGeoFn = func(_ context.Context, _ ipgeo.Source, _ string, _ bool, _ int, query string) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{IP: query, Asnumber: "AS13335"}, nil
	}

	if _, err := New().AnnotateIPs(context.Background(), AnnotateIPsRequest{Text: "plain text"}); err != nil {
		t.Fatalf("AnnotateIPs returned error: %v", err)
	}
	if _, err := New().GeoLookup(context.Background(), GeoLookupRequest{Query: "8.8.8.8"}); err != nil {
		t.Fatalf("GeoLookup returned error: %v", err)
	}
	if ensureCalls != 2 {
		t.Fatalf("ensureLeoMoeConnection calls = %d, want 2", ensureCalls)
	}
}

func TestAnnotateIPsAndGeoLookupSkipLeoRuntimeForDisabledGeoIP(t *testing.T) {
	restore := stubServiceRuntimeForTests(t)
	defer restore()

	var ensureCalls int
	ensureLeoMoeConnectionFn = func(context.Context) {
		ensureCalls++
	}
	lookupIPGeoFn = func(_ context.Context, _ ipgeo.Source, _ string, _ bool, _ int, query string) (*ipgeo.IPGeoData, error) {
		return &ipgeo.IPGeoData{IP: query}, nil
	}

	if _, err := New().AnnotateIPs(context.Background(), AnnotateIPsRequest{Text: "plain text", DataProvider: "disable-geoip"}); err != nil {
		t.Fatalf("AnnotateIPs returned error: %v", err)
	}
	if _, err := New().GeoLookup(context.Background(), GeoLookupRequest{Query: "8.8.8.8", DataProvider: "disable-geoip"}); err != nil {
		t.Fatalf("GeoLookup returned error: %v", err)
	}
	if ensureCalls != 0 {
		t.Fatalf("ensureLeoMoeConnection calls = %d, want 0", ensureCalls)
	}
}

func containsParam(params []string, target string) bool {
	for _, param := range params {
		if param == target {
			return true
		}
	}
	return false
}

func assertMTRBoundaries(t *testing.T, name string, boundaries ParameterBoundaries, wantDuration bool) {
	t.Helper()

	if containsParam(boundaries.Supported, "queries") {
		t.Fatalf("%s supported includes queries: %+v", name, boundaries)
	}
	if !containsParam(boundaries.NotApplicable, "queries") {
		t.Fatalf("%s not_applicable missing queries: %+v", name, boundaries)
	}
	if !containsParam(boundaries.Supported, "hop_interval_ms") || !containsParam(boundaries.Supported, "max_per_hop") {
		t.Fatalf("%s supported missing MTR controls: %+v", name, boundaries)
	}
	if gotDuration := containsParam(boundaries.Supported, "duration_ms"); gotDuration != wantDuration {
		t.Fatalf("%s duration_ms supported = %v, want %v: %+v", name, gotDuration, wantDuration, boundaries)
	}
}

func stubServiceRuntimeForTests(t *testing.T) func() {
	t.Helper()

	oldEnsureLeo := ensureLeoMoeConnectionFn
	oldLookupIPGeo := lookupIPGeoFn
	oldRunMTR := runMTRFn
	oldRunMTRRaw := runMTRRawFn
	oldRunMTU := runMTUTraceFn
	oldEnvDataProvider := util.EnvDataProvider
	util.EnvDataProvider = ""
	ensureLeoMoeConnectionFn = func(context.Context) {}
	return func() {
		ensureLeoMoeConnectionFn = oldEnsureLeo
		lookupIPGeoFn = oldLookupIPGeo
		runMTRFn = oldRunMTR
		runMTRRawFn = oldRunMTRRaw
		runMTUTraceFn = oldRunMTU
		util.EnvDataProvider = oldEnvDataProvider
	}
}
