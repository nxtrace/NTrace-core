package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/nxtrace/NTrace-core/ipgeo"
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

func TestBuildMTUConflictFlagsIncludesOutputDefault(t *testing.T) {
	flags := buildMTUConflictFlags(false, false, effectiveMTRModes{}, false, false, false, false, true, false, false, "", "", false)
	conflict, ok := checkMTUConflicts(flags)
	if ok {
		t.Fatal("expected mtu conflict")
	}
	if conflict != "--output-default" {
		t.Fatalf("conflict = %q, want --output-default", conflict)
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
			{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", RTTMs: 12.5, PMTU: 1400, Geo: &ipgeo.IPGeoData{Asnumber: "13335", CountryEn: "Hong Kong", Owner: "Cloudflare"}},
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
		"AS13335",
		"Cloudflare",
		"Path MTU: 1400",
		" 2  *",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestFormatMTUHopSnapshotStartPlaceholder(t *testing.T) {
	line := formatMTUHopSnapshot(mtutrace.StreamEvent{
		Kind: mtutrace.StreamEventTTLStart,
		TTL:  3,
	})
	if line != " 3  ..." {
		t.Fatalf("line = %q, want %q", line, " 3  ...")
	}
}

func TestMTUStreamRendererTTYRewritesCurrentLine(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = prevNoColor }()

	var buf bytes.Buffer
	renderer := newMTUStreamRenderer(&buf, true)
	events := []mtutrace.StreamEvent{
		{
			Kind:       mtutrace.StreamEventTTLStart,
			TTL:        1,
			Target:     "example.com",
			ResolvedIP: "203.0.113.9",
			StartMTU:   1500,
			ProbeSize:  65000,
		},
		{
			Kind: mtutrace.StreamEventTTLUpdate,
			TTL:  1,
			Hop:  mtutrace.Hop{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", RTTMs: 12.5, PMTU: 1500},
		},
		{
			Kind: mtutrace.StreamEventTTLUpdate,
			TTL:  1,
			Hop:  mtutrace.Hop{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", RTTMs: 12.5, PMTU: 1500, Geo: &ipgeo.IPGeoData{Asnumber: "13335", CountryEn: "Hong Kong", Owner: "Cloudflare"}},
		},
		{
			Kind: mtutrace.StreamEventTTLFinal,
			TTL:  1,
			Hop:  mtutrace.Hop{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", RTTMs: 12.5, PMTU: 1500, Geo: &ipgeo.IPGeoData{Asnumber: "13335", CountryEn: "Hong Kong", Owner: "Cloudflare"}},
		},
		{
			Kind:    mtutrace.StreamEventDone,
			PathMTU: 1500,
		},
	}

	for _, event := range events {
		if err := renderer.Render(event); err != nil {
			t.Fatalf("Render returned error: %v", err)
		}
	}

	output := buf.String()
	for _, want := range []string{
		"tracepath to example.com (203.0.113.9), start MTU 1500, 65000 byte packets\n",
		"\r\x1b[2K 1  ...",
		"\r\x1b[2K 1  192.0.2.1  12.50ms  pmtu 1500  AS13335",
		"Cloudflare\n",
		"Path MTU: 1500\n",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%q", want, output)
		}
	}
}

func TestMTUStreamRendererTTYAppliesColorsWhenEnabled(t *testing.T) {
	prevNoColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = prevNoColor }()

	var buf bytes.Buffer
	renderer := newMTUStreamRenderer(&buf, true)
	events := []mtutrace.StreamEvent{
		{
			Kind:       mtutrace.StreamEventTTLStart,
			TTL:        1,
			Target:     "example.com",
			ResolvedIP: "203.0.113.9",
			StartMTU:   1500,
			ProbeSize:  65000,
		},
		{
			Kind: mtutrace.StreamEventTTLFinal,
			TTL:  1,
			Hop:  mtutrace.Hop{TTL: 1, Event: mtutrace.EventDestination, IP: "203.0.113.9", RTTMs: 10.5, PMTU: 1480},
		},
		{
			Kind:    mtutrace.StreamEventDone,
			PathMTU: 1480,
		},
	}

	for _, event := range events {
		if err := renderer.Render(event); err != nil {
			t.Fatalf("Render returned error: %v", err)
		}
	}

	output := buf.String()
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("output should contain ANSI color codes:\n%q", output)
	}
	for _, want := range []string{"tracepath to example.com", "pmtu 1480", "Path MTU: 1480"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%q", want, output)
		}
	}
}

func TestMTUStreamRendererNonTTYPrintsOnlyFinalLines(t *testing.T) {
	var buf bytes.Buffer
	renderer := newMTUStreamRenderer(&buf, false)
	events := []mtutrace.StreamEvent{
		{
			Kind:       mtutrace.StreamEventTTLStart,
			TTL:        1,
			Target:     "example.com",
			ResolvedIP: "203.0.113.9",
			StartMTU:   1500,
			ProbeSize:  65000,
		},
		{
			Kind: mtutrace.StreamEventTTLUpdate,
			TTL:  1,
			Hop:  mtutrace.Hop{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", RTTMs: 12.5, PMTU: 1500},
		},
		{
			Kind: mtutrace.StreamEventTTLFinal,
			TTL:  1,
			Hop:  mtutrace.Hop{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", RTTMs: 12.5, PMTU: 1500, Geo: &ipgeo.IPGeoData{Asnumber: "13335", CountryEn: "Hong Kong", Owner: "Cloudflare"}},
		},
		{
			Kind:    mtutrace.StreamEventDone,
			PathMTU: 1500,
		},
	}

	for _, event := range events {
		if err := renderer.Render(event); err != nil {
			t.Fatalf("Render returned error: %v", err)
		}
	}

	output := buf.String()
	for _, unwanted := range []string{"\x1b[2K", " 1  ..."} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("output should not contain %q:\n%q", unwanted, output)
		}
	}
	for _, want := range []string{
		"tracepath to example.com (203.0.113.9), start MTU 1500, 65000 byte packets\n",
		" 1  192.0.2.1  12.50ms  pmtu 1500  AS13335",
		"Cloudflare\n",
		"Path MTU: 1500\n",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%q", want, output)
		}
	}
}

func TestMTUResultJSONIncludesGeo(t *testing.T) {
	res := &mtutrace.Result{
		Target:     "example.com",
		ResolvedIP: "203.0.113.9",
		StartMTU:   1500,
		ProbeSize:  65000,
		PathMTU:    1400,
		Hops: []mtutrace.Hop{
			{TTL: 1, Event: mtutrace.EventTimeExceeded, IP: "192.0.2.1", Geo: &ipgeo.IPGeoData{Country: "中国香港", Owner: "Cloudflare"}},
		},
	}

	encoded, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	output := string(encoded)
	for _, want := range []string{`"geo":`, `"country":"中国香港"`, `"owner":"Cloudflare"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("json output missing %q:\n%s", want, output)
		}
	}
}
