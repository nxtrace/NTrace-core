package trace

import (
	"errors"
	"net"
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
	AlwaysWaitRDNS   bool
	PacketInterval   int
	TTLInterval      int
	Lang             string
	DN42             bool
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
			tracer = &TCPTracerv6{Config: config}
			// return nil, errors.New("IPv6 TCP Traceroute is not supported")
		}
	default:
		return &Result{}, ErrInvalidMethod
	}
	return tracer.Execute()
}

type Result struct {
	Hops        [][]Hop
	lock        sync.Mutex
	TraceMapUrl string
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
	Lang     string
}

func (h *Hop) fetchIPData(c Config) (err error) {
	// DN42
	if c.DN42 {
		var ip string
		// 首先查找 PTR 记录
		r, err := net.LookupAddr(h.Address.String())
		if err == nil && len(r) > 0 {
			h.Hostname = r[0][:len(r[0])-1]
			ip = h.Address.String() + "," + h.Hostname
		}
		h.Geo, err = c.IPGeoSource(ip)
		return nil
	}

	// Debug Area
	// c.AlwaysWaitRDNS = true

	// Initialize a rDNS Channel
	rDNSChan := make(chan []string)
	fetchDoneChan := make(chan bool)

	if c.RDns && h.Hostname == "" {
		// Create a rDNS Query go-routine
		go func() {
			r, err := net.LookupAddr(h.Address.String())
			if err != nil {
				// No PTR Record
				rDNSChan <- nil
			} else {
				// One PTR Record is found
				rDNSChan <- r
			}
		}()
	}

	// Create Data Fetcher go-routine
	go func() {
		// Start to fetch IP Geolocation data
		if c.IPGeoSource != nil && h.Geo == nil {
			res := false
			h.Lang = c.Lang
			h.Geo, res = ipgeo.Filter(h.Address.String())
			if !res {
				h.Geo, err = c.IPGeoSource(h.Address.String())
			}
		}
		// Fetch Done
		fetchDoneChan <- true
	}()

	// Select Close Flag
	var selectClose bool
	if !c.AlwaysWaitRDNS {
		select {
		case <-fetchDoneChan:
			// When fetch done signal recieved, stop waiting PTR record
		case ptr := <-rDNSChan:
			// process result
			if err == nil && len(ptr) > 0 {
				h.Hostname = ptr[0][:len(ptr[0])-1]
			}
			selectClose = true
		}
	} else {
		select {
		case ptr := <-rDNSChan:
			// process result
			if err == nil && len(ptr) > 0 {
				h.Hostname = ptr[0]
			}
		// 1 second timeout
		case <-time.After(time.Second * 1):
		}
		selectClose = true
	}

	// When Select Close, fetchDoneChan Reciever will also be closed
	if selectClose {
		// New a reciever to prevent channel congestion
		<-fetchDoneChan
	}

	return
}
