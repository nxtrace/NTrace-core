package trace

import (
	"errors"
	"github.com/xgadget-lab/nexttrace/util"
	"net"
	"os"
	"sync"
	"time"

	"github.com/xgadget-lab/nexttrace/ipgeo"
)

var (
	ErrInvalidMethod      = errors.New("invalid method")
	ErrTracerouteExecuted = errors.New("traceroute already executed")
	ErrHopLimitTimeout    = errors.New("hop timeout")
)

type Config struct {
	SrcAddr          string
	BeginHop         int
	MaxHops          int
	NumMeasurements  int
	ParallelRequests int
	Timeout          time.Duration
	DestIP           net.IP
	DestPort         int
	Quic             bool
	IPGeoSource      ipgeo.Source
	RDns             bool
	PacketInterval   int
	TTLInterval      int
	RealtimePrinter  func(res *Result, ttl int)
	AsyncPrinter     func(res *Result)
}

type Method string

const (
	ICMPTrace Method = "icmp"
	UDPTrace  Method = "udp"
	TCPTrace  Method = "tcp"
)

type Tracer interface {
	Execute() (*Result, error)
}

func Traceroute(method Method, config Config) (*Result, error) {
	var tracer Tracer

	if config.MaxHops == 0 {
		config.MaxHops = 30
	}
	if config.NumMeasurements == 0 {
		config.NumMeasurements = 3
	}
	if config.ParallelRequests == 0 {
		config.ParallelRequests = config.NumMeasurements * 5
	}

	switch method {
	case ICMPTrace:
		if config.DestIP.To4() != nil {
			tracer = &ICMPTracer{Config: config}
		} else {
			tracer = &ICMPTracerv6{Config: config}
		}

	case UDPTrace:
		if config.DestIP.To4() != nil {
			tracer = &UDPTracer{Config: config}
		} else {
			return nil, errors.New("IPv6 UDP Traceroute is not supported")
		}
	case TCPTrace:
		if config.DestIP.To4() != nil {
			tracer = &TCPTracer{Config: config}
		} else {
			// tracer = &TCPTracerv6{Config: config}
			return nil, errors.New("IPv6 TCP Traceroute is not supported")
		}
	default:
		return &Result{}, ErrInvalidMethod
	}
	return tracer.Execute()
}

type Result struct {
	Hops [][]Hop
	lock sync.Mutex
}

func (s *Result) add(hop Hop) {
	s.lock.Lock()
	defer s.lock.Unlock()
	k := hop.TTL - 1
	for len(s.Hops) < hop.TTL {
		s.Hops = append(s.Hops, make([]Hop, 0))
	}
	s.Hops[k] = append(s.Hops[k], hop)

}

func (s *Result) reduce(final int) {
	if final > 0 && final < len(s.Hops) {
		s.Hops = s.Hops[:final]
	}
}

type Hop struct {
	Success  bool
	Address  net.Addr
	Hostname string
	TTL      int
	RTT      time.Duration
	Error    error
	Geo      *ipgeo.IPGeoData
}

func (h *Hop) fetchIPData(c Config) (err error) {
	if c.RDns {
		var rdnsENV = util.GetenvDefault("NEXTTRACE_RDNS", "1")
		if rdnsENV != "1" {
			c.RDns = false
		}
	}
	timeout := time.Millisecond * 2400
	if c.RDns && h.Hostname == "" {
		result := make(chan []string)
		go func() {
			r, err := net.LookupAddr(h.Address.String())
			if err != nil {
				result <- nil
			} else {
				result <- r
			}
		}()
		select {
		case ptr := <-result:
			// process result
			if err == nil && len(ptr) > 0 {
				h.Hostname = ptr[0]
			}
		case <-time.After(timeout):
			// handle timeout
			_ = os.Setenv("NEXTTRACE_RDNS", "0")
		}
	}
	if c.IPGeoSource != nil && h.Geo == nil {
		res := false
		h.Geo, res = ipgeo.Filter(h.Address.String())
		if !res {
			h.Geo, err = c.IPGeoSource(h.Address.String())
		}
	}
	return
}
