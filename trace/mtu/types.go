package mtu

import (
	"net"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

type Event string

const (
	EventTimeExceeded Event = "time_exceeded"
	EventPacketTooBig Event = "packet_too_big"
	EventFragNeeded   Event = "frag_needed"
	EventDestination  Event = "destination"
	EventTimeout      Event = "timeout"
)

type StreamEventKind string

const (
	StreamEventTTLStart  StreamEventKind = "ttl_start"
	StreamEventTTLUpdate StreamEventKind = "ttl_update"
	StreamEventTTLFinal  StreamEventKind = "ttl_final"
	StreamEventDone      StreamEventKind = "done"
)

type Config struct {
	Target         string
	DstIP          net.IP
	SrcIP          net.IP
	SourceDevice   string
	SrcPort        int
	DstPort        int
	BeginHop       int
	MaxHops        int
	Queries        int
	Timeout        time.Duration
	TTLInterval    time.Duration
	RDNS           bool
	AlwaysWaitRDNS bool
	IPGeoSource    ipgeo.Source
	Lang           string
}

type Hop struct {
	TTL      int              `json:"ttl"`
	Event    Event            `json:"event"`
	IP       string           `json:"ip,omitempty"`
	Hostname string           `json:"hostname,omitempty"`
	RTTMs    float64          `json:"rtt_ms,omitempty"`
	PMTU     int              `json:"pmtu,omitempty"`
	Geo      *ipgeo.IPGeoData `json:"geo,omitempty"`
}

type Result struct {
	Target     string `json:"target"`
	ResolvedIP string `json:"resolved_ip"`
	Protocol   string `json:"protocol"`
	IPVersion  int    `json:"ip_version"`
	StartMTU   int    `json:"start_mtu"`
	ProbeSize  int    `json:"probe_size"`
	PathMTU    int    `json:"path_mtu"`
	Hops       []Hop  `json:"hops"`
}

type StreamEvent struct {
	Kind       StreamEventKind `json:"kind"`
	TTL        int             `json:"ttl,omitempty"`
	Hop        Hop             `json:"hop,omitempty"`
	Target     string          `json:"target"`
	ResolvedIP string          `json:"resolved_ip"`
	Protocol   string          `json:"protocol"`
	IPVersion  int             `json:"ip_version"`
	StartMTU   int             `json:"start_mtu"`
	ProbeSize  int             `json:"probe_size"`
	PathMTU    int             `json:"path_mtu"`
}

type StreamSink func(StreamEvent)
