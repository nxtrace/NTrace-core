package mtu

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

type scriptedStep struct {
	response probeResponse
	err      error
}

type scriptedProber struct {
	steps []scriptedStep
	plans []probePlan
}

func (p *scriptedProber) Probe(_ context.Context, plan probePlan) (probeResponse, error) {
	p.plans = append(p.plans, plan)
	if len(p.steps) == 0 {
		return probeResponse{}, errors.New("unexpected probe")
	}
	step := p.steps[0]
	p.steps = p.steps[1:]
	return step.response, step.err
}

func (p *scriptedProber) Close() error { return nil }

func TestRunWithProberShrinksPMTUAndRetriesSameTTL(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     3,
		Queries:     2,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("192.0.2.1"), RTT: 12 * time.Millisecond}},
			{response: probeResponse{Event: EventFragNeeded, IP: net.ParseIP("198.51.100.1"), RTT: 14 * time.Millisecond, PMTU: 1400}},
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("198.51.100.1"), RTT: 15 * time.Millisecond}},
			{response: probeResponse{Event: EventDestination, IP: cfg.DstIP, RTT: 18 * time.Millisecond}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if res.PathMTU != 1400 {
		t.Fatalf("path mtu = %d, want 1400", res.PathMTU)
	}
	if res.ProbeSize != 65000 {
		t.Fatalf("probe size = %d, want 65000", res.ProbeSize)
	}
	if len(res.Hops) != 3 {
		t.Fatalf("hop count = %d, want 3", len(res.Hops))
	}
	if got := res.Hops[1].PMTU; got != 1400 {
		t.Fatalf("ttl 2 pmtu = %d, want 1400", got)
	}
	if got := prober.plans[0].PayloadSize; got != 64972 {
		t.Fatalf("initial payload size = %d, want 64972", got)
	}
	if got := prober.plans[2].PayloadSize; got != 1372 {
		t.Fatalf("payload size after local mtu shrink = %d, want 1372", got)
	}
}

func TestRunWithProberKeepsLocalPMTUOffHopOutput(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     1,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{err: &localMTUError{MTU: 1400}},
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("192.0.2.1"), RTT: 11 * time.Millisecond}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if res.PathMTU != 1400 {
		t.Fatalf("path mtu = %d, want 1400", res.PathMTU)
	}
	if got := res.Hops[0].PMTU; got != 0 {
		t.Fatalf("local pmtu should not be attributed to hop, got %d", got)
	}
	if got := prober.plans[1].PayloadSize; got != 1372 {
		t.Fatalf("payload size after local mtu shrink = %d, want 1372", got)
	}
}

func TestRunWithProberAnnotatesFirstHopWithLocalStartMTU(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     1,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{err: &localMTUError{MTU: 1500}},
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("192.0.2.1"), RTT: 11 * time.Millisecond}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if len(res.Hops) != 1 {
		t.Fatalf("hop count = %d, want 1", len(res.Hops))
	}
	if got := res.Hops[0].PMTU; got != 1500 {
		t.Fatalf("first hop pmtu = %d, want 1500", got)
	}
}

func TestRunWithProberAnnotatesFirstHopWithStartMTUWithoutLocalEvent(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     1,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("192.0.2.1"), RTT: 11 * time.Millisecond}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if len(res.Hops) != 1 {
		t.Fatalf("hop count = %d, want 1", len(res.Hops))
	}
	if got := res.Hops[0].PMTU; got != 1500 {
		t.Fatalf("first hop pmtu = %d, want 1500", got)
	}
}

func TestRunWithProberStopsTTLAfterFirstNonTimeout(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     3,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("1.1.1.1"), RTT: 10 * time.Millisecond}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if len(prober.plans) != 1 {
		t.Fatalf("probe count = %d, want 1", len(prober.plans))
	}
	if len(res.Hops) != 1 || res.Hops[0].Event != EventTimeExceeded {
		t.Fatalf("unexpected hops: %+v", res.Hops)
	}
}

func TestRunWithProberWritesTimeoutAfterExhaustingQueries(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     2,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{response: probeResponse{Event: EventTimeout}},
			{response: probeResponse{Event: EventTimeout}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if len(prober.plans) != 2 {
		t.Fatalf("probe count = %d, want 2", len(prober.plans))
	}
	if len(res.Hops) != 1 || res.Hops[0].Event != EventTimeout {
		t.Fatalf("unexpected timeout hop: %+v", res.Hops)
	}
}

func TestRunWithProberTimeoutHopDoesNotExposeLocalPMTU(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     1,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{err: &localMTUError{MTU: 1400}},
			{response: probeResponse{Event: EventTimeout}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if len(res.Hops) != 1 {
		t.Fatalf("hop count = %d, want 1", len(res.Hops))
	}
	if got := res.Hops[0].PMTU; got != 0 {
		t.Fatalf("timeout hop pmtu = %d, want 0", got)
	}
}

func TestRunWithProberFallbackShrinksAfterRepeatedLocalMTUErrors(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     1,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{err: &localMTUError{}},
			{err: &localMTUError{}},
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("192.0.2.1"), RTT: 11 * time.Millisecond}},
		},
	}

	res, err := runWithProber(context.Background(), cfg, prober)
	if err != nil {
		t.Fatalf("runWithProber returned error: %v", err)
	}
	if got := len(prober.plans); got != 3 {
		t.Fatalf("probe count = %d, want 3", got)
	}
	if got := prober.plans[0].PayloadSize; got != 64972 {
		t.Fatalf("initial payload size = %d, want 64972", got)
	}
	if got := prober.plans[1].PayloadSize; got != 1472 {
		t.Fatalf("payload size after first local mtu shrink = %d, want 1472", got)
	}
	if got := prober.plans[2].PayloadSize; got != 1471 {
		t.Fatalf("payload size after fallback local mtu shrink = %d, want 1471", got)
	}
	if got := res.PathMTU; got != 1499 {
		t.Fatalf("path mtu = %d, want 1499", got)
	}
	if len(res.Hops) != 1 || res.Hops[0].Event != EventTimeExceeded {
		t.Fatalf("unexpected hops: %+v", res.Hops)
	}
	if got := res.Hops[0].PMTU; got != 1500 {
		t.Fatalf("first hop pmtu = %d, want 1500", got)
	}
}

func TestRunStreamWithProberEmitsOrderedEvents(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     2,
		Queries:     2,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("192.0.2.1"), RTT: 10 * time.Millisecond}},
			{response: probeResponse{Event: EventFragNeeded, IP: net.ParseIP("198.51.100.1"), RTT: 12 * time.Millisecond, PMTU: 1400}},
			{response: probeResponse{Event: EventDestination, IP: cfg.DstIP, RTT: 15 * time.Millisecond}},
		},
	}

	var events []StreamEvent
	res, err := runStreamWithProber(context.Background(), cfg, prober, func(event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("runStreamWithProber returned error: %v", err)
	}
	if res.PathMTU != 1400 {
		t.Fatalf("path mtu = %d, want 1400", res.PathMTU)
	}

	var gotKinds []StreamEventKind
	for _, event := range events {
		gotKinds = append(gotKinds, event.Kind)
	}
	wantKinds := []StreamEventKind{
		StreamEventTTLStart,
		StreamEventTTLUpdate,
		StreamEventTTLFinal,
		StreamEventTTLStart,
		StreamEventTTLUpdate,
		StreamEventTTLUpdate,
		StreamEventTTLFinal,
		StreamEventDone,
	}
	if len(gotKinds) != len(wantKinds) {
		t.Fatalf("event count = %d, want %d (%v)", len(gotKinds), len(wantKinds), gotKinds)
	}
	for i, want := range wantKinds {
		if gotKinds[i] != want {
			t.Fatalf("event[%d] kind = %q, want %q", i, gotKinds[i], want)
		}
	}
	if got := events[6].Hop.PMTU; got != 1400 {
		t.Fatalf("final ttl2 pmtu = %d, want 1400", got)
	}
	if got := events[len(events)-1].PathMTU; got != 1400 {
		t.Fatalf("done path mtu = %d, want 1400", got)
	}
}

func TestRunStreamWithProberEmitsTimeoutFinalEvent(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     2,
		Timeout:     time.Second,
		TTLInterval: 0,
	}

	prober := &scriptedProber{
		steps: []scriptedStep{
			{response: probeResponse{Event: EventTimeout}},
			{response: probeResponse{Event: EventTimeout}},
		},
	}

	var events []StreamEvent
	_, err := runStreamWithProber(context.Background(), cfg, prober, func(event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("runStreamWithProber returned error: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("event count = %d, want 4", len(events))
	}
	if events[0].Kind != StreamEventTTLStart || events[1].Kind != StreamEventTTLUpdate || events[2].Kind != StreamEventTTLFinal || events[3].Kind != StreamEventDone {
		t.Fatalf("unexpected event sequence: %+v", events)
	}
	if got := events[2].Hop.Event; got != EventTimeout {
		t.Fatalf("final timeout event = %q, want %q", got, EventTimeout)
	}
}

func TestRunStreamWithProberEmitsGeoUpdateBeforeFinal(t *testing.T) {
	cfg := Config{
		Target:      "example.com",
		DstIP:       net.ParseIP("203.0.113.9"),
		SrcIP:       net.ParseIP("192.0.2.10"),
		DstPort:     33494,
		BeginHop:    1,
		MaxHops:     1,
		Queries:     1,
		Timeout:     time.Second,
		TTLInterval: 0,
		IPGeoSource: func(ip string, timeout time.Duration, lang string, maptrace bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{Asnumber: "13335", CountryEn: "Hong Kong", Owner: "Cloudflare"}, nil
		},
	}

	var events []StreamEvent
	prober := &scriptedProber{
		steps: []scriptedStep{
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("1.1.1.1"), RTT: 10 * time.Millisecond}},
		},
	}

	res, err := runStreamWithProber(context.Background(), cfg, prober, func(event StreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("runStreamWithProber returned error: %v", err)
	}
	if len(res.Hops) != 1 || res.Hops[0].Geo == nil || res.Hops[0].Geo.Asnumber != "13335" {
		t.Fatalf("unexpected result geo: %+v", res.Hops)
	}
	if len(events) != 5 {
		t.Fatalf("event count = %d, want 5", len(events))
	}
	wantKinds := []StreamEventKind{
		StreamEventTTLStart,
		StreamEventTTLUpdate,
		StreamEventTTLUpdate,
		StreamEventTTLFinal,
		StreamEventDone,
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("event[%d] kind = %q, want %q", i, events[i].Kind, want)
		}
	}
	if events[1].Hop.Geo != nil {
		t.Fatalf("expected first update without geo, got %+v", events[1].Hop.Geo)
	}
	if events[2].Hop.Geo == nil || events[2].Hop.Geo.Asnumber != "13335" {
		t.Fatalf("expected second update with geo, got %+v", events[2].Hop.Geo)
	}
	if events[3].Hop.Geo == nil || events[3].Hop.Geo.Asnumber != "13335" {
		t.Fatalf("expected final event with geo, got %+v", events[3].Hop.Geo)
	}
}

func TestCandidatePathMTUNeverIncreases(t *testing.T) {
	if got := candidatePathMTU(1400, 1500); got != 1400 {
		t.Fatalf("candidatePathMTU should not increase, got %d", got)
	}
}
