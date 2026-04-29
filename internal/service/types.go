package service

import (
	"time"

	speedresult "github.com/nxtrace/NTrace-core/internal/speedtest/result"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	mtutrace "github.com/nxtrace/NTrace-core/trace/mtu"
)

type CapabilitiesRequest struct{}

type ParameterBoundaries struct {
	Supported       []string `json:"supported"`
	NotApplicable   []string `json:"not_applicable,omitempty"`
	NotYetSupported []string `json:"not_yet_supported,omitempty"`
}

type Attempt struct {
	Success  bool             `json:"success"`
	IP       string           `json:"ip,omitempty"`
	Hostname string           `json:"hostname,omitempty"`
	RTTMs    float64          `json:"rtt_ms,omitempty"`
	Error    string           `json:"error,omitempty"`
	MPLS     []string         `json:"mpls,omitempty"`
	Geo      *ipgeo.IPGeoData `json:"geo,omitempty"`
}

type Hop struct {
	TTL      int       `json:"ttl"`
	Attempts []Attempt `json:"attempts"`
}

type TraceRequest struct {
	Target           string `json:"target" jsonschema:"Target domain, IP, or URL host to trace"`
	Protocol         string `json:"protocol,omitempty" jsonschema:"Probe protocol: icmp, tcp, or udp"`
	Port             int    `json:"port,omitempty" jsonschema:"Destination port for TCP/UDP probes"`
	Queries          int    `json:"queries,omitempty" jsonschema:"Probe samples per hop"`
	MaxHops          int    `json:"max_hops,omitempty" jsonschema:"Maximum TTL/hop count"`
	TimeoutMs        int    `json:"timeout_ms,omitempty" jsonschema:"Per-probe timeout in milliseconds"`
	PacketSize       *int   `json:"packet_size,omitempty" jsonschema:"Packet size including IP and protocol headers"`
	TOS              *int   `json:"tos,omitempty" jsonschema:"IPv4 TOS / IPv6 traffic class value 0-255"`
	ParallelRequests int    `json:"parallel_requests,omitempty" jsonschema:"Maximum concurrent probes"`
	BeginHop         int    `json:"begin_hop,omitempty" jsonschema:"First TTL to probe"`
	IPv4Only         bool   `json:"ipv4_only,omitempty" jsonschema:"Force IPv4 target resolution"`
	IPv6Only         bool   `json:"ipv6_only,omitempty" jsonschema:"Force IPv6 target resolution"`
	DataProvider     string `json:"data_provider,omitempty" jsonschema:"GeoIP provider name"`
	PowProvider      string `json:"pow_provider,omitempty" jsonschema:"PoW provider for LeoMoeAPI"`
	DotServer        string `json:"dot_server,omitempty" jsonschema:"DoT server for target and GeoIP DNS resolution"`
	DisableRDNS      bool   `json:"disable_rdns,omitempty" jsonschema:"Disable reverse DNS lookup"`
	AlwaysRDNS       bool   `json:"always_rdns,omitempty" jsonschema:"Wait for reverse DNS whenever possible"`
	DisableMaptrace  bool   `json:"disable_maptrace,omitempty" jsonschema:"Disable tracemap URL generation"`
	DisableMPLS      bool   `json:"disable_mpls,omitempty" jsonschema:"Disable MPLS parsing"`
	Language         string `json:"language,omitempty" jsonschema:"Output language: cn or en"`
	DN42             bool   `json:"dn42,omitempty" jsonschema:"Use DN42 mode"`
	SourceAddress    string `json:"source_address,omitempty" jsonschema:"Source IP address"`
	SourcePort       int    `json:"source_port,omitempty" jsonschema:"Source port"`
	SourceDevice     string `json:"source_device,omitempty" jsonschema:"Source network device"`
	ICMPMode         int    `json:"icmp_mode,omitempty" jsonschema:"Windows ICMP mode: 0 auto, 1 socket, 2 WinDivert"`
	PacketInterval   int    `json:"packet_interval,omitempty" jsonschema:"Per-packet interval in milliseconds"`
	TTLInterval      int    `json:"ttl_interval,omitempty" jsonschema:"TTL group interval in milliseconds"`
	MaxAttempts      int    `json:"max_attempts,omitempty" jsonschema:"Hard cap on probe attempts per hop"`
}

type TraceResponse struct {
	Target       string              `json:"target"`
	ResolvedIP   string              `json:"resolved_ip"`
	Protocol     string              `json:"protocol"`
	DataProvider string              `json:"data_provider"`
	Language     string              `json:"language"`
	Hops         []Hop               `json:"hops"`
	DurationMs   int64               `json:"duration_ms"`
	Parameters   ParameterBoundaries `json:"parameters"`
}

type MTRReportRequest struct {
	TraceRequest
	HopIntervalMs int `json:"hop_interval_ms,omitempty" jsonschema:"Per-hop probe interval in milliseconds"`
	MaxPerHop     int `json:"max_per_hop,omitempty" jsonschema:"Maximum probes per TTL"`
}

type MTRRawRequest struct {
	TraceRequest
	HopIntervalMs int `json:"hop_interval_ms,omitempty" jsonschema:"Per-hop probe interval in milliseconds"`
	MaxPerHop     int `json:"max_per_hop,omitempty" jsonschema:"Maximum records per TTL; defaults to 3 if duration_ms is unset"`
	DurationMs    int `json:"duration_ms,omitempty" jsonschema:"Maximum run duration in milliseconds"`
}

type MTRReportResponse struct {
	Target     string              `json:"target"`
	ResolvedIP string              `json:"resolved_ip"`
	Protocol   string              `json:"protocol"`
	Stats      []trace.MTRHopStat  `json:"stats"`
	DurationMs int64               `json:"duration_ms"`
	Parameters ParameterBoundaries `json:"parameters"`
}

type MTRRawResponse struct {
	Target     string               `json:"target"`
	ResolvedIP string               `json:"resolved_ip"`
	Protocol   string               `json:"protocol"`
	Records    []trace.MTRRawRecord `json:"records"`
	DurationMs int64                `json:"duration_ms"`
	Warnings   []string             `json:"warnings,omitempty"`
	Parameters ParameterBoundaries  `json:"parameters"`
}

type MTUTraceRequest struct {
	Target        string `json:"target" jsonschema:"Target domain or IP"`
	Port          int    `json:"port,omitempty" jsonschema:"Destination UDP port"`
	Queries       int    `json:"queries,omitempty" jsonschema:"Probe attempts per hop"`
	MaxHops       int    `json:"max_hops,omitempty" jsonschema:"Maximum TTL/hop count"`
	BeginHop      int    `json:"begin_hop,omitempty" jsonschema:"First TTL to probe"`
	TimeoutMs     int    `json:"timeout_ms,omitempty" jsonschema:"Per-probe timeout in milliseconds"`
	TTLIntervalMs int    `json:"ttl_interval_ms,omitempty" jsonschema:"TTL interval in milliseconds"`
	IPv4Only      bool   `json:"ipv4_only,omitempty" jsonschema:"Force IPv4 target resolution"`
	IPv6Only      bool   `json:"ipv6_only,omitempty" jsonschema:"Force IPv6 target resolution"`
	DataProvider  string `json:"data_provider,omitempty" jsonschema:"GeoIP provider name"`
	DotServer     string `json:"dot_server,omitempty" jsonschema:"DoT server"`
	DisableRDNS   bool   `json:"disable_rdns,omitempty" jsonschema:"Disable reverse DNS"`
	AlwaysRDNS    bool   `json:"always_rdns,omitempty" jsonschema:"Wait for reverse DNS"`
	Language      string `json:"language,omitempty" jsonschema:"Output language: cn or en"`
	SourceAddress string `json:"source_address,omitempty" jsonschema:"Source IP address"`
	SourcePort    int    `json:"source_port,omitempty" jsonschema:"Source port"`
	SourceDevice  string `json:"source_device,omitempty" jsonschema:"Source network device"`
}

type MTUTraceResponse struct {
	Target     string              `json:"target"`
	ResolvedIP string              `json:"resolved_ip"`
	Protocol   string              `json:"protocol"`
	IPVersion  int                 `json:"ip_version"`
	StartMTU   int                 `json:"start_mtu"`
	ProbeSize  int                 `json:"probe_size"`
	PathMTU    int                 `json:"path_mtu"`
	Hops       []mtutrace.Hop      `json:"hops"`
	DurationMs int64               `json:"duration_ms"`
	Parameters ParameterBoundaries `json:"parameters"`
}

type SpeedTestRequest struct {
	Provider      string `json:"provider,omitempty" jsonschema:"Speed backend: apple or cloudflare"`
	Max           string `json:"max,omitempty" jsonschema:"Per-thread transfer cap; MCP default is conservative"`
	TimeoutMs     int    `json:"timeout_ms,omitempty" jsonschema:"Per-thread timeout in milliseconds"`
	Threads       int    `json:"threads,omitempty" jsonschema:"Concurrent transfer workers"`
	LatencyCount  int    `json:"latency_count,omitempty" jsonschema:"Idle latency sample count"`
	EndpointIP    string `json:"endpoint_ip,omitempty" jsonschema:"Force endpoint IP"`
	NoMetadata    bool   `json:"no_metadata,omitempty" jsonschema:"Skip client/server metadata lookup"`
	Language      string `json:"language,omitempty" jsonschema:"Output language: cn or en"`
	DotServer     string `json:"dot_server,omitempty" jsonschema:"DoT server for endpoint discovery"`
	SourceAddress string `json:"source_address,omitempty" jsonschema:"Source address for HTTP connections"`
	SourceDevice  string `json:"source_device,omitempty" jsonschema:"Source device"`
}

type SpeedTestResponse struct {
	Result     speedresult.RunResult `json:"result"`
	Parameters ParameterBoundaries   `json:"parameters"`
}

type AnnotateIPsRequest struct {
	Text         string `json:"text" jsonschema:"Text containing IPv4/IPv6 literals"`
	DataProvider string `json:"data_provider,omitempty" jsonschema:"GeoIP provider name"`
	TimeoutMs    int    `json:"timeout_ms,omitempty" jsonschema:"Lookup timeout in milliseconds"`
	Language     string `json:"language,omitempty" jsonschema:"Output language: cn or en"`
	IPv4Only     bool   `json:"ipv4_only,omitempty" jsonschema:"Only annotate IPv4 literals"`
	IPv6Only     bool   `json:"ipv6_only,omitempty" jsonschema:"Only annotate IPv6 literals"`
}

type AnnotateIPsResponse struct {
	Text       string              `json:"text"`
	Parameters ParameterBoundaries `json:"parameters"`
}

type GeoLookupRequest struct {
	Query        string `json:"query" jsonschema:"IP address to look up"`
	DataProvider string `json:"data_provider,omitempty" jsonschema:"GeoIP provider name"`
	Language     string `json:"language,omitempty" jsonschema:"Output language: cn or en"`
}

type GeoLookupResponse struct {
	Query      string              `json:"query"`
	Geo        *ipgeo.IPGeoData    `json:"geo,omitempty"`
	Parameters ParameterBoundaries `json:"parameters"`
}

type GlobalpingTraceRequest struct {
	Target    string   `json:"target" jsonschema:"Target domain or IP for Globalping measurement"`
	Locations []string `json:"locations,omitempty" jsonschema:"Globalping magic location strings, such as country, city, ASN, ISP, cloud region"`
	Limit     int      `json:"limit,omitempty" jsonschema:"Maximum probes selected by Globalping"`
	Protocol  string   `json:"protocol,omitempty" jsonschema:"Probe protocol: ICMP, TCP, or UDP"`
	Port      int      `json:"port,omitempty" jsonschema:"Destination port for TCP/UDP probes"`
	Packets   int      `json:"packets,omitempty" jsonschema:"Packet count per probe"`
	IPVersion int      `json:"ip_version,omitempty" jsonschema:"Optional IP version: 4 or 6"`
}

type GlobalpingGetMeasurementRequest struct {
	MeasurementID string `json:"measurement_id" jsonschema:"Globalping measurement ID returned by nexttrace_globalping_trace"`
}

type GlobalpingLimitsRequest struct{}

type GlobalpingMeasurementResponse struct {
	MeasurementID string                  `json:"measurement_id"`
	Type          string                  `json:"type"`
	Target        string                  `json:"target"`
	Status        string                  `json:"status"`
	ProbesCount   int                     `json:"probes_count"`
	Results       []GlobalpingProbeResult `json:"results"`
	Parameters    ParameterBoundaries     `json:"parameters"`
}

type GlobalpingProbeResult struct {
	Probe            GlobalpingProbeInfo `json:"probe"`
	Status           string              `json:"status"`
	ResolvedAddress  string              `json:"resolved_address,omitempty"`
	ResolvedHostname string              `json:"resolved_hostname,omitempty"`
	Hops             []GlobalpingHop     `json:"hops,omitempty"`
	RawOutput        string              `json:"raw_output,omitempty"`
}

type GlobalpingProbeInfo struct {
	Continent string   `json:"continent,omitempty"`
	Region    string   `json:"region,omitempty"`
	Country   string   `json:"country,omitempty"`
	City      string   `json:"city,omitempty"`
	State     string   `json:"state,omitempty"`
	ASN       int      `json:"asn,omitempty"`
	Network   string   `json:"network,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type GlobalpingHop struct {
	TTL              int                 `json:"ttl"`
	ResolvedAddress  string              `json:"resolved_address,omitempty"`
	ResolvedHostname string              `json:"resolved_hostname,omitempty"`
	ASN              []int               `json:"asn,omitempty"`
	TimingsMs        []float64           `json:"timings_ms,omitempty"`
	Stats            *GlobalpingMTRStats `json:"stats,omitempty"`
}

type GlobalpingMTRStats struct {
	Min   float64 `json:"min"`
	Avg   float64 `json:"avg"`
	Max   float64 `json:"max"`
	StDev float64 `json:"stdev"`
	JMin  float64 `json:"jmin"`
	JAvg  float64 `json:"javg"`
	JMax  float64 `json:"jmax"`
	Total int     `json:"total"`
	Rcv   int     `json:"rcv"`
	Drop  int     `json:"drop"`
	Loss  float64 `json:"loss"`
}

type GlobalpingLimitsResponse struct {
	Measurements GlobalpingMeasurementLimits `json:"measurements"`
	Credits      GlobalpingCreditLimits      `json:"credits"`
	Parameters   ParameterBoundaries         `json:"parameters"`
}

type GlobalpingMeasurementLimits struct {
	Create GlobalpingCreateLimit `json:"create"`
}

type GlobalpingCreateLimit struct {
	Type      string `json:"type"`
	Limit     int64  `json:"limit"`
	Remaining int64  `json:"remaining"`
	Reset     int64  `json:"reset"`
}

type GlobalpingCreditLimits struct {
	Remaining int64 `json:"remaining"`
}

type CapabilitiesResponse struct {
	Tools      []ToolCapability    `json:"tools"`
	Parameters ParameterBoundaries `json:"parameters"`
}

type ToolCapability struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  ParameterBoundaries `json:"parameters"`
}

func durationMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
