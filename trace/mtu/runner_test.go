package mtu

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
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
			{err: &localMTUError{MTU: 1400}},
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
	if len(res.Hops) != 3 {
		t.Fatalf("hop count = %d, want 3", len(res.Hops))
	}
	if got := res.Hops[1].PMTU; got != 1400 {
		t.Fatalf("ttl 2 pmtu = %d, want 1400", got)
	}
	if got := prober.plans[0].PayloadSize; got != 1472 {
		t.Fatalf("initial payload size = %d, want 1472", got)
	}
	if got := prober.plans[2].PayloadSize; got != 1372 {
		t.Fatalf("payload size after local mtu shrink = %d, want 1372", got)
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
			{response: probeResponse{Event: EventTimeExceeded, IP: net.ParseIP("192.0.2.1"), RTT: 10 * time.Millisecond}},
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

func TestRunWithProberKeepsLocalPMTUOnTimeoutHop(t *testing.T) {
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
	if got := res.Hops[0].PMTU; got != 1400 {
		t.Fatalf("timeout hop pmtu = %d, want 1400", got)
	}
}

func TestCandidatePathMTUNeverIncreases(t *testing.T) {
	if got := candidatePathMTU(1400, 1500); got != 1400 {
		t.Fatalf("candidatePathMTU should not increase, got %d", got)
	}
}
