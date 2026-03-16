package mtu

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	p, err := newSocketProber(cfg)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	return runWithProber(ctx, cfg, p)
}

func runWithProber(ctx context.Context, cfg Config, p prober) (*Result, error) {
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
					nextMTU := candidatePathMTU(probeMTU, reportedMTU)
					if nextMTU == probeMTU {
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
				gotHop = true
				continue
			}
			if ttlPMTU > 0 {
				hop.PMTU = ttlPMTU
			} else if ttl == 1 && res.ProbeSize > res.StartMTU && res.StartMTU > 0 && res.PathMTU == res.StartMTU {
				hop.PMTU = res.StartMTU
			}
			gotHop = true
			break
		}

		if !gotHop {
			hop = Hop{TTL: ttl, Event: EventTimeout}
		} else if ttlSawRemote && hop.PMTU == 0 {
			hop.PMTU = ttlPMTU
		}
		res.Hops = append(res.Hops, hop)

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
		if cfg.RDNS {
			hop.Hostname = reverseLookup(hop.IP)
		}
	}
	if resp.RTT > 0 {
		hop.RTTMs = float64(resp.RTT) / float64(time.Millisecond)
	}
	return hop
}

func reverseLookup(ip string) string {
	ptrs, err := util.LookupAddr(ip)
	if err != nil || len(ptrs) == 0 {
		return ""
	}
	return strings.TrimSuffix(ptrs[0], ".")
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
