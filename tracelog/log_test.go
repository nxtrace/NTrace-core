package tracelog

import (
	"bytes"
	"net"
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
