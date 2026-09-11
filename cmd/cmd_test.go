package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akamensky/argparse"
	"github.com/fatih/color"
	fastTrace "github.com/nxtrace/NTrace-core/fast_trace"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
	"github.com/spf13/viper"
)

var errCmdOutputWriter = errors.New("cmd output writer failed")

type failingCmdOutputWriter struct{}

func (failingCmdOutputWriter) Write([]byte) (int, error) {
	return 0, errCmdOutputWriter
}

func TestMarshalTraceMapPayloadKeepsHistoricalShape(t *testing.T) {
	res := &trace.Result{
		Hops:        [][]trace.Hop{{{TTL: 1}}},
		StopReason:  &trace.StopReason{Hop: 1, Reason: trace.StopReasonDestination},
		TraceMapUrl: "https://map.example.test",
	}

	payload, err := marshalTraceMapPayload(res)
	if err != nil {
		t.Fatalf("marshalTraceMapPayload() error = %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("trace map payload decode: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("trace map payload fields = %v, want historical Hops and TraceMapUrl only", fields)
	}
	if _, ok := fields["Hops"]; !ok {
		t.Fatalf("trace map payload missing Hops: %s", payload)
	}
	if got := string(fields["TraceMapUrl"]); got != `"https://map.example.test"` {
		t.Fatalf("trace map payload TraceMapUrl = %s", got)
	}
	if _, ok := fields["StopReason"]; ok {
		t.Fatalf("trace map payload leaked StopReason: %s", payload)
	}

	fullResult, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("full result marshal: %v", err)
	}
	if !bytes.Contains(fullResult, []byte(`"StopReason"`)) {
		t.Fatalf("full JSON result lost StopReason: %s", fullResult)
	}
}

func TestLookupTargetIPHonorsContextCancellation(t *testing.T) {
	oldLookup := domainLookupFn
	domainLookupFn = func(ctx context.Context, host, ipVersion, dotServer string, disableOutput bool) (net.IP, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	defer func() { domainLookupFn = oldLookup }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := lookupTargetIP(ctx, "example.com", false, false, "", true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lookupTargetIP error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("lookupTargetIP returned too slowly after cancel: %v", elapsed)
	}
}

func TestLookupTargetIPOrExitReturnsFalseOnContextCancellation(t *testing.T) {
	oldLookup := domainLookupFn
	domainLookupFn = func(ctx context.Context, host, ipVersion, dotServer string, disableOutput bool) (net.IP, error) {
		return nil, context.Canceled
	}
	defer func() { domainLookupFn = oldLookup }()

	ip, ok := lookupTargetIPOrExit(context.Background(), "example.com", false, false, "", true)
	if ok {
		t.Fatal("lookupTargetIPOrExit ok = true, want false for canceled context")
	}
	if ip != nil {
		t.Fatalf("lookupTargetIPOrExit ip = %v, want nil", ip)
	}
}

func TestLookupTargetIPOrExitReturnsFalseOnContextDeadline(t *testing.T) {
	oldLookup := domainLookupFn
	domainLookupFn = func(ctx context.Context, host, ipVersion, dotServer string, disableOutput bool) (net.IP, error) {
		return nil, context.DeadlineExceeded
	}
	defer func() { domainLookupFn = oldLookup }()

	ip, ok := lookupTargetIPOrExit(context.Background(), "example.com", false, false, "", true)
	if ok {
		t.Fatal("lookupTargetIPOrExit ok = true, want false for deadline context")
	}
	if ip != nil {
		t.Fatalf("lookupTargetIPOrExit ip = %v, want nil", ip)
	}
}

func TestInitNextTraceAPIV3WebSocketSkipsV3WhenNextTraceAPIV4TokenConfigured(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "v4-token")
	oldPrepare := prepareNextTraceAPIV4FastIPFn
	oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
	var prepareCalls int
	var wsCalls int
	prepareNextTraceAPIV4FastIPFn = func(ctx context.Context, enableOutput bool) error {
		prepareCalls++
		if ctx == nil {
			t.Fatal("PrepareNextTraceAPIV4FastIP context = nil")
		}
		if !enableOutput {
			t.Fatal("PrepareNextTraceAPIV4FastIP enableOutput = false, want true")
		}
		return nil
	}
	newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
		wsCalls++
		return nil
	}
	t.Cleanup(func() {
		prepareNextTraceAPIV4FastIPFn = oldPrepare
		newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
	})
	dataProvider := ipgeo.NextTraceAPIProvider
	powProvider := "api.nxtrace.org"

	if got := initNextTraceAPIV3WebSocket(context.Background(), &dataProvider, &powProvider, false); got != nil {
		t.Fatalf("initNextTraceAPIV3WebSocket() = %+v, want nil when NextTrace API v4 token is configured", got)
	}
	if prepareCalls != 1 {
		t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 1", prepareCalls)
	}
	if wsCalls != 0 {
		t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 0 when API v4 preheat succeeds", wsCalls)
	}
}

func TestInitNextTraceAPIV3WebSocketSkipsV3WhenNextTraceAPIV4TokenFileConfigured(t *testing.T) {
	tests := []struct {
		name      string
		writePath func(paths nextTraceAPIV4TokenPaths) string
	}{
		{
			name: "session",
			writePath: func(paths nextTraceAPIV4TokenPaths) string {
				return paths.session
			},
		},
		{
			name: "latest",
			writePath: func(paths nextTraceAPIV4TokenPaths) string {
				return paths.latest
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := isolateCmdNextTraceAPIV4TokenFiles(t)
			writeNextTraceAPIV4TokenFileForTest(t, tt.writePath(paths), "file-token\n")
			oldPrepare := prepareNextTraceAPIV4FastIPFn
			oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
			var prepareCalls int
			var wsCalls int
			prepareNextTraceAPIV4FastIPFn = func(ctx context.Context, enableOutput bool) error {
				prepareCalls++
				if ctx == nil {
					t.Fatal("PrepareNextTraceAPIV4FastIP context = nil")
				}
				if !enableOutput {
					t.Fatal("PrepareNextTraceAPIV4FastIP enableOutput = false, want true")
				}
				return nil
			}
			newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
				wsCalls++
				return nil
			}
			t.Cleanup(func() {
				prepareNextTraceAPIV4FastIPFn = oldPrepare
				newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
			})
			dataProvider := ipgeo.NextTraceAPIProvider
			powProvider := "api.nxtrace.org"

			if got := initNextTraceAPIV3WebSocket(context.Background(), &dataProvider, &powProvider, false); got != nil {
				t.Fatalf("initNextTraceAPIV3WebSocket() = %+v, want nil when NextTrace API v4 token file is configured", got)
			}
			if prepareCalls != 1 {
				t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 1", prepareCalls)
			}
			if wsCalls != 0 {
				t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 0 when API v4 preheat succeeds", wsCalls)
			}
			if got := os.Getenv(util.EnvNextTraceAPIV4TokenKey); got != "file-token" {
				t.Fatalf("%s = %q, want token loaded from file", util.EnvNextTraceAPIV4TokenKey, got)
			}
		})
	}
}

func TestInitNextTraceAPIV3WebSocketFallsBackToV3WhenAPIV4FastIPFails(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "v4-token")
	oldPrepare := prepareNextTraceAPIV4FastIPFn
	oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
	var prepareCalls int
	var wsCalls int
	prepareNextTraceAPIV4FastIPFn = func(context.Context, bool) error {
		prepareCalls++
		return errors.New("fastip unavailable")
	}
	newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
		wsCalls++
		return nil
	}
	t.Cleanup(func() {
		prepareNextTraceAPIV4FastIPFn = oldPrepare
		newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
	})
	dataProvider := ipgeo.NextTraceAPIProvider
	powProvider := "api.nxtrace.org"

	_ = initNextTraceAPIV3WebSocket(context.Background(), &dataProvider, &powProvider, false)
	if prepareCalls != 1 {
		t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 1", prepareCalls)
	}
	if wsCalls != 1 {
		t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 1 after API v4 preheat failure", wsCalls)
	}
}

func TestInitNextTraceAPIV3WebSocketFallsBackToV3WhenAPIV4TokenMissing(t *testing.T) {
	isolateCmdNextTraceAPIV4TokenFiles(t)
	oldPrepare := prepareNextTraceAPIV4FastIPFn
	oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
	var prepareCalls int
	var wsCalls int
	prepareNextTraceAPIV4FastIPFn = func(context.Context, bool) error {
		prepareCalls++
		return nil
	}
	newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
		wsCalls++
		return nil
	}
	t.Cleanup(func() {
		prepareNextTraceAPIV4FastIPFn = oldPrepare
		newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
	})
	dataProvider := ipgeo.NextTraceAPIProvider
	powProvider := "api.nxtrace.org"

	_ = initNextTraceAPIV3WebSocket(context.Background(), &dataProvider, &powProvider, false)
	if prepareCalls != 0 {
		t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 0 without API v4 token", prepareCalls)
	}
	if wsCalls != 1 {
		t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 1 without API v4 token", wsCalls)
	}
}

func TestRunFastTraceModePreparesRuntimeAndMarksParams(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "v4-token")
	oldPrepare := prepareNextTraceAPIV4FastIPFn
	oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
	oldRunFastTrace := runFastTraceFn
	var prepareCalls int
	var wsCalls int
	var runCalls int
	var gotRuntimePrepared bool
	prepareNextTraceAPIV4FastIPFn = func(context.Context, bool) error {
		prepareCalls++
		return nil
	}
	newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
		wsCalls++
		return nil
	}
	runFastTraceFn = func(_ trace.Method, params fastTrace.ParamsFastTrace) {
		runCalls++
		gotRuntimePrepared = params.RuntimePrepared
	}
	t.Cleanup(func() {
		prepareNextTraceAPIV4FastIPFn = oldPrepare
		newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
		runFastTraceFn = oldRunFastTrace
	})
	dataProvider := ipgeo.NextTraceAPIProvider
	disableMaptrace := false
	powProvider := "api.nxtrace.org"

	if !runFastTraceModeWithRuntime(context.Background(), false, &dataProvider, &disableMaptrace, &powProvider, "", true, "", fastTrace.ParamsFastTrace{}, trace.ICMPTrace) {
		t.Fatal("runFastTraceModeWithRuntime returned false, want true")
	}
	if prepareCalls != 1 {
		t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 1", prepareCalls)
	}
	if wsCalls != 0 {
		t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 0 when API v4 preheat succeeds", wsCalls)
	}
	if runCalls != 1 {
		t.Fatalf("FastTest calls = %d, want 1", runCalls)
	}
	if !gotRuntimePrepared {
		t.Fatal("FastTest RuntimePrepared = false, want true")
	}
}

func TestRunFastTraceModeMarksRuntimePreparedAfterAPIV4FallbackToV3(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "v4-token")
	oldPrepare := prepareNextTraceAPIV4FastIPFn
	oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
	oldRunFastTrace := runFastTraceFn
	var prepareCalls int
	var wsCalls int
	var gotRuntimePrepared bool
	prepareNextTraceAPIV4FastIPFn = func(context.Context, bool) error {
		prepareCalls++
		return errors.New("fastip unavailable")
	}
	newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
		wsCalls++
		return &wshandle.WsConn{}
	}
	runFastTraceFn = func(_ trace.Method, params fastTrace.ParamsFastTrace) {
		gotRuntimePrepared = params.RuntimePrepared
	}
	t.Cleanup(func() {
		prepareNextTraceAPIV4FastIPFn = oldPrepare
		newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
		runFastTraceFn = oldRunFastTrace
	})
	dataProvider := ipgeo.NextTraceAPIProvider
	disableMaptrace := false
	powProvider := "api.nxtrace.org"

	if !runFastTraceModeWithRuntime(context.Background(), false, &dataProvider, &disableMaptrace, &powProvider, "", true, "", fastTrace.ParamsFastTrace{}, trace.ICMPTrace) {
		t.Fatal("runFastTraceModeWithRuntime returned false, want true")
	}
	if prepareCalls != 1 {
		t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 1", prepareCalls)
	}
	if wsCalls != 1 {
		t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 1 after API v4 preheat failure", wsCalls)
	}
	if !gotRuntimePrepared {
		t.Fatal("FastTest RuntimePrepared = false, want true after v3 fallback")
	}
}

func TestRunFastTraceModeLeavesRuntimeUnpreparedForNonNextTraceAPIProvider(t *testing.T) {
	tests := []struct {
		name         string
		dataProvider string
		envProvider  string
	}{
		{name: "cli provider", dataProvider: "IPInfo"},
		{name: "env override", dataProvider: ipgeo.NextTraceAPIProvider, envProvider: "IPInfo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateCmdNextTraceAPIV4TokenFiles(t)
			oldEnvDataProvider := util.EnvDataProvider
			util.EnvDataProvider = tt.envProvider
			oldPrepare := prepareNextTraceAPIV4FastIPFn
			oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
			oldRunFastTrace := runFastTraceFn
			var prepareCalls int
			var wsCalls int
			var runCalls int
			var gotRuntimePrepared bool
			prepareNextTraceAPIV4FastIPFn = func(context.Context, bool) error {
				prepareCalls++
				return nil
			}
			newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
				wsCalls++
				return &wshandle.WsConn{}
			}
			runFastTraceFn = func(_ trace.Method, params fastTrace.ParamsFastTrace) {
				runCalls++
				gotRuntimePrepared = params.RuntimePrepared
			}
			t.Cleanup(func() {
				util.EnvDataProvider = oldEnvDataProvider
				prepareNextTraceAPIV4FastIPFn = oldPrepare
				newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
				runFastTraceFn = oldRunFastTrace
			})
			dataProvider := tt.dataProvider
			disableMaptrace := false
			powProvider := "api.nxtrace.org"

			if !runFastTraceModeWithRuntime(context.Background(), false, &dataProvider, &disableMaptrace, &powProvider, "", true, "", fastTrace.ParamsFastTrace{}, trace.ICMPTrace) {
				t.Fatal("runFastTraceModeWithRuntime returned false, want true")
			}
			if prepareCalls != 0 {
				t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 0 for non-NextTrace API provider", prepareCalls)
			}
			if wsCalls != 0 {
				t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 0 for non-NextTrace API provider", wsCalls)
			}
			if runCalls != 1 {
				t.Fatalf("FastTest calls = %d, want 1", runCalls)
			}
			if gotRuntimePrepared {
				t.Fatal("FastTest RuntimePrepared = true, want false for non-NextTrace API provider")
			}
		})
	}
}

func TestRunFastTraceModePinsExplicitDN42Provider(t *testing.T) {
	t.Chdir(t.TempDir())

	oldRunFastTrace := runFastTraceFn
	var got fastTrace.ParamsFastTrace
	runFastTraceFn = func(_ trace.Method, params fastTrace.ParamsFastTrace) {
		got = params
	}
	t.Cleanup(func() {
		runFastTraceFn = oldRunFastTrace
	})

	dataProvider := " dn42 "
	disableMaptrace := false
	powProvider := "api.nxtrace.org"
	if !runFastTraceModeWithRuntime(context.Background(), false, &dataProvider, &disableMaptrace, &powProvider, "", true, "", fastTrace.ParamsFastTrace{}, trace.ICMPTrace) {
		t.Fatal("runFastTraceModeWithRuntime returned false, want true")
	}
	if dataProvider != "DN42" || got.DataProvider != "DN42" || !got.DN42 || got.IPGeoSource == nil {
		t.Fatalf("DN42 fast trace params = provider %q, data %q, DN42 %v, source nil %v", dataProvider, got.DataProvider, got.DN42, got.IPGeoSource == nil)
	}
	if !disableMaptrace {
		t.Fatal("disableMaptrace = false, want true for explicit DN42 provider")
	}
}

func TestRunFastTraceModeAppliesDN42EnvironmentOverride(t *testing.T) {
	t.Chdir(t.TempDir())
	oldEnvDataProvider := util.EnvDataProvider
	oldRunFastTrace := runFastTraceFn
	util.EnvDataProvider = " dn42 "
	var got fastTrace.ParamsFastTrace
	runFastTraceFn = func(_ trace.Method, params fastTrace.ParamsFastTrace) {
		got = params
	}
	t.Cleanup(func() {
		util.EnvDataProvider = oldEnvDataProvider
		runFastTraceFn = oldRunFastTrace
	})

	dataProvider := ipgeo.NextTraceAPIProvider
	disableMaptrace := false
	powProvider := "api.nxtrace.org"
	if !runFastTraceModeWithRuntime(context.Background(), false, &dataProvider, &disableMaptrace, &powProvider, "", true, "", fastTrace.ParamsFastTrace{}, trace.ICMPTrace) {
		t.Fatal("runFastTraceModeWithRuntime returned false, want true")
	}
	if dataProvider != "DN42" || got.DataProvider != "DN42" || !got.DN42 || got.IPGeoSource == nil {
		t.Fatalf("DN42 env params = provider %q, data %q, DN42 %v, source nil %v", dataProvider, got.DataProvider, got.DN42, got.IPGeoSource == nil)
	}
	if !disableMaptrace || got.RuntimePrepared {
		t.Fatalf("DN42 env state = disable maptrace %v, runtime prepared %v", disableMaptrace, got.RuntimePrepared)
	}
}

func TestBuildTraceConfigPinsAndRefreshesDN42Source(t *testing.T) {
	dir := t.TempDir()
	geoFeedPath := filepath.Join(dir, "geofeed.csv")
	ptrPath := filepath.Join(dir, "ptr.csv")
	if err := os.WriteFile(geoFeedPath, []byte("10.0.0.0/8,us,US,First\n"), 0o600); err != nil {
		t.Fatalf("write first geofeed: %v", err)
	}
	if err := os.WriteFile(ptrPath, nil, 0o600); err != nil {
		t.Fatalf("write ptr: %v", err)
	}
	previousGeoFeedPath := viper.Get("geoFeedPath")
	previousPtrPath := viper.Get("ptrPath")
	viper.Set("geoFeedPath", geoFeedPath)
	viper.Set("ptrPath", ptrPath)
	t.Cleanup(func() {
		viper.Set("geoFeedPath", previousGeoFeedPath)
		viper.Set("ptrPath", previousPtrPath)
	})

	cfg := buildTraceConfig(
		3, 0, false, "", "", 0, 1, net.ParseIP("10.0.0.1"), 0,
		30, 50, 300, 3, 3, 18, "en", true, false, "DN42", 1000,
		0, false, 0, false,
	)
	if !cfg.DN42 || cfg.IPGeoSource == nil || cfg.IPGeoDescriptor == nil || cfg.RefreshIPGeoSource == nil {
		t.Fatalf("DN42 config = %+v", cfg)
	}
	if descriptor := cfg.IPGeoDescriptor(); descriptor.Namespace != ipgeo.SourceNamespaceDN42 || !descriptor.HasGeneration {
		t.Fatalf("DN42 descriptor = %+v", descriptor)
	}
	first, err := cfg.IPGeoSource("10.0.0.1", time.Second, "en", false)
	if err != nil || first.City != "First" {
		t.Fatalf("first source = (%+v, %v), want First", first, err)
	}
	if err := os.WriteFile(geoFeedPath, []byte("10.0.0.0/8,us,US,Second City\n"), 0o600); err != nil {
		t.Fatalf("write second geofeed: %v", err)
	}
	stillFirst, err := cfg.IPGeoSource("10.0.0.1", time.Second, "en", false)
	if err != nil || stillFirst.City != "First" {
		t.Fatalf("pinned source = (%+v, %v), want First", stillFirst, err)
	}
	cfg.RefreshIPGeoSource()
	second, err := cfg.IPGeoSource("10.0.0.1", time.Second, "en", false)
	if err != nil || second.City != "Second City" {
		t.Fatalf("refreshed source = (%+v, %v), want Second City", second, err)
	}
}

func TestRunFastTraceModeSkipsRuntimeForGlobalpingFrom(t *testing.T) {
	oldPrepare := prepareNextTraceAPIV4FastIPFn
	oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
	oldRunFastTrace := runFastTraceFn
	var prepareCalls int
	var wsCalls int
	var runCalls int
	prepareNextTraceAPIV4FastIPFn = func(context.Context, bool) error {
		prepareCalls++
		return nil
	}
	newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
		wsCalls++
		return nil
	}
	runFastTraceFn = func(trace.Method, fastTrace.ParamsFastTrace) {
		runCalls++
	}
	t.Cleanup(func() {
		prepareNextTraceAPIV4FastIPFn = oldPrepare
		newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
		runFastTraceFn = oldRunFastTrace
	})
	dataProvider := ipgeo.NextTraceAPIProvider
	disableMaptrace := false
	powProvider := "api.nxtrace.org"

	if runFastTraceModeWithRuntime(context.Background(), false, &dataProvider, &disableMaptrace, &powProvider, "tokyo", true, "", fastTrace.ParamsFastTrace{}, trace.ICMPTrace) {
		t.Fatal("runFastTraceModeWithRuntime returned true for --from, want false")
	}
	if prepareCalls != 0 {
		t.Fatalf("PrepareNextTraceAPIV4FastIP calls = %d, want 0 for --from", prepareCalls)
	}
	if wsCalls != 0 {
		t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 0 for --from", wsCalls)
	}
	if runCalls != 0 {
		t.Fatalf("FastTest calls = %d, want 0 for --from", runCalls)
	}
}

type nextTraceAPIV4TokenPaths struct {
	session string
	latest  string
}

func isolateCmdNextTraceAPIV4TokenFiles(t *testing.T) nextTraceAPIV4TokenPaths {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "")
	return nextTraceAPIV4TokenPaths{
		session: util.NextTraceAPIV4SessionTokenPath(),
		latest:  util.NextTraceAPIV4LatestTokenPath(),
	}
}

func writeNextTraceAPIV4TokenFileForTest(t *testing.T, path, token string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll token dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		t.Fatalf("WriteFile token: %v", err)
	}
}

func TestMaybeRunUninterruptedRawReturnsOnCanceledContext(t *testing.T) {
	oldUninterrupted := util.Uninterrupted
	util.Uninterrupted = true
	defer func() { util.Uninterrupted = oldUninterrupted }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if !maybeRunUninterruptedRaw(true, trace.ICMPTrace, trace.Config{Context: ctx}) {
		t.Fatal("maybeRunUninterruptedRaw returned false, want true when raw loop is active")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("maybeRunUninterruptedRaw returned too slowly after cancel: %v", elapsed)
	}
}

func TestSelectTraceOutputModePriority(t *testing.T) {
	tests := []struct {
		name         string
		tablePrint   bool
		classicPrint bool
		jsonPrint    bool
		rawPrint     bool
		outputPath   string
		want         traceOutputMode
	}{
		{name: "realtime", want: traceOutputRealtime},
		{name: "output", outputPath: "trace.log", want: traceOutputFile},
		{name: "raw over output", rawPrint: true, outputPath: "trace.log", want: traceOutputRaw},
		{name: "classic over raw", classicPrint: true, rawPrint: true, outputPath: "trace.log", want: traceOutputClassic},
		{name: "table over classic", tablePrint: true, classicPrint: true, rawPrint: true, outputPath: "trace.log", want: traceOutputTable},
		{name: "JSON over table and output", tablePrint: true, classicPrint: true, jsonPrint: true, rawPrint: true, outputPath: "trace.log", want: traceOutputJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectTraceOutputMode(tt.tablePrint, tt.classicPrint, tt.jsonPrint, tt.rawPrint, tt.outputPath)
			if got != tt.want {
				t.Fatalf("selectTraceOutputMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTraceOutputPlanStopReasonVisibility(t *testing.T) {
	previousOutput, previousNoColor := color.Output, color.NoColor
	t.Cleanup(func() { color.Output, color.NoColor = previousOutput, previousNoColor })
	color.NoColor = true
	tests := []struct {
		name string
		mode traceOutputMode
		want bool
	}{
		{name: "default", mode: traceOutputRealtime, want: true},
		{name: "table", mode: traceOutputTable, want: true},
		{name: "output", mode: traceOutputFile, want: true},
		{name: "classic", mode: traceOutputClassic},
		{name: "raw", mode: traceOutputRaw},
		{name: "JSON", mode: traceOutputJSON},
	}
	for _, tt := range tests {
		for _, reasonCode := range []string{trace.StopReasonDestination, trace.StopReasonUnreachable, trace.StopReasonMaxHops} {
			for _, hidden := range []bool{false, true} {
				name := tt.name + "/" + reasonCode
				if hidden {
					name += "/hidden"
				}
				t.Run(name, func(t *testing.T) {
					var terminal bytes.Buffer
					color.Output = &terminal
					path := ""
					if tt.mode == traceOutputFile {
						path = filepath.Join(t.TempDir(), "trace.log")
					}
					plan, err := configureTracePrinters(&trace.Config{}, tt.mode, path, hidden)
					if err != nil {
						t.Fatal(err)
					}
					reason := &trace.StopReason{Hop: 5, Reason: reasonCode, Responses: []string{"response"}, Markers: []string{"!H"}}
					res := &trace.Result{StopReason: reason}
					before, err := json.Marshal(res)
					if err != nil {
						t.Fatal(err)
					}
					if err := plan.printStopReason(reason); err != nil {
						t.Fatal(err)
					}
					if err := plan.close(); err != nil {
						t.Fatal(err)
					}
					want := tt.want && !hidden
					if got := strings.Contains(terminal.String(), "Trace Stopped:"); got != want {
						t.Fatalf("terminal stop reason present = %v, want %v; output=%q", got, want, terminal.String())
					}
					if path != "" {
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						if got := strings.Contains(string(data), "Trace Stopped:"); got != want {
							t.Fatalf("file stop reason present = %v, want %v; output=%q", got, want, data)
						}
					}
					after, err := json.Marshal(res)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(before, after) {
						t.Fatalf("structured result changed: %s -> %s", before, after)
					}
				})
			}
		}
	}
}

func TestWriteIgnoredTraceOutputWarning(t *testing.T) {
	tests := []struct {
		name       string
		mode       traceOutputMode
		outputPath string
		want       string
	}{
		{name: "JSON overrides output", mode: traceOutputJSON, outputPath: "trace.log", want: "JSON"},
		{name: "table overrides output", mode: traceOutputTable, outputPath: "trace.log", want: "table"},
		{name: "classic overrides output", mode: traceOutputClassic, outputPath: "trace.log", want: "classic"},
		{name: "raw overrides output", mode: traceOutputRaw, outputPath: "trace.log", want: "raw"},
		{name: "file output active", mode: traceOutputFile, outputPath: "trace.log"},
		{name: "no output requested", mode: traceOutputJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writeIgnoredTraceOutputWarning(&output, tt.mode, tt.outputPath); err != nil {
				t.Fatalf("writeIgnoredTraceOutputWarning() error = %v", err)
			}
			if tt.want == "" {
				if output.Len() != 0 {
					t.Fatalf("warning = %q, want empty", output.String())
				}
				return
			}
			if got := output.String(); !strings.Contains(got, "output file is ignored") || !strings.Contains(got, tt.want) {
				t.Fatalf("warning = %q, want ignored warning for %s", got, tt.want)
			}
		})
	}
}

func TestJSONOutputOverridesFileWithoutSideEffects(t *testing.T) {
	previousOutput := color.Output
	previousNoColor := color.NoColor
	defer func() {
		color.Output = previousOutput
		color.NoColor = previousNoColor
	}()

	var terminal bytes.Buffer
	color.Output = &terminal
	color.NoColor = true
	path := filepath.Join(t.TempDir(), "trace.log")
	mode := selectTraceOutputMode(false, false, true, false, path)
	if mode != traceOutputJSON {
		t.Fatalf("selectTraceOutputMode() = %v, want JSON", mode)
	}

	conf := trace.Config{
		RealtimePrinter: func(*trace.Result, int) {},
		AsyncPrinter:    func(*trace.Result) {},
	}
	plan, err := configureTracePrinters(&conf, mode, path, false)
	if err != nil {
		t.Fatalf("configureTracePrinters() error = %v", err)
	}
	if plan.file != nil {
		t.Fatal("JSON output opened a trace file")
	}
	if conf.RealtimePrinter != nil || conf.AsyncPrinter != nil {
		t.Fatal("JSON output left a human-readable printer enabled")
	}
	if err := plan.printStopReason(&trace.StopReason{Hop: 5, Reason: trace.StopReasonDestination}); err != nil {
		t.Fatalf("printStopReason() error = %v", err)
	}
	if terminal.Len() != 0 {
		t.Fatalf("JSON output wrote a stop footer: %q", terminal.String())
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("JSON output file stat error = %v, want not exist", err)
	}
}

func TestTraceOutputFileWritesPlainStopReason(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	previousOutput := color.Output
	previousNoColor := color.NoColor
	defer func() {
		color.Output = previousOutput
		color.NoColor = previousNoColor
	}()

	var terminal bytes.Buffer
	color.Output = &terminal
	color.NoColor = false
	path := filepath.Join(t.TempDir(), "trace.log")
	conf := trace.Config{}
	plan, err := configureTracePrinters(&conf, traceOutputFile, path, false)
	if err != nil {
		t.Fatalf("configureTracePrinters() error = %v", err)
	}
	if conf.RealtimePrinter == nil {
		t.Fatal("RealtimePrinter = nil, want output printer")
	}
	conf.RealtimePrinter(&trace.Result{Hops: [][]trace.Hop{{}}}, 0)
	if !strings.Contains(terminal.String(), "\x1b[") {
		t.Fatalf("terminal hop output is not styled: %q", terminal.String())
	}
	reason := &trace.StopReason{Hop: 5, Reason: trace.StopReasonDestination, Responses: []string{"ICMP Echo Reply"}}
	if err := plan.printStopReason(reason); err != nil {
		t.Fatalf("printStopReason() error = %v", err)
	}
	if err := plan.close(); err != nil {
		t.Fatalf("close() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(got), "1   *\n") {
		t.Fatalf("output file missing plain hop output: %q", got)
	}
	wantStop := "Trace Stopped: Destination Reached at Hop 5 (ICMP Echo Reply)\n"
	if !strings.Contains(string(got), wantStop) {
		t.Fatalf("output file = %q, want stop line %q", got, wantStop)
	}
	if bytes.Contains(got, []byte("\x1b[")) {
		t.Fatalf("output file contains ANSI escapes: %q", got)
	}
	if !strings.Contains(terminal.String(), "Trace Stopped:") {
		t.Fatalf("terminal missing stop reason: %q", terminal.String())
	}
	if !strings.Contains(terminal.String(), "\x1b[") {
		t.Fatalf("terminal stop reason is not styled: %q", terminal.String())
	}
}

func TestTraceOutputFileStillWritesWhenTerminalFails(t *testing.T) {
	previousOutput := color.Output
	previousNoColor := color.NoColor
	defer func() {
		color.Output = previousOutput
		color.NoColor = previousNoColor
	}()

	color.Output = failingCmdOutputWriter{}
	color.NoColor = true
	path := filepath.Join(t.TempDir(), "trace.log")
	plan, err := configureTracePrinters(&trace.Config{}, traceOutputFile, path, false)
	if err != nil {
		t.Fatalf("configureTracePrinters() error = %v", err)
	}
	reason := &trace.StopReason{Hop: 5, Reason: trace.StopReasonDestination}
	if err := plan.printStopReason(reason); !errors.Is(err, errCmdOutputWriter) {
		t.Fatalf("printStopReason() error = %v, want %v", err, errCmdOutputWriter)
	}
	if err := plan.close(); err != nil {
		t.Fatalf("close() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if want := "Trace Stopped: Destination Reached at Hop 5\n"; string(got) != want {
		t.Fatalf("output file = %q, want %q", got, want)
	}
}

func TestFinalizeTraceResultRoutePathPrintsStopReason(t *testing.T) {
	previousOutput := color.Output
	previousNoColor := color.NoColor
	defer func() {
		color.Output = previousOutput
		color.NoColor = previousNoColor
	}()

	var terminal bytes.Buffer
	color.Output = &terminal
	color.NoColor = true
	res := &trace.Result{
		Hops:       [][]trace.Hop{},
		StopReason: &trace.StopReason{Hop: 5, Reason: trace.StopReasonDestination},
	}
	finalizeTraceResult(
		context.Background(),
		res,
		&traceOutputPlan{mode: traceOutputRealtime},
		false,
		true,
		net.ParseIP("192.0.2.1"),
		true,
		"disable-geoip",
	)
	if !strings.Contains(terminal.String(), "Trace Stopped: Destination Reached at Hop 5") {
		t.Fatalf("route-path output missing stop reason: %q", terminal.String())
	}
}

func TestRegisterGlobalpingFlagWithAvailability_DisabledStillParses(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	from := registerGlobalpingFlagWithAvailability(parser, false)

	if err := parser.Parse([]string{"ntr", "--from", "tokyo"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got := strings.TrimSpace(*from); got != "tokyo" {
		t.Fatalf("--from = %q, want tokyo", got)
	}
}

func TestGlobalpingAvailability(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, location := range []string{"", "tokyo", " "} {
			parser := argparse.NewParser("nexttrace", "")
			from := registerGlobalpingFlagWithAvailability(parser, enabled)
			args := []string{"nexttrace"}
			if location != "" {
				args = append(args, "--from", location)
			}
			if err := parser.Parse(args); err != nil {
				t.Fatal(err)
			}
			err := validateGlobalpingAvailability(*from, enabled)
			if wantError := !enabled && location != ""; (err != nil) != wantError {
				t.Fatalf("enabled=%v location=%q: error=%v", enabled, location, err)
			}
			if err != nil && !strings.Contains(err.Error(), "--from (Globalping) is not available") {
				t.Fatalf("unexpected availability error: %v", err)
			}
		}
	}
}

func TestRegisterWebUIFlagsWithAvailability_DisabledDoesNotRegister(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	flags := registerWebUIFlagsWithAvailability(parser, false)

	if err := parser.Parse([]string{"ntr", "--deploy"}); err == nil {
		t.Fatal("Parse returned nil, want --deploy to be unregistered")
	}
	if *flags.deploy {
		t.Fatal("disabled --deploy pointer should remain false")
	}
	if *flags.mcp {
		t.Fatal("disabled --mcp pointer should remain false")
	}
	if got := strings.TrimSpace(*flags.deployListen); got != "" {
		t.Fatalf("disabled --listen pointer = %q, want empty", got)
	}
	if got := strings.TrimSpace(*flags.deployToken); got != "" {
		t.Fatalf("disabled --deploy-token pointer = %q, want empty", got)
	}
}

func TestRegisterWebUIFlagsWithAvailability_EnabledParsesMCPAndToken(t *testing.T) {
	parser := argparse.NewParser("nexttrace", "")
	flags := registerWebUIFlagsWithAvailability(parser, true)

	if err := parser.Parse([]string{"nexttrace", "--deploy", "--mcp", "--listen", "127.0.0.1:1080", "--deploy-token", "secret"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !*flags.deploy || !*flags.mcp {
		t.Fatalf("deploy=%t mcp=%t, want both true", *flags.deploy, *flags.mcp)
	}
	if got := strings.TrimSpace(*flags.deployListen); got != "127.0.0.1:1080" {
		t.Fatalf("--listen = %q, want 127.0.0.1:1080", got)
	}
	if got := strings.TrimSpace(*flags.deployToken); got != "secret" {
		t.Fatalf("--deploy-token = %q, want secret", got)
	}
}

func TestMCPEndpointURLPrefersAccessAddress(t *testing.T) {
	got := mcpEndpointURL(listenInfo{
		Binding: "http://0.0.0.0:1080",
		Access:  "http://192.0.2.10:1080",
	})
	if got != "http://192.0.2.10:1080/mcp" {
		t.Fatalf("mcpEndpointURL wildcard = %q, want access endpoint", got)
	}

	got = mcpEndpointURL(listenInfo{Binding: "http://127.0.0.1:1080"})
	if got != "http://127.0.0.1:1080/mcp" {
		t.Fatalf("mcpEndpointURL loopback = %q, want binding endpoint", got)
	}
}

func TestValidateDeployMCPModeRequiresDeploy(t *testing.T) {
	if err := validateDeployMCPMode(false, true); err == nil {
		t.Fatal("validateDeployMCPMode(false, true) error = nil, want error")
	}
	if err := validateDeployMCPMode(true, true); err != nil {
		t.Fatalf("validateDeployMCPMode(true, true) error = %v", err)
	}
	if err := validateDeployMCPMode(false, false); err != nil {
		t.Fatalf("validateDeployMCPMode(false, false) error = %v", err)
	}
}

func TestDeployListenRequiresToken(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:1080", false},
		{"[::1]:1080", false},
		{"localhost:1080", false},
		{"0.0.0.0:1080", true},
		{"[::]:1080", true},
		{":1080", true},
		{"192.0.2.10:1080", true},
		{"example.com:1080", true},
	}
	for _, tt := range tests {
		if got := deployListenRequiresToken(tt.addr); got != tt.want {
			t.Fatalf("deployListenRequiresToken(%q) = %t, want %t", tt.addr, got, tt.want)
		}
	}
}

func TestResolveDeployAuthPlan(t *testing.T) {
	oldEnvToken := util.EnvDeployToken
	util.EnvDeployToken = ""
	defer func() { util.EnvDeployToken = oldEnvToken }()

	loopback, err := resolveDeployAuthPlan("127.0.0.1:1080", "")
	if err != nil {
		t.Fatalf("resolveDeployAuthPlan(loopback) error = %v", err)
	}
	if loopback.Enabled {
		t.Fatalf("loopback plan enabled = true, want false")
	}

	external, err := resolveDeployAuthPlan("0.0.0.0:1080", "")
	if err != nil {
		t.Fatalf("resolveDeployAuthPlan(external) error = %v", err)
	}
	if !external.Enabled || !external.AutoGenerated || strings.TrimSpace(external.Token) == "" {
		t.Fatalf("external plan = %+v, want enabled autogenerated token", external)
	}

	manual, err := resolveDeployAuthPlan("127.0.0.1:1080", "manual-token")
	if err != nil {
		t.Fatalf("resolveDeployAuthPlan(manual) error = %v", err)
	}
	if !manual.Enabled || manual.AutoGenerated || manual.Token != "manual-token" {
		t.Fatalf("manual plan = %+v, want manual token auth", manual)
	}

	util.EnvDeployToken = "env-token"
	envPlan, err := resolveDeployAuthPlan("127.0.0.1:1080", "")
	if err != nil {
		t.Fatalf("resolveDeployAuthPlan(env) error = %v", err)
	}
	if !envPlan.Enabled || envPlan.Token != "env-token" {
		t.Fatalf("env plan = %+v, want env token auth", envPlan)
	}

	cliPlan, err := resolveDeployAuthPlan("127.0.0.1:1080", "cli-token")
	if err != nil {
		t.Fatalf("resolveDeployAuthPlan(cli) error = %v", err)
	}
	if cliPlan.Token != "cli-token" {
		t.Fatalf("cli plan token = %q, want cli-token", cliPlan.Token)
	}
}

func TestRegisterTTLIntervalFlagWithMTRSupport_HelpOmitsTracerouteDefault(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	registerTTLIntervalFlagWithMTRSupport(parser, true)

	usage := parser.Usage(nil)
	if strings.Contains(usage, "Default: 300") {
		t.Fatalf("usage should not advertise traceroute default in MTR mode:\n%s", usage)
	}
}

func TestWindowsInitHelpTextMentionsExecutableDirectory(t *testing.T) {
	if got := windowsInitHelpText; !strings.Contains(got, "executable directory") {
		t.Fatalf("init help text = %q, want executable directory", got)
	}
}

func TestApplyTTLIntervalDefault(t *testing.T) {
	ttlInterval := 0
	applyTTLIntervalDefault(&ttlInterval, false, false)
	if ttlInterval != defaultTracerouteTTLIntervalMs {
		t.Fatalf("ttlInterval = %d, want %d", ttlInterval, defaultTracerouteTTLIntervalMs)
	}

	ttlInterval = 0
	applyTTLIntervalDefault(&ttlInterval, false, true)
	if ttlInterval != 0 {
		t.Fatalf("MTR ttlInterval = %d, want 0", ttlInterval)
	}

	ttlInterval = 0
	applyTTLIntervalDefault(&ttlInterval, true, false)
	if ttlInterval != 0 {
		t.Fatalf("explicit ttlInterval = %d, want 0", ttlInterval)
	}
}

func TestQueriesHelpDescribesModeSemantics(t *testing.T) {
	help := buildQueriesHelp()
	for _, want := range []string{"probes per hop", "unlimited", "TUI/raw", "10 with --report/--wide (including --raw)"} {
		if !strings.Contains(help, want) {
			t.Fatalf("queries help missing %q: %s", want, help)
		}
	}
	if enableTraceroute && !strings.Contains(help, "Traceroute: latency samples per hop (default 3)") {
		t.Fatalf("queries help missing traditional semantics: %s", help)
	}
	if !enableTraceroute && !strings.HasPrefix(help, "MTR only: max probes per hop.") {
		t.Fatalf("queries help missing MTR-only semantics: %s", help)
	}
}

func TestQueriesFlagDefaultsAndHelp(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		want     int
		explicit bool
	}{
		{nil, 3, false},
		{[]string{"-q", "0"}, 0, true},
		{[]string{"--queries", "10"}, 10, true},
	} {
		parser := argparse.NewParser(appBinName, "")
		queries := registerQueriesFlag(parser)
		if err := parser.Parse(append([]string{appBinName}, tc.args...)); err != nil {
			t.Fatal(err)
		}
		explicit, _, _, _ := detectExplicitProbeFlags(parser)
		if *queries != tc.want || explicit != tc.explicit {
			t.Fatalf("%v: queries=%d explicit=%v", tc.args, *queries, explicit)
		}
		if strings.Contains(parser.Usage(nil), "Default: 3") {
			t.Fatal("queries help advertises an unconditional default")
		}
	}
}

func TestAdvancedHelpTextMentionsTuningGuidance(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	registerPacketIntervalFlag(parser)
	parser.Int("", "max-attempts", &argparse.Options{Help: buildMaxAttemptsHelp()})
	parser.Int("", "parallel-requests", &argparse.Options{Default: 18, Help: buildParallelRequestsHelp()})
	parser.Int("", "timeout", &argparse.Options{Default: 1000, Help: buildTimeoutHelp()})
	parser.Int("", "psize", &argparse.Options{Help: buildPayloadSizeHelp()})

	usage := parser.Usage(nil)
	wants := []string{
		"load-balanced paths",
		"intercontinental",
		"raise for MTU or",
	}
	if enableTraceroute {
		wants = append(wants, "rate-limited links")
	}
	for _, want := range wants {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing tuning guidance %q:\n%s", want, usage)
		}
	}
}

func TestProbeOptionHelpMentionsRandomPacketSizeAndTOS(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	parser.Int("", "psize", &argparse.Options{Help: buildPayloadSizeHelp()})
	parser.Int("Q", "tos", &argparse.Options{Default: 0, Help: buildTOSHelp()})

	usage := parser.Usage(nil)
	for _, want := range []string{
		"Negative values randomize each probe",
		"type-of-service / traffic class",
		"DSCP*4+ECN",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q:\n%s", want, usage)
		}
	}
}

func TestDetectExplicitProbeFlags(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	parser.Int("q", "queries", &argparse.Options{Default: 3})
	parser.Int("i", "ttl-time", &argparse.Options{Default: 300})
	parser.Int("", "psize", &argparse.Options{})
	parser.Int("Q", "tos", &argparse.Options{Default: 0})

	if err := parser.Parse([]string{"ntr", "--psize", "-123", "-Q", "46", "-q", "5"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	queriesExplicit, ttlTimeExplicit, packetSizeExplicit, tosExplicit := detectExplicitProbeFlags(parser)
	if !queriesExplicit {
		t.Fatal("queriesExplicit = false, want true")
	}
	if ttlTimeExplicit {
		t.Fatal("ttlTimeExplicit = true, want false")
	}
	if !packetSizeExplicit {
		t.Fatal("packetSizeExplicit = false, want true")
	}
	if !tosExplicit {
		t.Fatal("tosExplicit = false, want true")
	}
}

func TestNormalizeNegativePacketSizeArgs(t *testing.T) {
	args := []string{"ntr", "--psize", "-84", "1.1.1.1"}
	got := normalizeNegativePacketSizeArgs(args)
	want := []string{"ntr", "--psize=-84", "1.1.1.1"}

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeDataProviderArgsAcceptsNextTraceAPIAndLegacyAliases(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "canonical mixed case separated short flag",
			args: []string{"nexttrace", "-d", "nExTtRaCe-aPi", "1.1.1.1"},
			want: []string{"nexttrace", "-d", ipgeo.NextTraceAPIProvider, "1.1.1.1"},
		},
		{
			name: "legacy LeoMoeAPI separated long flag",
			args: []string{"nexttrace", "--data-provider", "lEoMoEaPi", "1.1.1.1"},
			want: []string{"nexttrace", "--data-provider", ipgeo.NextTraceAPIProvider, "1.1.1.1"},
		},
		{
			name: "legacy LeoMoe equals long flag",
			args: []string{"nexttrace", "--data-provider=LEOMOE", "1.1.1.1"},
			want: []string{"nexttrace", "--data-provider=" + ipgeo.NextTraceAPIProvider, "1.1.1.1"},
		},
		{
			name: "canonical lowercase equals short flag",
			args: []string{"nexttrace", "-d=nexttrace-api", "1.1.1.1"},
			want: []string{"nexttrace", "-d=" + ipgeo.NextTraceAPIProvider, "1.1.1.1"},
		},
		{
			name: "unrelated provider unchanged",
			args: []string{"nexttrace", "--data-provider=IPInfo", "1.1.1.1"},
			want: []string{"nexttrace", "--data-provider=IPInfo", "1.1.1.1"},
		},
		{
			name: "positional text after separator unchanged",
			args: []string{"nexttrace", "--", "--data-provider=LEOMOE"},
			want: []string{"nexttrace", "--", "--data-provider=LEOMOE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := append([]string(nil), tt.args...)
			got := normalizeDataProviderArgs(tt.args)
			if strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
				t.Fatalf("normalizeDataProviderArgs() = %#v, want %#v", got, tt.want)
			}
			if strings.Join(tt.args, "\x00") != strings.Join(original, "\x00") {
				t.Fatalf("normalizeDataProviderArgs() mutated input to %#v", tt.args)
			}
		})
	}
}

func TestNormalizedNextTraceAPIProviderArgsParseAgainstCanonicalSelector(t *testing.T) {
	for _, args := range [][]string{
		{"nexttrace", "-d", "lEoMoEaPi"},
		{"nexttrace", "--data-provider=LEOMOE"},
		{"nexttrace", "-d=nexttrace-api"},
	} {
		parser := argparse.NewParser("nexttrace", "")
		provider := registerDataProviderFlag(parser)
		if err := parser.Parse(normalizeDataProviderArgs(args)); err != nil {
			t.Fatalf("Parse(%q) error = %v", args, err)
		}
		if *provider != ipgeo.NextTraceAPIProvider {
			t.Fatalf("Parse(%q) provider = %q, want %q", args, *provider, ipgeo.NextTraceAPIProvider)
		}
		usage := parser.Usage(nil)
		if strings.Contains(strings.ToUpper(usage), "LEOMOE") {
			t.Fatalf("Parse(%q) usage exposes legacy provider name: %s", args, usage)
		}
	}
}

func TestDataProviderSelectorAcceptsDN42(t *testing.T) {
	for _, args := range [][]string{
		{"nexttrace", "-d", "DN42"},
		{"nexttrace", "--data-provider=dn42"},
	} {
		parser := argparse.NewParser("nexttrace", "")
		provider := registerDataProviderFlag(parser)
		if err := parser.Parse(args); err != nil {
			t.Fatalf("Parse(%q) error = %v", args, err)
		}
		if !isDN42Provider(*provider) {
			t.Fatalf("Parse(%q) provider = %q, want DN42", args, *provider)
		}
		if usage := parser.Usage(nil); !strings.Contains(usage, "DN42") {
			t.Fatalf("Parse(%q) usage does not mention DN42: %s", args, usage)
		}
	}
}

func TestInitNextTraceAPIRuntimeCanonicalizesLegacyEnvironmentAlias(t *testing.T) {
	isolateCmdNextTraceAPIV4TokenFiles(t)
	oldEnvDataProvider := util.EnvDataProvider
	oldNewNextTraceAPIV3 := newNextTraceAPIV3WebSocketFn
	util.EnvDataProvider = "lEoMoE"
	var wsCalls int
	newNextTraceAPIV3WebSocketFn = func(context.Context) *wshandle.WsConn {
		wsCalls++
		return nil
	}
	t.Cleanup(func() {
		util.EnvDataProvider = oldEnvDataProvider
		newNextTraceAPIV3WebSocketFn = oldNewNextTraceAPIV3
	})

	dataProvider := ipgeo.NextTraceAPIProvider
	powProvider := "api.nxtrace.org"
	_, _ = initNextTraceAPIRuntime(context.Background(), &dataProvider, &powProvider, false)
	if dataProvider != ipgeo.NextTraceAPIProvider {
		t.Fatalf("data provider = %q, want canonical %q", dataProvider, ipgeo.NextTraceAPIProvider)
	}
	if wsCalls != 1 {
		t.Fatalf("NextTrace API v3 WebSocket calls = %d, want 1", wsCalls)
	}
}

func TestSupportsMapTraceRecognizesNextTraceAPIProviderAliases(t *testing.T) {
	for _, provider := range []string{ipgeo.NextTraceAPIProvider, "nExTtRaCe-aPi", "lEoMoEaPi", "LEOMOE", "IPInfo"} {
		if !supportsMapTrace(provider) {
			t.Errorf("supportsMapTrace(%q) = false, want true", provider)
		}
	}
	if supportsMapTrace("IP.SB") {
		t.Fatal("supportsMapTrace(IP.SB) = true, want false")
	}
}

func TestNegativePacketSizeParsesBeforeTarget(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	packetSize := parser.Int("", "psize", &argparse.Options{})
	ipv6Only := parser.Flag("6", "ipv6", &argparse.Options{})
	target := parser.StringPositional(&argparse.Options{})

	args := normalizeNegativePacketSizeArgs([]string{"ntr", "-6", "--psize", "-96", "2606:4700:4700::1111"})
	if err := parser.Parse(args); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !*ipv6Only {
		t.Fatal("-6 should parse as true")
	}
	if *packetSize != -96 {
		t.Fatalf("--psize = %d, want -96", *packetSize)
	}
	if *target != "2606:4700:4700::1111" {
		t.Fatalf("target = %q, want 2606:4700:4700::1111", *target)
	}
}

func TestResolvePacketSizeArg_DefaultsToProtocolMinimum(t *testing.T) {
	got := resolvePacketSizeArg(0, false, trace.TCPTrace, net.ParseIP("2a00:1450:4009:81a::200e"))
	if got != 64 {
		t.Fatalf("resolvePacketSizeArg() = %d, want 64", got)
	}
}

func TestRegisterTracerouteOutputFlagsParsesOutputPath(t *testing.T) {
	if !enableTraceroute {
		t.Skip("normal traceroute output flags are unavailable in the ntr flavor")
	}
	parser := argparse.NewParser("nexttrace", "")
	flags := registerTracerouteOutputFlags(parser)
	target := parser.StringPositional(&argparse.Options{})

	if err := parser.Parse([]string{"nexttrace", "-o", "trace.log", "1.1.1.1"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got := strings.TrimSpace(*flags.outputPath); got != "trace.log" {
		t.Fatalf("--output = %q, want trace.log", got)
	}
	if *flags.outputDefault {
		t.Fatal("--output-default should be false")
	}
	if *target != "1.1.1.1" {
		t.Fatalf("target = %q, want 1.1.1.1", *target)
	}
}

func TestRegisterTracerouteOutputFlagsParsesOutputDefault(t *testing.T) {
	if !enableTraceroute {
		t.Skip("normal traceroute output flags are unavailable in the ntr flavor")
	}
	parser := argparse.NewParser("nexttrace", "")
	flags := registerTracerouteOutputFlags(parser)
	target := parser.StringPositional(&argparse.Options{})

	if err := parser.Parse([]string{"nexttrace", "-O", "1.1.1.1"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !*flags.outputDefault {
		t.Fatal("--output-default should be true")
	}
	if got := strings.TrimSpace(*flags.outputPath); got != "" {
		t.Fatalf("--output = %q, want empty", got)
	}
	if *target != "1.1.1.1" {
		t.Fatalf("target = %q, want 1.1.1.1", *target)
	}
}

func TestResolveOutputPath(t *testing.T) {
	tests := []struct {
		name          string
		outputPath    string
		outputDefault bool
		want          string
		wantErr       string
	}{
		{name: "custom", outputPath: "custom.log", want: "custom.log"},
		{name: "default", outputDefault: true, want: tracelog.DefaultPath},
		{name: "disabled"},
		{name: "conflict", outputPath: "custom.log", outputDefault: true, wantErr: "--output 与 --output-default 不能同时使用"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveOutputPath(tt.outputPath, tt.outputDefault)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveOutputPath returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveOutputPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetFastIPOutputSuppressionRestoresPreviousValue(t *testing.T) {
	orig := util.SuppressFastIPOutput
	util.SuppressFastIPOutput = false
	restore := setFastIPOutputSuppression(true)
	if !util.SuppressFastIPOutput {
		t.Fatal("SuppressFastIPOutput should be true after suppression")
	}
	restore()
	if util.SuppressFastIPOutput != false {
		t.Fatalf("SuppressFastIPOutput = %v, want false", util.SuppressFastIPOutput)
	}
	util.SuppressFastIPOutput = orig
}

func TestResolveConfiguredSrcAddrPrefersExplicitSource(t *testing.T) {
	dstIP := net.ParseIP("1.1.1.1")
	resolved, explicit, err := resolveConfiguredSrcAddr(dstIP, "192.0.2.10", "codex-nonexistent-dev0")
	if err != nil {
		t.Fatalf("resolveConfiguredSrcAddr returned error: %v", err)
	}
	if !explicit {
		t.Fatal("explicit source should be reported as explicit")
	}
	if resolved != "192.0.2.10" {
		t.Fatalf("resolved source = %q, want %q", resolved, "192.0.2.10")
	}
}

func TestShouldForceNoColorForMTUNonTTY(t *testing.T) {
	tests := []struct {
		name        string
		mtuMode     bool
		jsonPrint   bool
		stdoutIsTTY bool
		want        bool
	}{
		{name: "mtu non-tty text", mtuMode: true, jsonPrint: false, stdoutIsTTY: false, want: true},
		{name: "mtu tty text", mtuMode: true, jsonPrint: false, stdoutIsTTY: true, want: false},
		{name: "mtu non-tty json", mtuMode: true, jsonPrint: true, stdoutIsTTY: false, want: false},
		{name: "non-mtu non-tty text", mtuMode: false, jsonPrint: false, stdoutIsTTY: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldForceNoColorForMTUNonTTY(tt.mtuMode, tt.jsonPrint, tt.stdoutIsTTY)
			if got != tt.want {
				t.Fatalf("shouldForceNoColorForMTUNonTTY() = %v, want %v", got, tt.want)
			}
		})
	}
}
