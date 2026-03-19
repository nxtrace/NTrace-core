package tracelog

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

func testTraceLogResult() *trace.Result {
	return &trace.Result{
		Hops: [][]trace.Hop{
			{
				{
					TTL:      1,
					Address:  &net.IPAddr{IP: net.ParseIP("192.0.2.1")},
					Hostname: "router1",
					RTT:      12 * time.Millisecond,
					Geo: &ipgeo.IPGeoData{
						Asnumber: "13335",
						Country:  "中国香港",
						Owner:    "Cloudflare",
					},
				},
			},
		},
	}
}

func TestWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHeader(&buf, "header\n"); err != nil {
		t.Fatalf("WriteHeader returned error: %v", err)
	}
	if got := buf.String(); got != "header\n" {
		t.Fatalf("header = %q, want %q", got, "header\n")
	}
}

func TestWriteRealtimeUsesProvidedWriter(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteRealtime(&buf, testTraceLogResult(), 0); err != nil {
		t.Fatalf("WriteRealtime returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{"1", "192.0.2.1", "AS13335", "Cloudflare", "12.00 ms"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%q", want, output)
		}
	}
}

func TestNewRealtimePrinterWrapsWriter(t *testing.T) {
	var buf bytes.Buffer
	printer := NewRealtimePrinter(&buf)
	printer(testTraceLogResult(), 0)
	if buf.Len() == 0 {
		t.Fatal("expected writer to receive trace output")
	}
}

func captureStdIO(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	stdoutBytes, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(stdoutBytes), string(stderrBytes)
}

func TestDefaultPathUsesTempDir(t *testing.T) {
	want := filepath.Join(os.TempDir(), "trace.log")
	if DefaultPath != want {
		t.Fatalf("DefaultPath = %q, want %q", DefaultPath, want)
	}
}

func TestRealtimePrinterFallsBackToStdoutWhenOpenFails(t *testing.T) {
	oldDefaultPath := DefaultPath
	DefaultPath = t.TempDir()
	defer func() { DefaultPath = oldDefaultPath }()

	stdout, stderr := captureStdIO(t, func() {
		RealtimePrinter(testTraceLogResult(), 0)
	})

	if !strings.Contains(stdout, "192.0.2.1") {
		t.Fatalf("stdout missing realtime output:\n%q", stdout)
	}
	if !strings.Contains(stderr, "open trace log") {
		t.Fatalf("stderr missing open failure:\n%q", stderr)
	}
}
