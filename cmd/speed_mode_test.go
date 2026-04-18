package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/akamensky/argparse"

	"github.com/nxtrace/NTrace-core/internal/speedtest/netx"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/apple"
	"github.com/nxtrace/NTrace-core/internal/speedtest/provider/cloudflare"
	"github.com/nxtrace/NTrace-core/internal/speedtest/result"
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
	restoreRoots := netx.SetExtraRootCAsForTest(srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs)
	defer restoreRoots()

	u, _ := url.Parse(srv.URL)
	var stdout, stderr bytes.Buffer
	rc := runSpeedMode([]string{"--speed", "--json", "--no-metadata", "--max", "64KiB", "--timeout", "1500", "--threads", "2", "--latency-count", "2", "--endpoint", u.Hostname()}, &stdout, &stderr)
	if rc != 0 && rc != 2 {
		t.Fatalf("runSpeedMode(apple) rc = %d, stderr=%s", rc, stderr.String())
	}
	assertPureJSONSpeedResult(t, stdout.Bytes(), "apple")
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
	restoreRoots := netx.SetExtraRootCAsForTest(srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs)
	defer restoreRoots()

	var stdout, stderr bytes.Buffer
	rc := runSpeedMode([]string{"--speed", "--speed-provider", "cloudflare", "--json", "--no-metadata", "--max", "64KiB", "--timeout", "1500", "--threads", "2", "--latency-count", "2", "--non-interactive"}, &stdout, &stderr)
	if rc != 0 && rc != 2 {
		t.Fatalf("runSpeedMode(cloudflare) rc = %d, stderr=%s", rc, stderr.String())
	}
	assertPureJSONSpeedResult(t, stdout.Bytes(), "cloudflare")
}

func assertPureJSONSpeedResult(t *testing.T, data []byte, providerName string) {
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
}

func TestContainsSpeedFlagSupportsAssignedFormAndRespectsTerminator(t *testing.T) {
	if !containsSpeedFlag([]string{"--speed=true"}) {
		t.Fatal("containsSpeedFlag(--speed=true) = false, want true")
	}
	if containsSpeedFlag([]string{"--", "--speed"}) {
		t.Fatal("containsSpeedFlag should ignore --speed after terminator")
	}
}
