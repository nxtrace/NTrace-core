package trace

import (
	"context"
	"net"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

// ---------------------------------------------------------------------------
// Per-hop independent scheduler (CLI MTR mode)
// ---------------------------------------------------------------------------

// mtrProbeResult holds the outcome of a single TTL probe.
type mtrProbeResult struct {
	TTL      int
	Success  bool
	Addr     net.Addr
	RTT      time.Duration
	MPLS     []string
	Hostname string           // pre-resolved PTR (fallback prober)
	Geo      *ipgeo.IPGeoData // pre-resolved geo  (fallback prober)
}

// mtrTTLProber abstracts single-TTL probing for the per-hop scheduler.
type mtrTTLProber interface {
	// ProbeTTL sends a probe at the given TTL and blocks until response or timeout.
	ProbeTTL(ctx context.Context, ttl int) (mtrProbeResult, error)
	// Reset invalidates in-flight probes and clears internal caches (e.g. knownFinalTTL).
	Reset() error
	// Close releases underlying resources (sockets etc.).
	Close() error
}

// mtrSchedulerConfig configures the per-hop scheduler.
type mtrSchedulerConfig struct {
	BeginHop          int
	MaxHops           int
	HopInterval       time.Duration // delay between successive probes to the same TTL
	Timeout           time.Duration // per-probe timeout; used to compute default MaxInFlightPerHop
	MaxPerHop         int           // 0 = unlimited (run until ctx cancelled)
	MaxConsecErrors   int           // per-TTL consecutive error limit; 0 → default 10
	MaxInFlightPerHop int           // max concurrent probes per TTL; 0 → ceil(Timeout/HopInterval)+1
	ParallelRequests  int
	ProgressThrottle  time.Duration
	FillGeo           bool
	AsyncMetadata     bool
	BaseConfig        Config // used for geo/RDNS lookup
	DstIP             net.IP

	IsPaused         func() bool
	IsResetRequested func() bool
}

// mtrHopState tracks per-TTL scheduling state.
type mtrHopState struct {
	completed       int
	inFlightCount   int
	nextAt          time.Time
	disabled        bool
	consecutiveErrs int
}

// mtrCompletedProbe wraps a finished probe for the result channel.
type mtrCompletedProbe struct {
	ttl    int
	result mtrProbeResult
	gen    uint64
	doneAt time.Time
	err    error
}

// runMTRScheduler runs the per-hop independent scheduling loop.
//
// Each TTL is probed independently: after a probe completes, the next probe for
// that TTL is scheduled after HopInterval. Concurrency across TTLs is limited by
// ParallelRequests. Iteration is defined as min(Snt) over active TTLs.
//
// onSnapshot is called periodically with aggregated stats (for TUI / report).
// onProbe is called per completed probe (for raw streaming mode).
func runMTRScheduler(
	ctx context.Context,
	prober mtrTTLProber,
	agg *MTRAggregator,
	cfg mtrSchedulerConfig,
	onSnapshot MTROnSnapshot,
	onProbe func(result mtrProbeResult, iteration int),
) error {
	defer prober.Close()
	rt, err := newMTRSchedulerRuntime(ctx, prober, agg, cfg, onSnapshot, onProbe)
	if err != nil {
		return err
	}
	return rt.run()
}

// mtrAddrToIP extracts net.IP from net.Addr.
func mtrAddrToIP(addr net.Addr) net.IP {
	if addr == nil {
		return nil
	}
	switch a := addr.(type) {
	case *net.IPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	case *net.TCPAddr:
		return a.IP
	}
	return nil
}

func mtrAddrString(addr net.Addr) string {
	if ip := mtrAddrToIP(addr); ip != nil {
		return ip.String()
	}
	if addr == nil {
		return ""
	}
	return addr.String()
}
