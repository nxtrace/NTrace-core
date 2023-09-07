package core

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

var (
	ErrInvalidMethod      = errors.New("invalid method")
	ErrTracerouteExecuted = errors.New("traceroute already executed")
	ErrHopLimitTimeout    = errors.New("hop timeout")
)

type Method string

type TraceInstance struct {
	Tracer
	ErrorStr string
}

type Plugin interface {
	OnDNSResolve(domain string) (net.IP, error)
	OnNewIPFound(ip net.Addr) error
	OnTTLChange(ttl int) error
	OnTTLCompleted(ttl int, hop []Hop) error
}

type Config struct {
	TraceMethod      Method
	SrcAddr          string
	BeginHop         int
	MaxHops          int
	NumMeasurements  int
	ParallelRequests int
	Timeout          time.Duration
	DestIP           net.IP
	DestPort         int
	Quic             bool
	PacketInterval   time.Duration
	TTLInterval      time.Duration
	Plugins          []Plugin
}

const (
	ICMPTrace Method = "icmp"
	UDPTrace  Method = "udp"
	TCPTrace  Method = "tcp"
)

type Tracer interface {
	Execute() (*Result, error)
	GetConfig() *Config
	SetConfig(Config)
}

type Result struct {
	Hops [][]Hop
	lock sync.Mutex
}

type Hop struct {
	Address  net.Addr
	Hostname string
	TTL      int
	RTT      time.Duration
	Error    error
}

func Traceroute(p []Plugin) {
	var test_config = Config{
		DestIP:           net.IPv4(1, 1, 1, 1),
		DestPort:         443,
		ParallelRequests: 30,
		NumMeasurements:  3,
		BeginHop:         1,
		MaxHops:          30,
		TTLInterval:      1 * time.Millisecond,
		Timeout:          2 * time.Second,
		TraceMethod:      ICMPTrace,
		Plugins:          p,
	}
	traceInstance, err := NewTracer(test_config)
	if err != nil {
		log.Println(err)
		return
	}

	res, err := traceInstance.Traceroute()
	if err != nil {
		log.Println(err)
	}
	log.Println(res)
}

func NewTracer(config Config) (*TraceInstance, error) {
	t := TraceInstance{}
	switch config.TraceMethod {
	case ICMPTrace:
		if config.DestIP.To4() != nil {
			t.Tracer = &ICMPTracer{Config: config}
		} else {
			t.Tracer = &ICMPTracerv6{Config: config}
		}

	case UDPTrace:
		if config.DestIP.To4() != nil {
			t.Tracer = &UDPTracer{Config: config}
		} else {
			t.Tracer = &UDPTracerv6{Config: config}
		}
	case TCPTrace:
		if config.DestIP.To4() != nil {
			t.Tracer = &TCPTracer{Config: config}
		} else {
			t.Tracer = &TCPTracerv6{Config: config}
		}
	default:
		return &TraceInstance{}, ErrInvalidMethod
	}
	return &t, t.CheckConfig()
}

func (t *TraceInstance) CheckConfig() (err error) {
	c := t.GetConfig()

	configValidConditions := map[string]bool{
		"DestIP is null":            c.DestIP == nil,
		"BeginHop is empty":         c.BeginHop == 0,
		"MaxHops is empty":          c.MaxHops == 0,
		"NumMeasurements is empty":  c.NumMeasurements == 0,
		"ParallelRequests is empty": c.ParallelRequests == 0,
		"Trace Timeout is empty":    c.Timeout == 0,
		"You must specific at least one of TTLInterval and PacketInterval":           c.TTLInterval|c.PacketInterval == 0,
		"You choose " + string(c.TraceMethod) + " trace. DestPort must be specified": (c.TraceMethod == TCPTrace || c.TraceMethod == UDPTrace) && c.DestPort == 0,
	}

	var (
		inValidFlag bool
	)

	for condition, notValid := range configValidConditions {
		if notValid {
			inValidFlag = true
			t.ErrorStr += fmt.Sprintf("Invalid config: %s\n", condition)
		}
	}

	if inValidFlag {
		return fmt.Errorf(t.ErrorStr)
	}

	return nil

}

func (t *TraceInstance) Traceroute() (*Result, error) {
	if t.ErrorStr != "" {
		log.Fatal(t.ErrorStr)
	}
	return t.Tracer.Execute()
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
