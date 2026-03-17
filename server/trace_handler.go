package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracemap"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
)

var traceMu sync.Mutex
var leoConnMu sync.Mutex
var traceMapURLFn = tracemap.GetMapUrl
var withTraceMapScopeFn = func(setup *traceExecution, callback func() (string, error)) (string, error) {
	return withTraceGeoDNSScope(setup, callback)
}

type traceExecution struct {
	Req          traceRequest
	Target       string
	Protocol     string
	DataProvider string
	Method       trace.Method
	IP           net.IP
	Config       trace.Config
	NeedsLeoWS   bool
	PowProvider  string
}

type traceRequest struct {
	Target            string `json:"target"`
	Protocol          string `json:"protocol"`
	Port              int    `json:"port"`
	Queries           int    `json:"queries"`
	MaxHops           int    `json:"max_hops"`
	TimeoutMs         int    `json:"timeout_ms"`
	PacketSize        *int   `json:"packet_size"`
	TOS               *int   `json:"tos"`
	ParallelRequests  int    `json:"parallel_requests"`
	BeginHop          int    `json:"begin_hop"`
	IPv4Only          bool   `json:"ipv4_only"`
	IPv6Only          bool   `json:"ipv6_only"`
	DataProvider      string `json:"data_provider"`
	PowProvider       string `json:"pow_provider"`
	DotServer         string `json:"dot_server"`
	DisableRDNS       bool   `json:"disable_rdns"`
	AlwaysRDNS        bool   `json:"always_rdns"`
	DisableMaptrace   bool   `json:"disable_maptrace"`
	DisableMPLS       bool   `json:"disable_mpls"`
	Language          string `json:"language"`
	DN42              bool   `json:"dn42"`
	SourceAddress     string `json:"source_address"`
	SourcePort        int    `json:"source_port"`
	SourceDevice      string `json:"source_device"`
	ICMPMode          int    `json:"icmp_mode"`
	PacketInterval    int    `json:"packet_interval"`
	TTLInterval       int    `json:"ttl_interval"`
	MaxAttempts       int    `json:"max_attempts"`
	AlwaysWaitRDNS    bool   `json:"always_wait_rdns"`
	Maptrace          *bool  `json:"maptrace"` // deprecated toggle compatibility
	LanguageOverride  string `json:"language_override"`
	DataProviderAlias string `json:"data_provider_alias"`
	Mode              string `json:"mode"`
	IntervalMs        int    `json:"interval_ms"`
	HopIntervalMs     int    `json:"hop_interval_ms"`
	MaxRounds         int    `json:"max_rounds"`
}

type hopAttempt struct {
	Success  bool             `json:"success"`
	IP       string           `json:"ip,omitempty"`
	Hostname string           `json:"hostname,omitempty"`
	RTT      float64          `json:"rtt_ms,omitempty"`
	Error    string           `json:"error,omitempty"`
	MPLS     []string         `json:"mpls,omitempty"`
	Geo      *ipgeo.IPGeoData `json:"geo,omitempty"`
}

type hopResponse struct {
	TTL      int          `json:"ttl"`
	Attempts []hopAttempt `json:"attempts"`
}

type traceResponse struct {
	Target       string        `json:"target"`
	ResolvedIP   string        `json:"resolved_ip"`
	Protocol     string        `json:"protocol"`
	DataProvider string        `json:"data_provider"`
	TraceMapURL  string        `json:"trace_map_url,omitempty"`
	Language     string        `json:"language"`
	Hops         []hopResponse `json:"hops"`
	DurationMs   int64         `json:"duration_ms"`
}

type traceProtocolSelection struct {
	protocol string
	method   trace.Method
	dstPort  int
}

func normalizeTraceRequest(req *traceRequest) (int, error) {
	if req == nil {
		return http.StatusBadRequest, errors.New("request is required")
	}

	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Maptrace != nil {
		req.DisableMaptrace = !*req.Maptrace
	}
	if req.IPv4Only && req.IPv6Only {
		return http.StatusBadRequest, errors.New("ipv4_only and ipv6_only cannot be true at the same time")
	}
	if err := validateSourceDevice(req.SourceDevice); err != nil {
		return http.StatusBadRequest, err
	}
	if req.IntervalMs <= 0 {
		req.IntervalMs = 0
	}
	if req.MaxRounds < 0 {
		req.MaxRounds = 0
	}
	if req.TOS != nil && (*req.TOS < 0 || *req.TOS > 255) {
		return http.StatusBadRequest, errors.New("tos must be within range 0-255")
	}
	return 0, nil
}

func resolveTraceProtocol(req traceRequest) (traceProtocolSelection, int, error) {
	protocol := strings.ToLower(strings.TrimSpace(req.Protocol))
	if protocol == "" {
		protocol = "icmp"
	}
	if !contains(supportedProtocols, protocol) {
		return traceProtocolSelection{}, http.StatusBadRequest, fmt.Errorf("unsupported protocol %q", protocol)
	}

	method := trace.ICMPTrace
	switch protocol {
	case "udp":
		method = trace.UDPTrace
	case "tcp":
		method = trace.TCPTrace
	}

	dstPort := req.Port
	if dstPort == 0 {
		switch method {
		case trace.UDPTrace:
			dstPort = 33494
		case trace.TCPTrace:
			dstPort = 80
		}
	}

	return traceProtocolSelection{
		protocol: protocol,
		method:   method,
		dstPort:  dstPort,
	}, 0, nil
}

func resolveTraceDataProvider(req *traceRequest) (string, bool) {
	dataProvider := normalizeDataProvider(req.DataProvider, req.DataProviderAlias)
	if dataProvider == "" {
		dataProvider = defaults["data_provider"].(string)
	}

	if strings.EqualFold(dataProvider, "DN42") {
		req.DN42 = true
	}
	if req.DN42 {
		config.InitConfig()
		req.DisableMaptrace = true
		dataProvider = "DN42"
	}

	needsLeoWS := strings.EqualFold(dataProvider, "LEOMOEAPI")
	if needsLeoWS && util.EnvDataProvider != "" {
		dataProvider = util.EnvDataProvider
		needsLeoWS = strings.EqualFold(dataProvider, "LEOMOEAPI")
	}

	return dataProvider, needsLeoWS
}

func resolveTraceIPVersion(req traceRequest) string {
	switch {
	case req.IPv4Only:
		return "4"
	case req.IPv6Only:
		return "6"
	default:
		return "all"
	}
}

func prepareTrace(req traceRequest) (*traceExecution, int, error) {
	exec := &traceExecution{
		Req: req,
	}

	if statusCode, err := normalizeTraceRequest(&exec.Req); err != nil {
		return nil, statusCode, err
	}

	target, err := normalizeTarget(exec.Req.Target)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	exec.Target = target

	protocol, statusCode, err := resolveTraceProtocol(exec.Req)
	if err != nil {
		return nil, statusCode, err
	}
	exec.Protocol = protocol.protocol
	exec.Method = protocol.method

	dataProvider, needsLeoWS := resolveTraceDataProvider(&exec.Req)
	ip, err := util.DomainLookUp(target, resolveTraceIPVersion(exec.Req), strings.ToLower(exec.Req.DotServer), true)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	exec.IP = ip

	exec.DataProvider = dataProvider
	exec.PowProvider = strings.TrimSpace(exec.Req.PowProvider)
	exec.NeedsLeoWS = needsLeoWS
	exec.Config, err = buildTraceConfig(exec.Req, exec.Method, ip, dataProvider, protocol.dstPort)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	return exec, 0, nil
}

func traceHandler(c *gin.Context) {
	var req traceRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxTraceRequestBodyBytes)
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request payload too large"})
			return
		}
		c.JSON(400, gin.H{"error": "invalid request payload", "details": err.Error()})
		return
	}

	setup, statusCode, err := prepareTrace(req)
	if err != nil {
		if statusCode == 0 {
			statusCode = 500
		}
		log.Printf("[deploy] prepare trace failed target=%s error=%v", sanitizeLogParam(req.Target), err)
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[deploy] trace request target=%s proto=%s provider=%s lang=%s ipv4_only=%t ipv6_only=%t", sanitizeLogParam(setup.Target), sanitizeLogParam(setup.Protocol), sanitizeLogParam(setup.DataProvider), sanitizeLogParam(setup.Config.Lang), setup.Req.IPv4Only, setup.Req.IPv6Only)
	log.Printf("[deploy] target resolved target=%s ip=%s via dot=%s", sanitizeLogParam(setup.Target), setup.IP, sanitizeLogParam(strings.ToLower(setup.Req.DotServer)))

	traceMu.Lock()
	defer traceMu.Unlock()

	if setup.NeedsLeoWS {
		if _, err := withTraceSetupContext(setup, func() (struct{}, error) {
			ensureLeoMoeConnection()
			return struct{}{}, nil
		}); err != nil {
			log.Printf("[deploy] failed to initialize LeoMoeAPI connection target=%s error=%v", sanitizeLogParam(setup.Target), err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}

	configured := setup.Config
	log.Printf("[deploy] starting trace target=%s resolved=%s method=%s lang=%s queries=%d maxHops=%d", sanitizeLogParam(setup.Target), setup.IP.String(), string(setup.Method), sanitizeLogParam(configured.Lang), configured.NumMeasurements, configured.MaxHops)

	start := time.Now()
	res, err := withTraceSetupContext(setup, func() (*trace.Result, error) {
		return traceTracerouteFn(setup.Method, configured)
	})
	duration := time.Since(start)
	if err != nil {
		log.Printf("[deploy] trace failed target=%s error=%v", sanitizeLogParam(setup.Target), err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	traceMapURL := traceMapURLForResult(setup, res)
	if traceMapURL != "" {
		log.Printf("[deploy] trace map generated target=%s mapUrl=%s", sanitizeLogParam(setup.Target), traceMapURL)
	}

	response := traceResponse{
		Target:       setup.Target,
		ResolvedIP:   setup.IP.String(),
		Protocol:     setup.Protocol,
		DataProvider: setup.DataProvider,
		TraceMapURL:  traceMapURL,
		Language:     configured.Lang,
		Hops:         convertHops(res, configured.Lang),
		DurationMs:   duration.Milliseconds(),
	}

	log.Printf("[deploy] trace completed target=%s hops=%d duration=%s", sanitizeLogParam(setup.Target), len(response.Hops), duration)
	c.JSON(200, response)
}

func buildTraceConfig(req traceRequest, method trace.Method, ip net.IP, dataProvider string, port int) (trace.Config, error) {
	lang := strings.TrimSpace(req.Language)
	if lang == "" {
		lang = defaults["language"].(string)
	}

	timeout := req.TimeoutMs
	if timeout <= 0 {
		timeout = defaults["timeout_ms"].(int)
	}

	packetSize := trace.DefaultPacketSize(method, ip)
	if req.PacketSize != nil {
		packetSize = *req.PacketSize
	}
	packetSizeSpec, err := trace.NormalizePacketSize(method, ip, packetSize)
	if err != nil {
		return trace.Config{}, err
	}

	tos := defaults["tos"].(int)
	if req.TOS != nil {
		tos = *req.TOS
	}

	if req.PacketInterval <= 0 {
		req.PacketInterval = 50
	}
	if req.TTLInterval <= 0 {
		req.TTLInterval = 50
	}

	maxHops := req.MaxHops
	if maxHops <= 0 {
		maxHops = defaults["max_hops"].(int)
	}

	queries := req.Queries
	if queries <= 0 {
		queries = defaults["queries"].(int)
	}

	parallel := req.ParallelRequests
	if parallel <= 0 {
		parallel = defaults["parallel_requests"].(int)
	}

	beginHop := req.BeginHop
	if beginHop <= 0 {
		beginHop = defaults["begin_hop"].(int)
	}

	alwaysWait := req.AlwaysWaitRDNS || req.AlwaysRDNS

	ostype := 3
	switch runtime.GOOS {
	case "darwin":
		ostype = 1
	case "windows":
		ostype = 2
	}

	return trace.Config{
		OSType:           ostype,
		ICMPMode:         req.ICMPMode,
		SrcAddr:          req.SourceAddress,
		SrcPort:          req.SourcePort,
		SourceDevice:     strings.TrimSpace(req.SourceDevice),
		BeginHop:         beginHop,
		MaxHops:          maxHops,
		NumMeasurements:  queries,
		MaxAttempts:      req.MaxAttempts,
		ParallelRequests: parallel,
		Timeout:          time.Duration(timeout) * time.Millisecond,
		DstIP:            ip,
		DstPort:          port,
		IPGeoSource:      ipgeo.GetSourceWithGeoDNS(dataProvider, req.DotServer),
		RDNS:             !req.DisableRDNS,
		AlwaysWaitRDNS:   alwaysWait,
		PacketInterval:   req.PacketInterval,
		TTLInterval:      req.TTLInterval,
		Lang:             lang,
		DN42:             req.DN42,
		PktSize:          packetSizeSpec.PayloadSize,
		RandomPacketSize: packetSizeSpec.Random,
		TOS:              tos,
		Maptrace:         !req.DisableMaptrace,
		DisableMPLS:      req.DisableMPLS,
	}, nil
}

func withTraceSetupContext[T any](setup *traceExecution, callback func() (T, error)) (T, error) {
	if callback == nil {
		var zero T
		return zero, nil
	}

	prevPowProvider := util.PowProviderParam
	util.PowProviderParam = ""
	if setup != nil {
		util.PowProviderParam = setup.PowProvider
		if setup.NeedsLeoWS {
			if setup.PowProvider != "" {
				log.Printf("[deploy] LeoMoeAPI using custom PoW provider=%s", sanitizeLogParam(setup.PowProvider))
			} else {
				log.Printf("[deploy] LeoMoeAPI using default PoW provider")
			}
		} else if setup.PowProvider != "" {
			log.Printf("[deploy] overriding PoW provider=%s", sanitizeLogParam(setup.PowProvider))
		}
	}
	defer func() {
		util.PowProviderParam = prevPowProvider
	}()

	return withTraceGeoDNSScope(setup, callback)
}

func withTraceGeoDNSScope[T any](setup *traceExecution, callback func() (T, error)) (T, error) {
	if callback == nil {
		var zero T
		return zero, nil
	}
	dotServer := ""
	if setup != nil {
		dotServer = strings.TrimSpace(strings.ToLower(setup.Req.DotServer))
	}
	return util.WithGeoDNSResolver(dotServer, callback)
}

func traceMapURLForResult(setup *traceExecution, res *trace.Result) string {
	if setup == nil || res == nil || !setup.Config.Maptrace || !shouldGenerateMap(setup.DataProvider) {
		return ""
	}
	payload, err := json.Marshal(res)
	if err != nil {
		return ""
	}
	url, err := withTraceMapScopeFn(setup, func() (string, error) {
		return traceMapURLFn(string(payload))
	})
	if err != nil {
		return ""
	}
	return url
}

func convertHops(res *trace.Result, lang string) []hopResponse {
	if res == nil || len(res.Hops) == 0 {
		return nil
	}

	hops := make([]hopResponse, 0, len(res.Hops))
	for idx, attempts := range res.Hops {
		resp := buildHopResponse(attempts, idx, lang)
		if len(resp.Attempts) == 0 {
			continue
		}
		hops = append(hops, resp)
	}
	return hops
}

func buildHopResponse(attempts []trace.Hop, idx int, lang string) hopResponse {
	resp := hopResponse{
		TTL:      idx + 1,
		Attempts: make([]hopAttempt, 0, len(attempts)),
	}

	for _, attempt := range attempts {
		ha := hopAttempt{
			Success: attempt.Success,
			MPLS:    attempt.MPLS,
		}
		if attempt.Address != nil {
			ha.IP = attempt.Address.String()
		}
		if attempt.Hostname != "" {
			ha.Hostname = attempt.Hostname
		}
		if attempt.RTT > 0 {
			ha.RTT = float64(attempt.RTT) / float64(time.Millisecond)
		}
		if attempt.Error != nil {
			ha.Error = attempt.Error.Error()
		}
		if attempt.Geo != nil {
			ha.Geo = localizeGeo(attempt.Geo, lang)
		}
		resp.Attempts = append(resp.Attempts, ha)
	}
	return resp
}

func parseTargetURLHost(target string) (string, string, error) {
	fallbackSource := target
	if !strings.Contains(target, "://") {
		return "", fallbackSource, nil
	}

	u, err := url.Parse(target)
	if err != nil {
		return "", "", fmt.Errorf("invalid target format: %w", err)
	}
	if u.Host != "" {
		return u.Host, fallbackSource, nil
	}
	if u.Path != "" {
		fallbackSource = strings.TrimPrefix(target, u.Scheme+"://")
	}
	return "", fallbackSource, nil
}

func extractTargetHost(target, fallbackSource string) (string, error) {
	parseTarget := target
	if strings.Contains(target, "/") {
		if !strings.HasPrefix(parseTarget, "//") {
			parseTarget = "//" + parseTarget
		}
		if u, err := url.Parse(parseTarget); err == nil && u.Host != "" {
			return u.Host, nil
		}
	}

	if !strings.Contains(fallbackSource, "/") {
		return "", nil
	}
	idx := strings.Index(fallbackSource, "/")
	if idx <= 0 {
		return "", errors.New("invalid target format")
	}
	candidate := strings.TrimSpace(fallbackSource[:idx])
	if candidate == "" {
		return "", errors.New("invalid target format")
	}
	return candidate, nil
}

func stripTargetPort(target string) string {
	// Try standard SplitHostPort first — handles host:port and [IPv6]:port.
	if host, _, err := net.SplitHostPort(target); err == nil {
		return host
	}
	// Bare [IPv6] without port.
	if open := strings.Index(target, "["); open >= 0 {
		close := strings.Index(target[open:], "]")
		if close > 1 {
			return target[open+1 : open+close]
		}
	}
	// host:port with exactly one colon (plain IPv4 / hostname).
	if strings.Count(target, ":") == 1 {
		return target[:strings.Index(target, ":")]
	}
	return target
}

func normalizeTarget(input string) (string, error) {
	target := strings.TrimSpace(input)
	if target == "" {
		return "", errors.New("target is required")
	}

	host, fallbackSource, err := parseTargetURLHost(target)
	if err != nil {
		return "", err
	}
	if host == "" {
		host, err = extractTargetHost(target, fallbackSource)
		if err != nil {
			return "", err
		}
	}
	if host != "" {
		target = host
	}

	return strings.TrimSpace(stripTargetPort(target)), nil
}
func normalizeDataProvider(provider string, alias string) string {
	candidate := strings.TrimSpace(provider)
	if candidate == "" {
		candidate = strings.TrimSpace(alias)
	}
	if candidate == "" {
		return ""
	}

	upper := strings.ToUpper(candidate)
	switch upper {
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

func contains(list []string, v string) bool {
	for _, item := range list {
		if strings.EqualFold(item, v) {
			return true
		}
	}
	return false
}

func shouldGenerateMap(provider string) bool {
	allowed := []string{"LEOMOEAPI", "IPINFO", "IP-API.COM", "IPAPI.COM"}
	for _, item := range allowed {
		if strings.EqualFold(provider, item) {
			return true
		}
	}
	return false
}

func validateSourceDevice(device string) error {
	device = strings.TrimSpace(device)
	if device == "" {
		return nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("list network interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Name == device {
			return nil
		}
	}

	return fmt.Errorf("unknown source_device %q", device)
}

func ensureLeoMoeConnection() {
	leoConnMu.Lock()
	defer leoConnMu.Unlock()

	conn := wshandle.GetWsConn()
	if conn == nil || conn.MsgSendCh == nil || conn.MsgReceiveCh == nil {
		log.Println("[deploy] establishing initial LeoMoeAPI websocket")
		wshandle.New()
		return
	}

	if !conn.IsConnected() && !conn.IsConnecting() {
		log.Println("[deploy] reconnecting LeoMoeAPI websocket")
		wshandle.New()
	}
}

func localizeGeo(src *ipgeo.IPGeoData, lang string) *ipgeo.IPGeoData {
	if src == nil {
		return nil
	}

	dst := *src
	switch strings.ToLower(lang) {
	case "en":
		if dst.CountryEn != "" {
			dst.Country = dst.CountryEn
		}
		if dst.ProvEn != "" {
			dst.Prov = dst.ProvEn
		}
		if dst.CityEn != "" {
			dst.City = dst.CityEn
		}
	default:
		if dst.Country == "" && dst.CountryEn != "" {
			dst.Country = dst.CountryEn
		}
		if dst.Prov == "" && dst.ProvEn != "" {
			dst.Prov = dst.ProvEn
		}
		if dst.City == "" && dst.CityEn != "" {
			dst.City = dst.CityEn
		}
	}
	return &dst
}
