package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/internal/nali"
	speedconfig "github.com/nxtrace/NTrace-core/internal/speedtest/config"
	speedrunner "github.com/nxtrace/NTrace-core/internal/speedtest/runner"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	mtutrace "github.com/nxtrace/NTrace-core/trace/mtu"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
)

const (
	defaultProtocol         = "icmp"
	defaultDataProvider     = "LeoMoeAPI"
	defaultLanguage         = "cn"
	defaultQueries          = 3
	defaultMaxHops          = 30
	defaultTimeoutMs        = 1000
	defaultParallelRequests = 18
	defaultBeginHop         = 1
	defaultPacketIntervalMs = 50
	defaultTTLIntervalMs    = 300
	defaultMTRHopIntervalMs = 1000
	defaultMTRRawMaxPerHop  = 3
	defaultSpeedMax         = "50M"
	defaultSpeedTimeoutMs   = 5000
	defaultSpeedThreads     = 2
	defaultSpeedLatency     = 5
)

// RuntimeMu serializes process-global runtime mutations shared by Web, WebSocket, and MCP traces.
var RuntimeMu sync.Mutex

type Service struct{}

type traceSetup struct {
	Request      TraceRequest
	Target       string
	Protocol     string
	Method       trace.Method
	DataProvider string
	NeedsLeoWS   bool
	PowProvider  string
	IP           net.IP
	Config       trace.Config
}

func New() *Service {
	return &Service{}
}

func (s *Service) Capabilities(context.Context, CapabilitiesRequest) (CapabilitiesResponse, error) {
	return CapabilitiesResponse{
		Tools: []ToolCapability{
			toolCapability("nexttrace_capabilities", "List NextTrace MCP tools and parameter boundaries.", []string{}),
			toolCapability("nexttrace_traceroute", "Run local ICMP/TCP/UDP traceroute and return structured hops.", traceSupportedParams()),
			toolCapability("nexttrace_mtr_report", "Run bounded local MTR report and return per-hop statistics.", append(traceSupportedParams(), "hop_interval_ms", "max_per_hop")),
			toolCapability("nexttrace_mtr_raw", "Run bounded local MTR raw stream and return probe-level records.", append(traceSupportedParams(), "hop_interval_ms", "max_per_hop", "duration_ms")),
			toolCapability("nexttrace_mtu_trace", "Run local UDP path-MTU discovery.", []string{"target", "port", "queries", "max_hops", "begin_hop", "timeout_ms", "ttl_interval_ms", "ipv4_only", "ipv6_only", "data_provider", "dot_server", "disable_rdns", "always_rdns", "language", "source_address", "source_port", "source_device"}),
			toolCapability("nexttrace_speed_test", "Run a conservative local speed test.", []string{"provider", "max", "timeout_ms", "threads", "latency_count", "endpoint_ip", "no_metadata", "language", "dot_server", "source_address", "source_device"}),
			toolCapability("nexttrace_annotate_ips", "Annotate IPv4/IPv6 literals in text with GeoIP metadata.", []string{"text", "data_provider", "timeout_ms", "language", "ipv4_only", "ipv6_only"}),
			toolCapability("nexttrace_geo_lookup", "Look up GeoIP metadata for one IP address.", []string{"query", "data_provider", "language"}),
			toolCapabilityWithBoundaries("nexttrace_globalping_trace", "Run Globalping multi-probe MTR/traceroute from requested locations.", globalpingTraceParameterBoundaries()),
			toolCapabilityWithBoundaries("nexttrace_globalping_limits", "Read current Globalping rate/credit limits.", globalpingLimitsParameterBoundaries()),
			toolCapabilityWithBoundaries("nexttrace_globalping_get_measurement", "Fetch a previous Globalping measurement by ID.", globalpingGetParameterBoundaries()),
		},
		Parameters: ParameterBoundaries{
			Supported:       []string{"structured_content", "mcp_streamable_http", "bearer_token", "x_nexttrace_token"},
			NotApplicable:   []string{"stdio_mcp_transport"},
			NotYetSupported: []string{"globalping_location_search"},
		},
	}, nil
}

func (s *Service) Traceroute(ctx context.Context, req TraceRequest) (TraceResponse, error) {
	start := time.Now()
	setup, err := s.prepareTrace(ctx, req)
	if err != nil {
		return TraceResponse{}, err
	}

	res, err := withTraceRuntime(ctx, setup, func() (*trace.Result, error) {
		return trace.TracerouteWithContext(ctx, setup.Method, setup.Config)
	})
	if err != nil {
		return TraceResponse{}, err
	}

	return TraceResponse{
		Target:       setup.Target,
		ResolvedIP:   setup.IP.String(),
		Protocol:     setup.Protocol,
		DataProvider: setup.DataProvider,
		Language:     setup.Config.Lang,
		Hops:         convertTraceHops(res, setup.Config.Lang),
		DurationMs:   durationMs(start),
		Parameters:   traceParameterBoundaries(),
	}, nil
}

func (s *Service) MTRReport(ctx context.Context, req MTRReportRequest) (MTRReportResponse, error) {
	start := time.Now()
	base := req.TraceRequest
	base.Queries = 1
	setup, err := s.prepareTrace(ctx, base)
	if err != nil {
		return MTRReportResponse{}, err
	}

	hopInterval := positiveOrDefault(req.HopIntervalMs, defaultMTRHopIntervalMs)
	maxPerHop := positiveOrDefault(req.MaxPerHop, 10)
	var latest []trace.MTRHopStat
	err = withTraceRuntimeNoResult(ctx, setup, func() error {
		return trace.RunMTR(ctx, setup.Method, setup.Config, trace.MTROptions{
			HopInterval: time.Duration(hopInterval) * time.Millisecond,
			MaxPerHop:   maxPerHop,
		}, func(_ int, stats []trace.MTRHopStat) {
			latest = cloneMTRStats(stats)
		})
	})
	if err != nil {
		return MTRReportResponse{}, err
	}

	return MTRReportResponse{
		Target:     setup.Target,
		ResolvedIP: setup.IP.String(),
		Protocol:   setup.Protocol,
		Stats:      latest,
		DurationMs: durationMs(start),
		Parameters: traceParameterBoundaries(),
	}, nil
}

func (s *Service) MTRRaw(ctx context.Context, req MTRRawRequest) (MTRRawResponse, error) {
	start := time.Now()
	base := req.TraceRequest
	base.Queries = 1
	setup, err := s.prepareTrace(ctx, base)
	if err != nil {
		return MTRRawResponse{}, err
	}

	runCtx := ctx
	if req.DurationMs > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(req.DurationMs)*time.Millisecond)
		defer cancel()
	}
	hopInterval := positiveOrDefault(req.HopIntervalMs, defaultMTRHopIntervalMs)
	maxPerHop := req.MaxPerHop
	if maxPerHop <= 0 && req.DurationMs <= 0 {
		maxPerHop = defaultMTRRawMaxPerHop
	}

	var records []trace.MTRRawRecord
	err = withTraceRuntimeNoResult(runCtx, setup, func() error {
		return trace.RunMTRRaw(runCtx, setup.Method, setup.Config, trace.MTRRawOptions{
			HopInterval: time.Duration(hopInterval) * time.Millisecond,
			MaxPerHop:   maxPerHop,
		}, func(rec trace.MTRRawRecord) {
			records = append(records, rec)
		})
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		return MTRRawResponse{}, err
	}

	warnings := []string{}
	if maxPerHop == defaultMTRRawMaxPerHop && req.DurationMs <= 0 && req.MaxPerHop <= 0 {
		warnings = append(warnings, "max_per_hop defaulted to 3 to keep MCP raw output bounded")
	}
	return MTRRawResponse{
		Target:     setup.Target,
		ResolvedIP: setup.IP.String(),
		Protocol:   setup.Protocol,
		Records:    records,
		DurationMs: durationMs(start),
		Warnings:   warnings,
		Parameters: traceParameterBoundaries(),
	}, nil
}

func (s *Service) MTUTrace(ctx context.Context, req MTUTraceRequest) (MTUTraceResponse, error) {
	start := time.Now()
	cfg, err := s.buildMTUConfig(ctx, req)
	if err != nil {
		return MTUTraceResponse{}, err
	}
	res, err := mtutrace.Run(ctx, cfg)
	if err != nil {
		return MTUTraceResponse{}, err
	}
	hops := sanitizeMTUHops(res.Hops, cfg.Lang)
	return MTUTraceResponse{
		Target:     res.Target,
		ResolvedIP: res.ResolvedIP,
		Protocol:   res.Protocol,
		IPVersion:  res.IPVersion,
		StartMTU:   res.StartMTU,
		ProbeSize:  res.ProbeSize,
		PathMTU:    res.PathMTU,
		Hops:       hops,
		DurationMs: durationMs(start),
		Parameters: ParameterBoundaries{
			Supported:     []string{"target", "port", "queries", "max_hops", "begin_hop", "timeout_ms", "ttl_interval_ms", "ipv4_only", "ipv6_only", "data_provider", "dot_server", "disable_rdns", "always_rdns", "language", "source_address", "source_port", "source_device"},
			NotApplicable: []string{"protocol", "packet_size", "tos"},
		},
	}, nil
}

func (s *Service) SpeedTest(ctx context.Context, req SpeedTestRequest) (SpeedTestResponse, error) {
	cfg, err := buildSpeedConfig(req)
	if err != nil {
		return SpeedTestResponse{}, err
	}
	return SpeedTestResponse{
		Result: speedrunner.Run(ctx, cfg, nil, false),
		Parameters: ParameterBoundaries{
			Supported: []string{"provider", "max", "timeout_ms", "threads", "latency_count", "endpoint_ip", "no_metadata", "language", "dot_server", "source_address", "source_device"},
		},
	}, nil
}

func (s *Service) AnnotateIPs(ctx context.Context, req AnnotateIPsRequest) (AnnotateIPsResponse, error) {
	if strings.TrimSpace(req.Text) == "" {
		return AnnotateIPsResponse{}, errors.New("text is required")
	}
	family := nali.FamilyAll
	switch {
	case req.IPv4Only && req.IPv6Only:
		return AnnotateIPsResponse{}, errors.New("ipv4_only and ipv6_only cannot both be true")
	case req.IPv4Only:
		family = nali.Family4
	case req.IPv6Only:
		family = nali.Family6
	}
	timeout := positiveOrDefault(req.TimeoutMs, defaultTimeoutMs)
	annotator := nali.New(nali.Config{
		Source:  ipgeo.GetSource(normalizeDataProvider(req.DataProvider, "")),
		Timeout: time.Duration(timeout) * time.Millisecond,
		Lang:    normalizeLanguage(req.Language),
		Family:  family,
	})
	return AnnotateIPsResponse{
		Text: annotator.AnnotateLine(ctx, req.Text),
		Parameters: ParameterBoundaries{
			Supported: []string{"text", "data_provider", "timeout_ms", "language", "ipv4_only", "ipv6_only"},
		},
	}, nil
}

func (s *Service) GeoLookup(ctx context.Context, req GeoLookupRequest) (GeoLookupResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return GeoLookupResponse{}, errors.New("query is required")
	}
	provider := normalizeDataProvider(req.DataProvider, "")
	if provider == "" {
		provider = defaultDataProvider
	}
	geo, err := trace.LookupIPGeo(ctx, ipgeo.GetSource(provider), normalizeLanguage(req.Language), false, defaultQueries, query)
	if err != nil {
		return GeoLookupResponse{}, err
	}
	return GeoLookupResponse{
		Query: query,
		Geo:   localizeGeo(geo, normalizeLanguage(req.Language)),
		Parameters: ParameterBoundaries{
			Supported: []string{"query", "data_provider", "language"},
		},
	}, nil
}

func (s *Service) prepareTrace(ctx context.Context, req TraceRequest) (*traceSetup, error) {
	if req.IPv4Only && req.IPv6Only {
		return nil, errors.New("ipv4_only and ipv6_only cannot both be true")
	}
	if req.TOS != nil && (*req.TOS < 0 || *req.TOS > 255) {
		return nil, errors.New("tos must be within range 0-255")
	}

	target, err := normalizeTarget(req.Target)
	if err != nil {
		return nil, err
	}
	method, protocol, port, err := resolveProtocol(req.Protocol, req.Port)
	if err != nil {
		return nil, err
	}
	dataProvider, needsLeo := resolveDataProvider(&req)
	ip, err := util.DomainLookUpWithContext(ctx, target, resolveIPVersion(req), strings.ToLower(req.DotServer), true)
	if err != nil {
		return nil, err
	}
	cfg, err := buildTraceConfig(req, method, ip, dataProvider, port)
	if err != nil {
		return nil, err
	}
	cfg, err = trace.NormalizeExplicitSourceConfig(method, cfg)
	if err != nil {
		return nil, err
	}
	cfg.Context = ctx

	return &traceSetup{
		Request:      req,
		Target:       target,
		Protocol:     protocol,
		Method:       method,
		DataProvider: dataProvider,
		NeedsLeoWS:   needsLeo,
		PowProvider:  strings.TrimSpace(req.PowProvider),
		IP:           ip,
		Config:       cfg,
	}, nil
}

func (s *Service) buildMTUConfig(ctx context.Context, req MTUTraceRequest) (mtutrace.Config, error) {
	if req.IPv4Only && req.IPv6Only {
		return mtutrace.Config{}, errors.New("ipv4_only and ipv6_only cannot both be true")
	}
	target, err := normalizeTarget(req.Target)
	if err != nil {
		return mtutrace.Config{}, err
	}
	ip, err := util.DomainLookUpWithContext(ctx, target, resolveMTUIPVersion(req), strings.ToLower(req.DotServer), true)
	if err != nil {
		return mtutrace.Config{}, err
	}
	sourceCfg, err := trace.NormalizeExplicitSourceConfig(trace.UDPTrace, trace.Config{
		OSType:       resolveOSType(),
		DstIP:        ip,
		SrcAddr:      req.SourceAddress,
		SourceDevice: strings.TrimSpace(req.SourceDevice),
	})
	if err != nil {
		return mtutrace.Config{}, err
	}
	sourceDevice := sourceCfg.SourceDevice
	if sourceDevice == "" && resolveOSType() == 2 && strings.TrimSpace(req.SourceAddress) == "" {
		sourceDevice = strings.TrimSpace(req.SourceDevice)
	}
	srcIP, err := resolveMTUSourceIP(ip, sourceCfg.SrcAddr)
	if err != nil {
		return mtutrace.Config{}, err
	}

	port := req.Port
	if port <= 0 {
		port = 33494
	}
	provider := normalizeDataProvider(req.DataProvider, "")
	if provider == "" {
		provider = defaultDataProvider
	}
	return mtutrace.Config{
		Target:         target,
		DstIP:          ip,
		SrcIP:          srcIP,
		SourceDevice:   sourceDevice,
		SrcPort:        req.SourcePort,
		DstPort:        port,
		BeginHop:       positiveOrDefault(req.BeginHop, defaultBeginHop),
		MaxHops:        positiveOrDefault(req.MaxHops, defaultMaxHops),
		Queries:        positiveOrDefault(req.Queries, defaultQueries),
		Timeout:        time.Duration(positiveOrDefault(req.TimeoutMs, defaultTimeoutMs)) * time.Millisecond,
		TTLInterval:    time.Duration(positiveOrDefault(req.TTLIntervalMs, defaultTTLIntervalMs)) * time.Millisecond,
		RDNS:           !req.DisableRDNS,
		AlwaysWaitRDNS: req.AlwaysRDNS,
		IPGeoSource:    ipgeo.GetSourceWithGeoDNS(provider, req.DotServer),
		Lang:           normalizeLanguage(req.Language),
	}, nil
}

func withTraceRuntime[T any](ctx context.Context, setup *traceSetup, fn func() (T, error)) (T, error) {
	RuntimeMu.Lock()
	defer RuntimeMu.Unlock()

	var zero T
	if setup == nil || fn == nil {
		return zero, nil
	}
	prevPowProvider := util.PowProviderParam
	util.PowProviderParam = setup.PowProvider
	defer func() {
		util.PowProviderParam = prevPowProvider
	}()

	return util.WithGeoDNSResolver(strings.ToLower(strings.TrimSpace(setup.Request.DotServer)), func() (T, error) {
		if setup.NeedsLeoWS {
			ensureLeoMoeConnection(ctx)
		}
		return fn()
	})
}

func withTraceRuntimeNoResult(ctx context.Context, setup *traceSetup, fn func() error) error {
	_, err := withTraceRuntime(ctx, setup, func() (struct{}, error) {
		if fn == nil {
			return struct{}{}, nil
		}
		return struct{}{}, fn()
	})
	return err
}

func ensureLeoMoeConnection(ctx context.Context) {
	conn := wshandle.GetWsConn()
	if conn == nil || conn.MsgSendCh == nil || conn.MsgReceiveCh == nil {
		wshandle.NewWithContext(ctx)
		return
	}
	if !conn.IsConnected() && !conn.IsConnecting() {
		wshandle.NewWithContext(ctx)
	}
}

func resolveProtocol(raw string, port int) (trace.Method, string, int, error) {
	protocol := strings.ToLower(strings.TrimSpace(raw))
	if protocol == "" {
		protocol = defaultProtocol
	}
	switch protocol {
	case "icmp":
		return trace.ICMPTrace, protocol, port, nil
	case "tcp":
		if port <= 0 {
			port = 80
		}
		return trace.TCPTrace, protocol, port, nil
	case "udp":
		if port <= 0 {
			port = 33494
		}
		return trace.UDPTrace, protocol, port, nil
	default:
		return "", "", 0, fmt.Errorf("unsupported protocol %q", protocol)
	}
}

func resolveDataProvider(req *TraceRequest) (string, bool) {
	provider := normalizeDataProvider(req.DataProvider, "")
	if provider == "" {
		provider = defaultDataProvider
	}
	if strings.EqualFold(provider, "DN42") {
		req.DN42 = true
	}
	if req.DN42 {
		config.InitConfig()
		req.DisableMaptrace = true
		provider = "DN42"
	}
	needsLeo := strings.EqualFold(provider, "LEOMOEAPI")
	if needsLeo && util.EnvDataProvider != "" {
		provider = util.EnvDataProvider
		needsLeo = strings.EqualFold(provider, "LEOMOEAPI")
	}
	return provider, needsLeo
}

func buildTraceConfig(req TraceRequest, method trace.Method, ip net.IP, provider string, port int) (trace.Config, error) {
	packetSize := trace.DefaultPacketSize(method, ip)
	if req.PacketSize != nil {
		packetSize = *req.PacketSize
	}
	packetSizeSpec, err := trace.NormalizePacketSize(method, ip, packetSize)
	if err != nil {
		return trace.Config{}, err
	}
	tos := 0
	if req.TOS != nil {
		tos = *req.TOS
	}
	return trace.Config{
		OSType:           resolveOSType(),
		ICMPMode:         req.ICMPMode,
		SrcAddr:          strings.TrimSpace(req.SourceAddress),
		SrcPort:          req.SourcePort,
		SourceDevice:     strings.TrimSpace(req.SourceDevice),
		BeginHop:         positiveOrDefault(req.BeginHop, defaultBeginHop),
		MaxHops:          positiveOrDefault(req.MaxHops, defaultMaxHops),
		NumMeasurements:  positiveOrDefault(req.Queries, defaultQueries),
		MaxAttempts:      req.MaxAttempts,
		ParallelRequests: positiveOrDefault(req.ParallelRequests, defaultParallelRequests),
		Timeout:          time.Duration(positiveOrDefault(req.TimeoutMs, defaultTimeoutMs)) * time.Millisecond,
		DstIP:            ip,
		DstPort:          port,
		IPGeoSource:      ipgeo.GetSourceWithGeoDNS(provider, req.DotServer),
		RDNS:             !req.DisableRDNS,
		AlwaysWaitRDNS:   req.AlwaysRDNS,
		PacketInterval:   positiveOrDefault(req.PacketInterval, defaultPacketIntervalMs),
		TTLInterval:      positiveOrDefault(req.TTLInterval, defaultTTLIntervalMs),
		Lang:             normalizeLanguage(req.Language),
		DN42:             req.DN42,
		PktSize:          packetSizeSpec.PayloadSize,
		RandomPacketSize: packetSizeSpec.Random,
		TOS:              tos,
		Maptrace:         !req.DisableMaptrace,
		DisableMPLS:      req.DisableMPLS,
	}, nil
}

func buildSpeedConfig(req SpeedTestRequest) (*speedconfig.Config, error) {
	args := []string{"--speed", "--non-interactive", "--no-color", "--max", valueOrDefault(req.Max, defaultSpeedMax)}
	args = append(args, "--timeout", strconv.Itoa(positiveOrDefault(req.TimeoutMs, defaultSpeedTimeoutMs)))
	args = append(args, "--threads", strconv.Itoa(positiveOrDefault(req.Threads, defaultSpeedThreads)))
	args = append(args, "--latency-count", strconv.Itoa(positiveOrDefault(req.LatencyCount, defaultSpeedLatency)))
	if strings.TrimSpace(req.Provider) != "" {
		args = append(args, "--speed-provider", req.Provider)
	}
	if strings.TrimSpace(req.EndpointIP) != "" {
		args = append(args, "--endpoint", req.EndpointIP)
	}
	if req.NoMetadata {
		args = append(args, "--no-metadata")
	}
	if strings.TrimSpace(req.Language) != "" {
		args = append(args, "--language", req.Language)
	}
	if strings.TrimSpace(req.DotServer) != "" {
		args = append(args, "--dot-server", req.DotServer)
	}
	if strings.TrimSpace(req.SourceAddress) != "" {
		args = append(args, "--source", req.SourceAddress)
	}
	if strings.TrimSpace(req.SourceDevice) != "" {
		args = append(args, "--dev", req.SourceDevice)
	}
	return speedconfig.Load(args...)
}

func convertTraceHops(res *trace.Result, lang string) []Hop {
	if res == nil {
		return nil
	}
	hops := make([]Hop, 0, len(res.Hops))
	for idx, attempts := range res.Hops {
		resp := Hop{TTL: idx + 1, Attempts: make([]Attempt, 0, len(attempts))}
		for _, hop := range attempts {
			attempt := Attempt{
				Success: hop.Success,
				MPLS:    hop.MPLS,
			}
			if hop.Address != nil {
				attempt.IP = hop.Address.String()
			}
			if hop.Hostname != "" {
				attempt.Hostname = hop.Hostname
			}
			if hop.RTT > 0 {
				attempt.RTTMs = float64(hop.RTT) / float64(time.Millisecond)
			}
			if hop.Error != nil {
				attempt.Error = hop.Error.Error()
			}
			if hop.Geo != nil {
				attempt.Geo = localizeGeo(hop.Geo, lang)
			}
			resp.Attempts = append(resp.Attempts, attempt)
		}
		if len(resp.Attempts) > 0 {
			hops = append(hops, resp)
		}
	}
	return hops
}

func cloneMTRStats(stats []trace.MTRHopStat) []trace.MTRHopStat {
	if len(stats) == 0 {
		return nil
	}
	out := make([]trace.MTRHopStat, len(stats))
	copy(out, stats)
	return out
}

func resolveMTUSourceIP(dstIP net.IP, srcAddr string) (net.IP, error) {
	if trimmed := strings.TrimSpace(srcAddr); trimmed != "" {
		srcIP := net.ParseIP(trimmed)
		if srcIP == nil {
			return nil, fmt.Errorf("invalid source IP %q", srcAddr)
		}
		if util.IsIPv6(dstIP) {
			if !util.IsIPv6(srcIP) {
				return nil, fmt.Errorf("source IP %q does not match IPv6 destination %s", srcAddr, dstIP)
			}
			return srcIP, nil
		}
		if srcIP.To4() == nil {
			return nil, fmt.Errorf("source IP %q does not match IPv4 destination %s", srcAddr, dstIP)
		}
		return srcIP.To4(), nil
	}
	if util.IsIPv6(dstIP) {
		resolved, _ := util.LocalIPPortv6(dstIP, nil, "udp6")
		if resolved == nil {
			return nil, fmt.Errorf("unable to determine IPv6 source address for %s", dstIP)
		}
		return resolved, nil
	}
	resolved, _ := util.LocalIPPort(dstIP, nil, "udp")
	if resolved == nil {
		return nil, fmt.Errorf("unable to determine IPv4 source address for %s", dstIP)
	}
	return resolved, nil
}

func normalizeTarget(input string) (string, error) {
	target := strings.TrimSpace(input)
	if target == "" {
		return "", errors.New("target is required")
	}
	host, fallback, err := parseTargetURLHost(target)
	if err != nil {
		return "", err
	}
	if host == "" {
		host, err = extractTargetHost(target, fallback)
		if err != nil {
			return "", err
		}
	}
	if host != "" {
		target = host
	}
	return strings.TrimSpace(stripTargetPort(target)), nil
}

func parseTargetURLHost(target string) (string, string, error) {
	fallback := target
	if !strings.Contains(target, "://") {
		return "", fallback, nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return "", "", fmt.Errorf("invalid target format: %w", err)
	}
	if u.Host != "" {
		return u.Host, fallback, nil
	}
	if u.Path != "" {
		fallback = strings.TrimPrefix(target, u.Scheme+"://")
	}
	return "", fallback, nil
}

func extractTargetHost(target, fallback string) (string, error) {
	if strings.Contains(target, "/") {
		parseTarget := target
		if !strings.HasPrefix(parseTarget, "//") {
			parseTarget = "//" + parseTarget
		}
		if u, err := url.Parse(parseTarget); err == nil && u.Host != "" {
			return u.Host, nil
		}
	}
	if !strings.Contains(fallback, "/") {
		return "", nil
	}
	idx := strings.Index(fallback, "/")
	if idx <= 0 {
		return "", errors.New("invalid target format")
	}
	candidate := strings.TrimSpace(fallback[:idx])
	if candidate == "" {
		return "", errors.New("invalid target format")
	}
	return candidate, nil
}

func stripTargetPort(target string) string {
	if host, _, err := net.SplitHostPort(target); err == nil {
		return host
	}
	if open := strings.Index(target, "["); open >= 0 {
		closeIdx := strings.Index(target[open:], "]")
		if closeIdx > 1 {
			return target[open+1 : open+closeIdx]
		}
	}
	if strings.Count(target, ":") == 1 {
		return target[:strings.Index(target, ":")]
	}
	return target
}

func normalizeDataProvider(provider, alias string) string {
	candidate := strings.TrimSpace(provider)
	if candidate == "" {
		candidate = strings.TrimSpace(alias)
	}
	if candidate == "" {
		return ""
	}
	switch strings.ToUpper(candidate) {
	case "IP.SB":
		return "IP.SB"
	case "IP-API.COM", "IPAPI.COM":
		return "IPAPI.com"
	case "IPINFO", "IP INFO":
		return "IPInfo"
	case "IPINSIGHT", "IP INSIGHT":
		return "IPInsight"
	case "IPINFOLOCAL", "IP INFO LOCAL":
		return "IPInfoLocal"
	case "LEOMOEAPI", "LEOMOE":
		return "LeoMoeAPI"
	case "CHUNZHEN":
		return "chunzhen"
	case "DN42":
		return "DN42"
	case "DISABLE-GEOIP", "DISABLE_GEOIP":
		return "disable-geoip"
	case "IPDB.ONE":
		return "ipdb.one"
	default:
		return candidate
	}
}

func resolveIPVersion(req TraceRequest) string {
	switch {
	case req.IPv4Only:
		return "4"
	case req.IPv6Only:
		return "6"
	default:
		return "all"
	}
}

func resolveMTUIPVersion(req MTUTraceRequest) string {
	switch {
	case req.IPv4Only:
		return "4"
	case req.IPv6Only:
		return "6"
	default:
		return "all"
	}
}

func resolveOSType() int {
	switch runtime.GOOS {
	case "darwin":
		return 1
	case "windows":
		return 2
	default:
		return 3
	}
}

func normalizeLanguage(lang string) string {
	if strings.EqualFold(strings.TrimSpace(lang), "en") {
		return "en"
	}
	return defaultLanguage
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func localizeGeo(src *ipgeo.IPGeoData, lang string) *ipgeo.IPGeoData {
	if src == nil {
		return nil
	}
	dst := *src
	if dst.Router == nil {
		dst.Router = map[string][]string{}
	}
	if strings.EqualFold(lang, "en") {
		if dst.CountryEn != "" {
			dst.Country = dst.CountryEn
		}
		if dst.ProvEn != "" {
			dst.Prov = dst.ProvEn
		}
		if dst.CityEn != "" {
			dst.City = dst.CityEn
		}
		return &dst
	}
	if dst.Country == "" && dst.CountryEn != "" {
		dst.Country = dst.CountryEn
	}
	if dst.Prov == "" && dst.ProvEn != "" {
		dst.Prov = dst.ProvEn
	}
	if dst.City == "" && dst.CityEn != "" {
		dst.City = dst.CityEn
	}
	return &dst
}

func sanitizeMTUHops(hops []mtutrace.Hop, lang string) []mtutrace.Hop {
	if len(hops) == 0 {
		return nil
	}
	out := make([]mtutrace.Hop, len(hops))
	copy(out, hops)
	for i := range out {
		out[i].Geo = localizeGeo(out[i].Geo, lang)
	}
	return out
}

func traceSupportedParams() []string {
	return []string{"target", "protocol", "port", "queries", "max_hops", "timeout_ms", "packet_size", "tos", "parallel_requests", "begin_hop", "ipv4_only", "ipv6_only", "data_provider", "pow_provider", "dot_server", "disable_rdns", "always_rdns", "disable_maptrace", "disable_mpls", "language", "dn42", "source_address", "source_port", "source_device", "icmp_mode", "packet_interval", "ttl_interval", "max_attempts"}
}

func traceParameterBoundaries() ParameterBoundaries {
	return ParameterBoundaries{
		Supported: traceSupportedParams(),
		NotApplicable: []string{
			"globalping_locations",
			"globalping_limit",
		},
	}
}

func toolCapability(name, description string, supported []string) ToolCapability {
	return toolCapabilityWithBoundaries(name, description, ParameterBoundaries{Supported: supported})
}

func toolCapabilityWithBoundaries(name, description string, params ParameterBoundaries) ToolCapability {
	return ToolCapability{
		Name:        name,
		Description: description,
		Parameters:  params,
	}
}

func globalpingTraceParameterBoundaries() ParameterBoundaries {
	return ParameterBoundaries{
		Supported:     []string{"target", "locations", "limit", "protocol", "port", "packets", "ip_version"},
		NotApplicable: []string{"source_address", "source_device", "dot_server", "packet_size", "tos", "ttl_interval"},
	}
}

func globalpingLimitsParameterBoundaries() ParameterBoundaries {
	return ParameterBoundaries{
		Supported:     []string{},
		NotApplicable: []string{"target", "locations", "source_address", "source_device", "dot_server", "packet_size", "tos", "ttl_interval"},
	}
}

func globalpingGetParameterBoundaries() ParameterBoundaries {
	return ParameterBoundaries{
		Supported:     []string{"measurement_id"},
		NotApplicable: []string{"source_address", "source_device", "dot_server", "packet_size", "tos", "ttl_interval"},
	}
}
