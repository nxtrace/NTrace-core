package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
)

func TestPrepareTrace_DoesNotForceLegacyInterval(t *testing.T) {
	setup, statusCode, err := prepareTrace(traceRequest{
		Target:       "1.1.1.1",
		Mode:         "mtr",
		DataProvider: "disable-geoip",
	})
	if err != nil {
		t.Fatalf("prepareTrace returned error: %v (status=%d)", err, statusCode)
	}
	if setup.Req.IntervalMs != 0 {
		t.Fatalf("prepareTrace IntervalMs = %d, want 0", setup.Req.IntervalMs)
	}
}

func TestResolveWebMTRHopInterval_DefaultsToOneSecond(t *testing.T) {
	got := resolveWebMTRHopInterval(traceRequest{})
	if got != time.Second {
		t.Fatalf("resolveWebMTRHopInterval() = %v, want %v", got, time.Second)
	}
}

func TestResolveWebMTRHopInterval_PrefersHopIntervalMs(t *testing.T) {
	got := resolveWebMTRHopInterval(traceRequest{IntervalMs: 2500, HopIntervalMs: 750})
	if got != 750*time.Millisecond {
		t.Fatalf("resolveWebMTRHopInterval() = %v, want %v", got, 750*time.Millisecond)
	}
}

func TestBuildTraceConfig_PropagatesSessionScopedFields(t *testing.T) {
	cfg := buildTraceConfig(traceRequest{
		SourceDevice: "en7",
		DisableMPLS:  true,
		DotServer:    "cloudflare",
	}, net.ParseIP("1.1.1.1"), "IPInfo", 80)

	if cfg.SourceDevice != "en7" {
		t.Fatalf("buildTraceConfig SourceDevice = %q, want en7", cfg.SourceDevice)
	}
	if !cfg.DisableMPLS {
		t.Fatal("buildTraceConfig DisableMPLS = false, want true")
	}
	if cfg.IPGeoSource == nil {
		t.Fatal("buildTraceConfig IPGeoSource = nil, want wrapped source")
	}
}

func TestPrepareTrace_RejectsUnknownSourceDevice(t *testing.T) {
	_, statusCode, err := prepareTrace(traceRequest{
		Target:       "1.1.1.1",
		DataProvider: "disable-geoip",
		SourceDevice: "codex-nonexistent-dev0",
	})
	if err == nil {
		t.Fatal("prepareTrace should reject unknown source_device")
	}
	if statusCode != http.StatusBadRequest {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusBadRequest)
	}
}

func TestNormalizeTarget(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		hasErr bool
	}{
		{name: "empty", input: " ", hasErr: true},
		{name: "url host", input: "https://example.com/path", want: "example.com"},
		{name: "host with port", input: "example.com:8443", want: "example.com"},
		{name: "ipv6 with brackets", input: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "bare ipv6 brackets", input: "[::1]", want: "::1"},
		{name: "malformed reversed brackets", input: "foo]bar[", want: "foo]bar["},
		{name: "malformed open only", input: "[abc", want: "[abc"},
		{name: "malformed close only", input: "abc]", want: "abc]"},
		{name: "slash target", input: "example.com/path", want: "example.com"},
		{name: "invalid slash target", input: "/only-path", hasErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeTarget(tc.input)
			if tc.hasErr {
				if err == nil {
					t.Fatalf("normalizeTarget(%q) error = nil, want error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeTarget(%q) returned error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeTarget(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTraceHandler_RejectsOversizedJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"target":"` + strings.Repeat("a", maxTraceRequestBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/trace", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	traceHandler(c)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestExecuteMTRRaw_PerHopDoesNotMutateSessionGlobals(t *testing.T) {
	oldRunMTRRaw := traceRunMTRRawFn
	defer func() { traceRunMTRRawFn = oldRunMTRRaw }()

	oldSrcDev := util.SrcDev
	oldDisableMPLS := util.DisableMPLS
	oldPowProvider := util.PowProviderParam
	defer func() {
		util.SrcDev = oldSrcDev
		util.DisableMPLS = oldDisableMPLS
		util.PowProviderParam = oldPowProvider
	}()

	util.SrcDev = "keep-dev"
	util.DisableMPLS = false
	util.PowProviderParam = "keep-pow"

	traceRunMTRRawFn = func(_ context.Context, _ trace.Method, cfg trace.Config, opts trace.MTRRawOptions, _ trace.MTRRawOnRecord) error {
		if cfg.SourceDevice != "en7" {
			t.Fatalf("cfg.SourceDevice = %q, want en7", cfg.SourceDevice)
		}
		if !cfg.DisableMPLS {
			t.Fatal("cfg.DisableMPLS = false, want true")
		}
		if opts.HopInterval != time.Second {
			t.Fatalf("opts.HopInterval = %v, want %v", opts.HopInterval, time.Second)
		}
		return nil
	}

	err := executeMTRRaw(context.Background(), &wsTraceSession{}, &traceExecution{
		Req: traceRequest{
			SourceDevice:  "en7",
			DisableMPLS:   true,
			HopIntervalMs: 1000,
			DotServer:     "cloudflare",
		},
		Target: "1.1.1.1",
		Method: trace.ICMPTrace,
		IP:     net.ParseIP("1.1.1.1"),
		Config: trace.Config{
			DstIP:            net.ParseIP("1.1.1.1"),
			SourceDevice:     "en7",
			DisableMPLS:      true,
			IPGeoSource:      nil,
			Timeout:          time.Second,
			MaxHops:          30,
			ParallelRequests: 1,
		},
	}, trace.MTRRawOptions{
		HopInterval: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("executeMTRRaw returned error: %v", err)
	}

	if util.SrcDev != "keep-dev" {
		t.Fatalf("util.SrcDev = %q, want keep-dev", util.SrcDev)
	}
	if util.DisableMPLS {
		t.Fatal("util.DisableMPLS = true, want false")
	}
	if util.PowProviderParam != "keep-pow" {
		t.Fatalf("util.PowProviderParam = %q, want keep-pow", util.PowProviderParam)
	}
}
