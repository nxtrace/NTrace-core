package cmd

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/akamensky/argparse"

	speedconfig "github.com/nxtrace/NTrace-core/internal/speedtest/config"
	"github.com/nxtrace/NTrace-core/internal/speedtest/netx"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/apple"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/cloudflare"
	"github.com/nxtrace/NTrace-core/internal/speedtest/result"
	"github.com/nxtrace/NTrace-core/util"
)

func TestRegisterSpeedFlagWithAvailabilityEnabledAddsSingleHelpEntry(t *testing.T) {
	parser := argparse.NewParser("nexttrace", "")
	registerSpeedFlagWithAvailability(parser, true)

	usage := parser.Usage(nil)
	if !strings.Contains(usage, "--speed") {
		t.Fatalf("usage missing --speed:\n%s", usage)
	}
	if strings.Contains(usage, "--speed-provider") {
		t.Fatalf("main usage should not expose speed-only flags:\n%s", usage)
	}
}

func TestRegisterSpeedFlagWithAvailabilityDisabledDoesNotAcceptSpeed(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	registerSpeedFlagWithAvailability(parser, false)
	if err := parser.Parse([]string{"ntr", "--speed"}); err == nil {
		t.Fatal("Parse() error = nil, want unknown flag when speed mode unavailable")
	}
}

func TestRunSpeedModeHelpOutputsDedicatedUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runSpeedMode([]string{"--speed", "--help"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runSpeedMode(help) rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout.String(), "nexttrace --speed [options]") {
		t.Fatalf("stdout missing dedicated speed usage:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestMaybeRunSpeedModeWithAvailabilityDisabledRejectsSpeed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, rc := maybeRunSpeedModeWithAvailability(false, []string{"--speed"}, &stdout, &stderr)
	if !handled {
		t.Fatal("handled = false, want true")
	}
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(stderr.String(), "--speed is not available") {
		t.Fatalf("stderr = %q, want unavailable message", stderr.String())
	}
}

func TestRunSpeedModeRejectsUnexpectedTarget(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runSpeedMode([]string{"--speed", "1.1.1.1"}, &stdout, &stderr)
	if rc == 0 {
		t.Fatal("runSpeedMode(unexpected target) rc = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr = %q, want unexpected argument error", stderr.String())
	}
}

type fakeSpeedMetadataBackend struct {
	closed bool
}

func (f *fakeSpeedMetadataBackend) Close() {
	f.closed = true
}

func TestStartSpeedMetadataBackendHonorsNoMetadata(t *testing.T) {
	calls := 0
	prev := newSpeedMetadataBackend
	newSpeedMetadataBackend = func(context.Context) speedMetadataBackend {
		calls++
		return &fakeSpeedMetadataBackend{}
	}
	defer func() { newSpeedMetadataBackend = prev }()

	if backend := startSpeedMetadataBackend(context.Background(), &speedconfig.Config{NoMetadata: true}); backend != nil {
		t.Fatalf("backend = %#v, want nil when metadata is disabled", backend)
	}
	if calls != 0 {
		t.Fatalf("metadata backend calls = %d, want 0", calls)
	}
}

func TestStartSpeedMetadataBackendClosesWhenEnabled(t *testing.T) {
	fake := &fakeSpeedMetadataBackend{}
	prev := newSpeedMetadataBackend
	newSpeedMetadataBackend = func(context.Context) speedMetadataBackend {
		return fake
	}
	defer func() { newSpeedMetadataBackend = prev }()

	backend := startSpeedMetadataBackend(context.Background(), &speedconfig.Config{})
	if backend != fake {
		t.Fatalf("backend = %#v, want fake backend", backend)
	}
	closeSpeedMetadataBackend(backend)
	if !fake.closed {
		t.Fatal("metadata backend was not closed")
	}
}

func TestSuppressSpeedMetadataOutputOnlyForJSONMetadata(t *testing.T) {
	orig := util.SuppressFastIPOutput
	defer func() { util.SuppressFastIPOutput = orig }()

	util.SuppressFastIPOutput = false
	restore := suppressSpeedMetadataOutput(&speedconfig.Config{OutputJSON: true})
	if !util.SuppressFastIPOutput {
		t.Fatal("SuppressFastIPOutput = false, want true for JSON metadata")
	}
	restore()
	if util.SuppressFastIPOutput {
		t.Fatal("SuppressFastIPOutput = true after restore, want false")
	}

	restore = suppressSpeedMetadataOutput(&speedconfig.Config{OutputJSON: true, NoMetadata: true})
	if util.SuppressFastIPOutput {
		t.Fatal("SuppressFastIPOutput = true, want unchanged when metadata is disabled")
	}
	restore()
}

func TestRunSpeedModeAppleJSON(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/gm/small":
			time.Sleep(10 * time.Millisecond)
			_, _ = io.WriteString(w, "1")
		case "/api/v1/gm/large":
			time.Sleep(20 * time.Millisecond)
			_, _ = io.WriteString(w, strings.Repeat("a", 64<<10))
		case "/api/v1/gm/slurp":
			time.Sleep(20 * time.Millisecond)
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restoreApple := apple.SetBaseForTest(srv.URL)
	defer restoreApple()
	restoreRoots := netx.SetExtraRootCAsForTest(testServerRootCAs(srv))
	defer restoreRoots()

	u, _ := url.Parse(srv.URL)
	var stdout, stderr bytes.Buffer
	rc := runSpeedMode([]string{"--speed", "--json", "--no-metadata", "--max", "64KiB", "--timeout", "1500", "--threads", "2", "--latency-count", "2", "--endpoint", u.Hostname()}, &stdout, &stderr)
	if rc != 0 && rc != 2 {
		t.Fatalf("runSpeedMode(apple) rc = %d, want 0 or degraded 2, stderr=%s", rc, stderr.String())
	}
	res := assertPureJSONSpeedResult(t, stdout.Bytes(), "apple")
	if res.ExitCode != rc {
		t.Fatalf("JSON exit_code = %d, want rc %d", res.ExitCode, rc)
	}
	if rc == 2 && !res.Degraded {
		t.Fatal("Degraded = false, want true when rc=2")
	}
}

func TestRunSpeedModeCloudflareJSON(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/__down":
			w.Header().Set("cf-meta-ip", "198.51.100.10")
			w.Header().Set("cf-meta-colo", "HKG")
			time.Sleep(10 * time.Millisecond)
			if r.URL.Query().Get("bytes") == "0" {
				_, _ = io.WriteString(w, "0")
				return
			}
			_, _ = io.WriteString(w, strings.Repeat("b", 64<<10))
		case "/__up":
			time.Sleep(20 * time.Millisecond)
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restoreCF := cloudflare.SetBaseForTest(srv.URL)
	defer restoreCF()
	restoreRoots := netx.SetExtraRootCAsForTest(testServerRootCAs(srv))
	defer restoreRoots()

	var stdout, stderr bytes.Buffer
	rc := runSpeedMode([]string{"--speed", "--speed-provider", "cloudflare", "--json", "--no-metadata", "--max", "64KiB", "--timeout", "1500", "--threads", "2", "--latency-count", "2", "--non-interactive"}, &stdout, &stderr)
	if rc != 0 && rc != 2 {
		t.Fatalf("runSpeedMode(cloudflare) rc = %d, want 0 or degraded 2, stderr=%s", rc, stderr.String())
	}
	res := assertPureJSONSpeedResult(t, stdout.Bytes(), "cloudflare")
	if res.ExitCode != rc {
		t.Fatalf("JSON exit_code = %d, want rc %d", res.ExitCode, rc)
	}
	if rc == 2 && !res.Degraded {
		t.Fatal("Degraded = false, want true when rc=2")
	}
}

func assertPureJSONSpeedResult(t *testing.T, data []byte, providerName string) result.RunResult {
	t.Helper()
	if bytes.Contains(data, []byte("preferred API IP")) {
		t.Fatalf("speed JSON output should not contain text noise:\n%s", data)
	}
	var res result.RunResult
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, data)
	}
	if res.Config.Provider != providerName {
		t.Fatalf("provider = %q, want %q", res.Config.Provider, providerName)
	}
	if len(res.Rounds) != 4 {
		t.Fatalf("len(Rounds) = %d, want 4", len(res.Rounds))
	}
	return res
}

func testServerRootCAs(srv *httptest.Server) *x509.CertPool {
	if transport, ok := srv.Client().Transport.(*http.Transport); ok && transport.TLSClientConfig != nil && transport.TLSClientConfig.RootCAs != nil {
		return transport.TLSClientConfig.RootCAs
	}
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return pool
}

func TestContainsSpeedFlagSupportsAssignedFormAndRespectsTerminator(t *testing.T) {
	for _, arg := range []string{"--speed=true", "--speed=True", "--speed=1", "--speed="} {
		if !containsSpeedFlag([]string{arg}) {
			t.Fatalf("containsSpeedFlag(%s) = false, want true", arg)
		}
	}
	for _, arg := range []string{"--speed=false", "--speed=0", "--speed=no"} {
		if containsSpeedFlag([]string{arg}) {
			t.Fatalf("containsSpeedFlag(%s) = true, want false", arg)
		}
	}
	if containsSpeedFlag([]string{"--", "--speed"}) {
		t.Fatal("containsSpeedFlag should ignore --speed after terminator")
	}
}
