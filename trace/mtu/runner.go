package mtu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nxtrace/NTrace-core/util"
)

type prober interface {
	Probe(ctx context.Context, plan probePlan) (probeResponse, error)
	Close() error
}

type probePlan struct {
	TTL         int
	Token       uint32
	PayloadSize int
	Timeout     time.Duration
}

type localMTUError struct {
	MTU int
}

func (e *localMTUError) Error() string {
	if e == nil {
		return "local pmtu update"
	}
	if e.MTU > 0 {
		return fmt.Sprintf("local pmtu update: %d", e.MTU)
	}
	return "local pmtu update"
}

func Run(ctx context.Context, cfg Config) (*Result, error) {
	return RunStream(ctx, cfg, nil)
}

func RunStream(ctx context.Context, cfg Config, sink StreamSink) (*Result, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	p, err := newSocketProber(cfg)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	return runStreamWithProber(ctx, cfg, p, sink)
}

func runWithProber(ctx context.Context, cfg Config, p prober) (*Result, error) {
	return runStreamWithProber(ctx, cfg, p, nil)
}

func runStreamWithProber(ctx context.Context, cfg Config, p prober, sink StreamSink) (*Result, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	startMTU := initialPathMTU(cfg)
	probeMTU := initialProbeMTU(cfg.ipVersion())
	res := &Result{
		Target:     cfg.Target,
		ResolvedIP: cfg.DstIP.String(),
		Protocol:   "udp",
		IPVersion:  cfg.ipVersion(),
		StartMTU:   startMTU,
		ProbeSize:  probeMTU,
		PathMTU:    startMTU,
		Hops:       make([]Hop, 0, cfg.MaxHops-cfg.BeginHop+1),
	}

	var token uint32 = 1
	for ttl := cfg.BeginHop; ttl <= cfg.MaxHops; ttl++ {
		emitStreamEvent(sink, StreamEventTTLStart, res, Hop{TTL: ttl})

		var hop Hop
		gotHop := false
		ttlPMTU := 0
		ttlSawRemote := false

		for attempt := 0; attempt < cfg.Queries; {
			payloadSize := payloadSizeForMTU(probeMTU, res.IPVersion)
			resp, err := p.Probe(ctx, probePlan{
				TTL:         ttl,
				Token:       token,
				PayloadSize: payloadSize,
				Timeout:     cfg.Timeout,
			})
			token++
			if err != nil {
				var mtuErr *localMTUError
				if errors.As(err, &mtuErr) {
					reportedMTU := mtuErr.MTU
					if reportedMTU <= 0 {
						reportedMTU = res.PathMTU
					}
					nextMTU, ok := nextLocalProbeMTU(probeMTU, reportedMTU, res.IPVersion)
					if !ok {
						return nil, err
					}
					if ttl == cfg.BeginHop && probeMTU > res.StartMTU && nextMTU == res.StartMTU {
						ttlPMTU = candidatePathMTU(ttlPMTU, nextMTU)
					}
					probeMTU = nextMTU
					res.PathMTU = candidatePathMTU(res.PathMTU, nextMTU)
					continue
				}
				return nil, err
			}

			attempt++
			if resp.Event == EventTimeout {
				continue
			}

			hop = buildHop(cfg, ttl, resp)
			if resp.Event == EventFragNeeded || resp.Event == EventPacketTooBig {
				ttlSawRemote = true
				ttlPMTU = candidatePathMTU(ttlPMTU, hop.PMTU)
				probeMTU = candidatePathMTU(probeMTU, hop.PMTU)
				res.PathMTU = candidatePathMTU(res.PathMTU, hop.PMTU)
				hop.PMTU = ttlPMTU
				emitStreamEvent(sink, StreamEventTTLUpdate, res, hop)
				gotHop = true
				continue
			}
			if ttlPMTU > 0 {
				hop.PMTU = ttlPMTU
			} else if ttl == 1 && res.ProbeSize > res.StartMTU && res.StartMTU > 0 && res.PathMTU == res.StartMTU {
				hop.PMTU = res.StartMTU
			}
			emitStreamEvent(sink, StreamEventTTLUpdate, res, hop)
			gotHop = true
			break
		}

		if !gotHop {
			hop = Hop{TTL: ttl, Event: EventTimeout}
			emitStreamEvent(sink, StreamEventTTLUpdate, res, hop)
		} else if ttlSawRemote && hop.PMTU == 0 {
			hop.PMTU = ttlPMTU
		}
		if updatedHop, changed := enrichHopMetadata(ctx, cfg, hop); changed {
			hop = updatedHop
			emitStreamEvent(sink, StreamEventTTLUpdate, res, hop)
		}
		res.Hops = append(res.Hops, hop)
		emitStreamEvent(sink, StreamEventTTLFinal, res, hop)

		if hop.Event == EventDestination {
			break
		}
		if ttl < cfg.MaxHops && cfg.TTLInterval > 0 {
			if err := sleepContext(ctx, cfg.TTLInterval); err != nil {
				return nil, err
			}
		}
	}

	res.PathMTU = candidatePathMTU(res.StartMTU, res.PathMTU)
	emitStreamEvent(sink, StreamEventDone, res, Hop{})
	return res, nil
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.DstIP == nil {
		return cfg, errors.New("destination IP is required")
	}
	if cfg.ipVersion() == 0 {
		return cfg, errors.New("destination IP is invalid")
	}
	if cfg.Target == "" {
		cfg.Target = cfg.DstIP.String()
	}
	if cfg.BeginHop < 1 {
		cfg.BeginHop = 1
	}
	if cfg.MaxHops < cfg.BeginHop {
		return cfg, fmt.Errorf("max hops %d is smaller than first hop %d", cfg.MaxHops, cfg.BeginHop)
	}
	if cfg.Queries < 1 {
		cfg.Queries = 1
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}
	if cfg.DstPort == 0 {
		cfg.DstPort = 33494
	}
	if cfg.SrcIP == nil {
		return cfg, errors.New("source IP is required")
	}
	if (cfg.SrcIP.To4() == nil) != (cfg.DstIP.To4() == nil) {
		return cfg, errors.New("source and destination IP address families do not match")
	}
	return cfg, nil
}

func (cfg Config) ipVersion() int {
	if util.IsIPv6(cfg.DstIP) {
		return 6
	}
	if cfg.DstIP.To4() != nil {
		return 4
	}
	return 0
}

func initialPathMTU(cfg Config) int {
	if mtu := util.GetMTUByIPForDevice(cfg.SrcIP, cfg.SourceDevice); mtu > 0 {
		return mtu
	}
	if cfg.ipVersion() == 6 {
		return 1280
	}
	return 1500
}

func initialProbeMTU(ipVersion int) int {
	if ipVersion == 6 {
		return 65000
	}
	return 65000
}

func payloadSizeForMTU(pathMTU, ipVersion int) int {
	overhead := 28
	if ipVersion == 6 {
		overhead = 48
	}
	if payload := pathMTU - overhead; payload > probePayloadMinLen {
		return payload
	}
	return probePayloadMinLen
}

func minProbeMTU(ipVersion int) int {
	if ipVersion == 6 {
		return 48 + probePayloadMinLen
	}
	return 28 + probePayloadMinLen
}

func nextLocalProbeMTU(currentProbeMTU, reportedMTU, ipVersion int) (int, bool) {
	nextMTU := candidatePathMTU(currentProbeMTU, reportedMTU)
	if nextMTU < currentProbeMTU {
		return nextMTU, true
	}
	// Some platforms report EMSGSIZE before exposing a smaller socket MTU.
	if currentProbeMTU <= minProbeMTU(ipVersion) {
		return 0, false
	}
	return currentProbeMTU - 1, true
}

func candidatePathMTU(current, discovered int) int {
	if discovered <= 0 {
		return current
	}
	if current == 0 || discovered < current {
		return discovered
	}
	return current
}

func buildHop(cfg Config, ttl int, resp probeResponse) Hop {
	hop := Hop{
		TTL:   ttl,
		Event: resp.Event,
		PMTU:  resp.PMTU,
	}
	if resp.IP != nil {
		hop.IP = resp.IP.String()
	}
	if resp.RTT > 0 {
		hop.RTTMs = float64(resp.RTT) / float64(time.Millisecond)
	}
	return hop
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func emitStreamEvent(sink StreamSink, kind StreamEventKind, res *Result, hop Hop) {
	if sink == nil || res == nil {
		return
	}
	if hop.TTL == 0 && kind != StreamEventDone {
		hop.TTL = 0
	}
	sink(StreamEvent{
		Kind:       kind,
		TTL:        hop.TTL,
		Hop:        hop,
		Target:     res.Target,
		ResolvedIP: res.ResolvedIP,
		Protocol:   res.Protocol,
		IPVersion:  res.IPVersion,
		StartMTU:   res.StartMTU,
		ProbeSize:  res.ProbeSize,
		PathMTU:    res.PathMTU,
	})
}
