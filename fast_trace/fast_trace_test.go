package fastTrace

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
	"github.com/spf13/viper"
)

func TestTrace(t *testing.T) {
	//pFastTrace := ParamsFastTrace{
	//	SrcDev:         "",
	//	SrcAddr:        "",
	//	BeginHop:       1,
	//	MaxHops:        30,
	//	RDNS:           false,
	//	AlwaysWaitRDNS: false,
	//	Lang:           "",
	//	PktSize:        52,
	//}
	//ft := FastTracer{ParamsFastTrace: pFastTrace}
	//// 建立 WebSocket 连接
	//w := wshandle.New()
	//w.Interrupt = make(chan os.Signal, 1)
	//signal.Notify(w.Interrupt, os.Interrupt)
	//defer func() {
	//	w.Conn.Close()
	//}()
	//fmt.Println("TCP v4")
	//ft.TracerouteMethod = trace.TCPTrace
	//ft.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("TCP v6")
	//ft.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("ICMP v4")
	//ft.TracerouteMethod = trace.ICMPTrace
	//ft.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("ICMP v6")
	//ft.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
}

func TestPromptFastTraceChoiceCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	choice, ok := promptFastTraceChoice(ctx, "请选择选项：", "1")
	if ok {
		t.Fatal("promptFastTraceChoice ok = true, want false for canceled context")
	}
	if choice != "" {
		t.Fatalf("promptFastTraceChoice choice = %q, want empty", choice)
	}
}

func TestPromptFastTraceChoiceDeadlineExceededContext(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	choice, ok := promptFastTraceChoice(ctx, "请选择选项：", "1")
	if ok {
		t.Fatal("promptFastTraceChoice ok = true, want false for deadline exceeded context")
	}
	if choice != "" {
		t.Fatalf("promptFastTraceChoice choice = %q, want empty", choice)
	}
}

func TestReadFastTestv6ChoiceCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	choice, ok := readFastTestv6Choice(ctx)
	if ok {
		t.Fatal("readFastTestv6Choice ok = true, want false for canceled context")
	}
	if choice != "" {
		t.Fatalf("readFastTestv6Choice choice = %q, want empty", choice)
	}
}

func TestFastTraceStopReasonReachesTerminalAndOutputFile(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	previousTraceroute := fastTraceTracerouteFn
	previousLookup := fastTraceDomainLookupFn
	previousOutput := color.Output
	previousNoColor := color.NoColor
	t.Cleanup(func() {
		fastTraceTracerouteFn = previousTraceroute
		fastTraceDomainLookupFn = previousLookup
		color.Output = previousOutput
		color.NoColor = previousNoColor
	})

	fastTraceDomainLookupFn = func(_ context.Context, host, _, _ string, _ bool) (net.IP, error) {
		return net.ParseIP(host), nil
	}
	reason := &trace.StopReason{Hop: 5, Reason: trace.StopReasonDestination, Responses: []string{"ICMP Echo Reply"}}

	tests := []struct {
		name string
		run  func(params ParamsFastTrace)
	}{
		{
			name: "file target",
			run: func(params ParamsFastTrace) {
				runFileTraceTarget(params, trace.ICMPTrace, IpListElement{Ip: "192.0.2.1", Desc: "file target", Version4: true})
			},
		},
		{
			name: "IPv4 interactive target",
			run: func(params ParamsFastTrace) {
				f := FastTracer{TracerouteMethod: trace.ICMPTrace, ParamsFastTrace: params}
				f.tracert("test", ISPCollection{ISPName: "ISP", IP: "192.0.2.1"})
			},
		},
		{
			name: "IPv6 interactive target",
			run: func(params ParamsFastTrace) {
				f := FastTracer{TracerouteMethod: trace.ICMPTrace, ParamsFastTrace: params}
				f.tracert_v6("test", ISPCollection{ISPName: "ISP", IPv6: "2001:db8::1"})
			},
		},
	}

	for _, hidden := range []bool{false, true} {
		for _, tt := range tests {
			name := tt.name
			if hidden {
				name += "/hidden"
			}
			t.Run(name, func(t *testing.T) {
				var terminal bytes.Buffer
				var styledHop bool
				var calls int
				fastTraceTracerouteFn = func(_ trace.Method, conf trace.Config) (*trace.Result, error) {
					calls++
					if conf.RealtimePrinter == nil {
						t.Fatal("RealtimePrinter = nil, want configured output printer")
					}
					before := terminal.Len()
					conf.RealtimePrinter(&trace.Result{Hops: [][]trace.Hop{{}}}, 0)
					styledHop = strings.Contains(terminal.String()[before:], "\x1b[")
					return &trace.Result{StopReason: reason}, nil
				}
				color.Output = &terminal
				color.NoColor = false
				outputPath := filepath.Join(t.TempDir(), "trace.log")

				params := fastTraceTestParams(outputPath)
				params.NoStopReason = hidden
				tt.run(params)
				wantCount := 1
				if hidden {
					wantCount = 0
				}

				if calls != 1 {
					t.Fatalf("Traceroute calls = %d, want 1", calls)
				}
				if !styledHop {
					t.Fatalf("terminal hop output is not styled: %q", terminal.String())
				}
				if got := strings.Count(terminal.String(), "Trace Stopped:"); got != wantCount {
					t.Fatalf("terminal stop reason count = %d, want %d; output=%q", got, wantCount, terminal.String())
				}
				fileOutput, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("ReadFile output: %v", err)
				}
				if got := strings.Count(string(fileOutput), "Trace Stopped:"); got != wantCount {
					t.Fatalf("file stop reason count = %d, want %d; output=%q", got, wantCount, fileOutput)
				}
				if bytes.Contains(fileOutput, []byte("\x1b[")) {
					t.Fatalf("file contains ANSI escapes: %q", fileOutput)
				}
			})
		}
	}
}

func TestFastTraceTracerErrorDoesNotWriteStopReason(t *testing.T) {
	previousTraceroute := fastTraceTracerouteFn
	previousOutput := color.Output
	previousNoColor := color.NoColor
	t.Cleanup(func() {
		fastTraceTracerouteFn = previousTraceroute
		color.Output = previousOutput
		color.NoColor = previousNoColor
	})

	fastTraceTracerouteFn = func(trace.Method, trace.Config) (*trace.Result, error) {
		return nil, errors.New("probe failed")
	}
	var terminal bytes.Buffer
	color.Output = &terminal
	color.NoColor = true
	outputPath := filepath.Join(t.TempDir(), "trace.log")

	runFileTraceTarget(fastTraceTestParams(outputPath), trace.ICMPTrace, IpListElement{Ip: "192.0.2.1", Desc: "file target", Version4: true})

	if strings.Contains(terminal.String(), "Trace Stopped:") {
		t.Fatalf("terminal contains misleading stop reason: %q", terminal.String())
	}
	fileOutput, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile output: %v", err)
	}
	if strings.Contains(string(fileOutput), "Trace Stopped:") {
		t.Fatalf("file contains misleading stop reason: %q", fileOutput)
	}
}

func TestFastTraceNilResultDoesNotPanicOrWriteStopReason(t *testing.T) {
	previousTraceroute := fastTraceTracerouteFn
	previousOutput := color.Output
	previousNoColor := color.NoColor
	t.Cleanup(func() {
		fastTraceTracerouteFn = previousTraceroute
		color.Output = previousOutput
		color.NoColor = previousNoColor
	})

	fastTraceTracerouteFn = func(trace.Method, trace.Config) (*trace.Result, error) {
		return nil, nil
	}
	var terminal bytes.Buffer
	color.Output = &terminal
	color.NoColor = true
	outputPath := filepath.Join(t.TempDir(), "trace.log")

	runFileTraceTarget(fastTraceTestParams(outputPath), trace.ICMPTrace, IpListElement{Ip: "192.0.2.1", Desc: "file target", Version4: true})

	if strings.Contains(terminal.String(), "Trace Stopped:") {
		t.Fatalf("terminal contains misleading stop reason: %q", terminal.String())
	}
}

func TestFileTraceWritesStopReasonForEveryTarget(t *testing.T) {
	previousTraceroute := fastTraceTracerouteFn
	previousOutput := color.Output
	previousNoColor := color.NoColor
	t.Cleanup(func() {
		fastTraceTracerouteFn = previousTraceroute
		color.Output = previousOutput
		color.NoColor = previousNoColor
	})

	for _, hidden := range []bool{false, true} {
		name := "visible"
		if hidden {
			name = "hidden"
		}
		t.Run(name, func(t *testing.T) {
			reasons := []*trace.StopReason{
				{Hop: 2, Reason: trace.StopReasonDestination, Responses: []string{"ICMP Echo Reply"}},
				{Hop: 4, Reason: trace.StopReasonUnreachable, Responses: []string{"ICMP Host Unreachable"}, Markers: []string{"!H"}},
				{Hop: 30, Reason: trace.StopReasonMaxHops},
			}
			var calls int
			fastTraceTracerouteFn = func(trace.Method, trace.Config) (*trace.Result, error) {
				if calls >= len(reasons) {
					t.Fatalf("unexpected traceroute call %d", calls+1)
				}
				reason := reasons[calls]
				calls++
				return &trace.Result{StopReason: reason}, nil
			}

			dir := t.TempDir()
			targetsPath := filepath.Join(dir, "targets.txt")
			if err := os.WriteFile(targetsPath, []byte("192.0.2.1 first\n2001:db8::1 second\n192.0.2.2 third\n"), 0o600); err != nil {
				t.Fatalf("WriteFile targets: %v", err)
			}
			outputPath := filepath.Join(dir, "trace.log")
			var terminal bytes.Buffer
			color.Output = &terminal
			color.NoColor = true

			testFile(ParamsFastTrace{
				Context:         context.Background(),
				File:            targetsPath,
				MaxHops:         30,
				Timeout:         time.Second,
				OutputPath:      outputPath,
				NoStopReason:    hidden,
				RuntimePrepared: true,
			}, trace.ICMPTrace)

			if calls != len(reasons) {
				t.Fatalf("Traceroute calls = %d, want %d", calls, len(reasons))
			}
			wantCount := len(reasons)
			if hidden {
				wantCount = 0
			}
			if got := strings.Count(terminal.String(), "Trace Stopped:"); got != wantCount {
				t.Fatalf("terminal stop reason count = %d, want %d; output=%q", got, wantCount, terminal.String())
			}
			fileOutput, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("ReadFile output: %v", err)
			}
			if got := strings.Count(string(fileOutput), "Trace Stopped:"); got != wantCount {
				t.Fatalf("file stop reason count = %d, want %d; output=%q", got, wantCount, fileOutput)
			}
			if hidden {
				return
			}
			first := strings.Index(string(fileOutput), "Destination Reached at Hop 2")
			second := strings.Index(string(fileOutput), "No Continuing Route Observed at Hop 4")
			if first < 0 || second < 0 || first >= second {
				t.Fatalf("file stop reasons out of order: %q", fileOutput)
			}
		})
	}
}

func fastTraceTestParams(outputPath string) ParamsFastTrace {
	return ParamsFastTrace{
		Context:    context.Background(),
		MaxHops:    30,
		Timeout:    time.Second,
		OutputPath: outputPath,
	}
}

func TestBuildFileTraceConfigUsesPinnedDN42Source(t *testing.T) {
	wantSourceCalls := 0
	source := func(string, time.Duration, string, bool) (*ipgeo.IPGeoData, error) {
		wantSourceCalls++
		return &ipgeo.IPGeoData{Country: "DN42"}, nil
	}
	cfg, err := buildFileTraceConfig(ParamsFastTrace{
		Context:      context.Background(),
		MaxHops:      30,
		Timeout:      time.Second,
		DataProvider: " dn42 ",
		IPGeoSource:  source,
	}, trace.ICMPTrace, IpListElement{Ip: "10.0.0.1", Version4: true})
	if err != nil {
		t.Fatalf("buildFileTraceConfig returned error: %v", err)
	}
	if !cfg.DN42 || cfg.IPGeoSource == nil {
		t.Fatalf("DN42 config = %+v", cfg)
	}
	if cfg.IPGeoDescriptor != nil {
		t.Fatal("custom IPGeoSource should not receive a process-cache descriptor")
	}
	if _, err := cfg.IPGeoSource("10.0.0.1", time.Second, "en", false); err != nil {
		t.Fatalf("pinned source returned error: %v", err)
	}
	if wantSourceCalls != 1 {
		t.Fatalf("source calls = %d, want 1", wantSourceCalls)
	}
}

func TestPinFastTraceGeoSourceUsesEffectiveDN42Provider(t *testing.T) {
	dir := t.TempDir()
	geoFeedPath := filepath.Join(dir, "geofeed.csv")
	ptrPath := filepath.Join(dir, "ptr.csv")
	if err := os.WriteFile(geoFeedPath, []byte("10.0.0.0/8,us,US,DN42 City\n"), 0o600); err != nil {
		t.Fatalf("write geofeed: %v", err)
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

	for _, tt := range []struct {
		name   string
		params ParamsFastTrace
	}{
		{name: "provider", params: ParamsFastTrace{DataProvider: " dn42 "}},
		{name: "flag", params: ParamsFastTrace{DN42: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			params := pinFastTraceGeoSource(tt.params)
			if params.IPGeoSource == nil || params.IPGeoDescriptor == nil || !fastTraceDN42(params) {
				t.Fatalf("pinned params = %+v", params)
			}
			if descriptor := params.IPGeoDescriptor(); descriptor.Namespace != ipgeo.SourceNamespaceDN42 || !descriptor.HasGeneration {
				t.Fatalf("pinned descriptor = %+v", descriptor)
			}
			geo, err := params.IPGeoSource("10.0.0.1", time.Second, "en", false)
			if err != nil || geo.City != "DN42 City" {
				t.Fatalf("pinned source = (%+v, %v), want DN42 City", geo, err)
			}
		})
	}
}

func TestTestFileSkipsFastTraceWSWhenRuntimePrepared(t *testing.T) {
	file := emptyFastTraceFile(t)
	oldInit := initFastTraceWSFn
	oldClose := closeFastTraceWSFn
	var initCalls int
	var closeCalls int
	initFastTraceWSFn = func(context.Context) *wshandle.WsConn {
		initCalls++
		return nil
	}
	closeFastTraceWSFn = func(*wshandle.WsConn) {
		closeCalls++
	}
	t.Cleanup(func() {
		initFastTraceWSFn = oldInit
		closeFastTraceWSFn = oldClose
	})

	testFile(ParamsFastTrace{
		Context:         context.Background(),
		File:            file,
		RuntimePrepared: true,
	}, trace.ICMPTrace)

	if initCalls != 0 {
		t.Fatalf("initFastTraceWS calls = %d, want 0 when runtime is prepared", initCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("closeFastTraceWS calls = %d, want 0 when runtime is prepared", closeCalls)
	}
}

func TestTestFileSkipsFastTraceWSForDN42(t *testing.T) {
	file := emptyFastTraceFile(t)
	oldInit := initFastTraceWSFn
	t.Cleanup(func() { initFastTraceWSFn = oldInit })

	for _, tt := range []struct {
		name   string
		params ParamsFastTrace
	}{
		{name: "provider", params: ParamsFastTrace{DataProvider: "DN42"}},
		{name: "flag", params: ParamsFastTrace{DN42: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var initCalls int
			initFastTraceWSFn = func(context.Context) *wshandle.WsConn {
				initCalls++
				return nil
			}
			params := tt.params
			params.Context = context.Background()
			params.File = file
			testFile(params, trace.ICMPTrace)

			if initCalls != 0 {
				t.Fatalf("initFastTraceWS calls = %d, want 0 for DN42", initCalls)
			}
		})
	}
}

func TestTestFileInitializesFastTraceWSByDefault(t *testing.T) {
	isolateFastTraceNextTraceAPIV4TokenFiles(t)
	file := emptyFastTraceFile(t)
	oldInit := initFastTraceWSFn
	oldClose := closeFastTraceWSFn
	var initCalls int
	var closeCalls int
	initFastTraceWSFn = func(context.Context) *wshandle.WsConn {
		initCalls++
		return nil
	}
	closeFastTraceWSFn = func(*wshandle.WsConn) {
		closeCalls++
	}
	t.Cleanup(func() {
		initFastTraceWSFn = oldInit
		closeFastTraceWSFn = oldClose
	})

	testFile(ParamsFastTrace{
		Context: context.Background(),
		File:    file,
	}, trace.ICMPTrace)

	if initCalls != 1 {
		t.Fatalf("initFastTraceWS calls = %d, want 1 by default", initCalls)
	}
	if closeCalls != 1 {
		t.Fatalf("closeFastTraceWS calls = %d, want 1 by default", closeCalls)
	}
}

func TestTestFileSkipsFastTraceWSWhenAPIV4TokenConfigured(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "v4-token")
	file := emptyFastTraceFile(t)
	oldInit := initFastTraceWSFn
	oldClose := closeFastTraceWSFn
	var initCalls int
	var closeCalls int
	initFastTraceWSFn = func(context.Context) *wshandle.WsConn {
		initCalls++
		return nil
	}
	closeFastTraceWSFn = func(*wshandle.WsConn) {
		closeCalls++
	}
	t.Cleanup(func() {
		initFastTraceWSFn = oldInit
		closeFastTraceWSFn = oldClose
	})

	testFile(ParamsFastTrace{
		Context: context.Background(),
		File:    file,
	}, trace.ICMPTrace)

	if initCalls != 0 {
		t.Fatalf("initFastTraceWS calls = %d, want 0 when API v4 token is configured", initCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("closeFastTraceWS calls = %d, want 0 when API v4 token is configured", closeCalls)
	}
}

func emptyFastTraceFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "targets.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile targets: %v", err)
	}
	return path
}

func isolateFastTraceNextTraceAPIV4TokenFiles(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "")
}
