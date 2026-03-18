package printer

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

func testTracerouteTableResult() *trace.Result {
	return &trace.Result{
		Hops: [][]trace.Hop{
			{
				{
					TTL:      1,
					Address:  &net.IPAddr{IP: net.ParseIP("192.0.2.1")},
					Hostname: "router1",
					RTT:      12 * time.Millisecond,
					Geo: &ipgeo.IPGeoData{
						Asnumber:  "13335",
						CountryEn: "Hong Kong",
						Owner:     "Cloudflare",
					},
				},
			},
		},
	}
}

func TestWriteTracerouteTableNonTTYOmitsClearScreenANSI(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = prevNoColor }()

	var buf bytes.Buffer
	writeTracerouteTable(&buf, testTracerouteTableResult(), false)
	output := buf.String()

	if strings.Contains(output, "\033[H\033[2J") {
		t.Fatalf("output should not contain clear-screen ANSI:\n%q", output)
	}
	for _, want := range []string{"Hop", "router1 (192.0.2.1)", "Cloudflare"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%q", want, output)
		}
	}
}

func TestWriteTracerouteTableTTYIncludesClearScreenANSI(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = prevNoColor }()

	var buf bytes.Buffer
	writeTracerouteTable(&buf, testTracerouteTableResult(), true)
	output := buf.String()

	if !strings.HasPrefix(output, "\033[H\033[2J") {
		t.Fatalf("output should start with clear-screen ANSI:\n%q", output)
	}
}
