package cmd

import (
	"bytes"
	"strings"
	"testing"

	mtutrace "github.com/nxtrace/NTrace-core/trace/mtu"
)

func TestNormalizeMTUProtocolFlagsAutoEnablesUDP(t *testing.T) {
	tcp := false
	udp := false
	if err := normalizeMTUProtocolFlags(&tcp, &udp); err != nil {
		t.Fatalf("normalizeMTUProtocolFlags returned error: %v", err)
	}
	if !udp {
		t.Fatal("udp should be enabled in mtu mode")
	}
}

func TestNormalizeMTUProtocolFlagsRejectsTCP(t *testing.T) {
	tcp := true
	udp := false
	if err := normalizeMTUProtocolFlags(&tcp, &udp); err == nil {
		t.Fatal("expected tcp to be rejected in mtu mode")
	}
}

func TestCheckMTUConflicts(t *testing.T) {
	conflict, ok := checkMTUConflicts([]mtuConflictFlag{
		{flag: "--table", enabled: true},
		{flag: "--from", enabled: true},
	})
	if ok {
		t.Fatal("expected mtu conflict")
	}
	if conflict != "--table" {
		t.Fatalf("conflict = %q, want --table", conflict)
	}
}

func TestPrintMTUResultIncludesPMTUAndSummary(t *testing.T) {
	var buf bytes.Buffer
	res := &mtutrace.Result{
		Target:     "example.com",
		ResolvedIP: "203.0.113.9",
		StartMTU:   1500,
		ProbeSize:  65000,
		PathMTU:    1400,
		Hops: []mtutrace.Hop{
			{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", RTTMs: 12.5, PMTU: 1400},
			{TTL: 2, Event: mtutrace.EventTimeout},
		},
	}
	if err := printMTUResult(&buf, res); err != nil {
		t.Fatalf("printMTUResult returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		"tracepath to example.com (203.0.113.9), start MTU 1500, 65000 byte packets",
		"pmtu 1400",
		"Path MTU: 1400",
		" 2  *",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}
