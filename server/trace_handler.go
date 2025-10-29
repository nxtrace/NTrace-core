package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
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
	PacketSize        int    `json:"packet_size"`
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

func prepareTrace(req traceRequest) (*traceExecution, int, error) {
	exec := &traceExecution{
		Req: req,
	}

	exec.Req.Mode = strings.ToLower(strings.TrimSpace(exec.Req.Mode))

	if exec.Req.Maptrace != nil {
		exec.Req.DisableMaptrace = !*exec.Req.Maptrace
	}

	target, err := normalizeTarget(exec.Req.Target)
	if err != nil {
		return nil, 400, err
	}
	exec.Target = target

	if exec.Req.IPv4Only && exec.Req.IPv6Only {
		return nil, 400, errors.New("ipv4_only and ipv6_only cannot be true at the same time")
	}

	if exec.Req.IntervalMs <= 0 {
		exec.Req.IntervalMs = 2000
	}
	if exec.Req.MaxRounds < 0 {
		exec.Req.MaxRounds = 0
	}

	protocol := strings.ToLower(strings.TrimSpace(exec.Req.Protocol))
	if protocol == "" {
		protocol = "icmp"
	}
	if !contains(supportedProtocols, protocol) {
		return nil, 400, fmt.Errorf("unsupported protocol %q", protocol)
	}
	exec.Protocol = protocol

	dataProvider := normalizeDataProvider(exec.Req.DataProvider, exec.Req.DataProviderAlias)
	if dataProvider == "" {
		dataProvider = defaults["data_provider"].(string)
	}

	if strings.EqualFold(dataProvider, "DN42") {
		exec.Req.DN42 = true
	}

	needsLeoWS := strings.EqualFold(dataProvider, "LEOMOEAPI")
	if needsLeoWS && util.EnvDataProvider != "" {
		dataProvider = util.EnvDataProvider
		needsLeoWS = strings.EqualFold(dataProvider, "LEOMOEAPI")
	}

	if exec.Req.DN42 {
		config.InitConfig()
		exec.Req.DisableMaptrace = true
		dataProvider = "DN42"
	}

	ipVersion := "all"
	if exec.Req.IPv4Only {
		ipVersion = "4"
	} else if exec.Req.IPv6Only {
		ipVersion = "6"
	}

	ip, err := util.DomainLookUp(target, ipVersion, strings.ToLower(exec.Req.DotServer), true)
	if err != nil {
		return nil, 500, err
	}
	exec.IP = ip

	method := trace.ICMPTrace
	switch protocol {
	case "udp":
		method = trace.UDPTrace
	case "tcp":
		method = trace.TCPTrace
	}
	exec.Method = method

	dstPort := exec.Req.Port
	if dstPort == 0 {
		switch method {
		case trace.UDPTrace:
			dstPort = 33494
		case trace.TCPTrace:
			dstPort = 80
		default:
			dstPort = 0
		}
	}

	exec.DataProvider = dataProvider
	exec.PowProvider = strings.TrimSpace(exec.Req.PowProvider)
	exec.NeedsLeoWS = needsLeoWS
	exec.Config = buildTraceConfig(exec.Req, ip, dataProvider, dstPort)

	return exec, 0, nil
}

func traceHandler(c *gin.Context) {
	var req traceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request payload", "details": err.Error()})
		return
	}

	setup, statusCode, err := prepareTrace(req)
	if err != nil {
		if statusCode == 0 {
			statusCode = 500
		}
		log.Printf("[deploy] prepare trace failed target=%s error=%v", strings.TrimSpace(req.Target), err)
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[deploy] trace request target=%s proto=%s provider=%s lang=%s ipv4_only=%t ipv6_only=%t", setup.Target, setup.Protocol, setup.DataProvider, setup.Config.Lang, setup.Req.IPv4Only, setup.Req.IPv6Only)
	log.Printf("[deploy] target resolved target=%s ip=%s via dot=%s", setup.Target, setup.IP, strings.ToLower(setup.Req.DotServer))

	traceMu.Lock()
	defer traceMu.Unlock()

	prevSrcPort := util.SrcPort
	prevDstIP := util.DstIP
	prevSrcDev := util.SrcDev
	prevDisableMPLS := util.DisableMPLS
	prevPowProvider := util.PowProviderParam
	defer func() {
		util.SrcPort = prevSrcPort
		util.DstIP = prevDstIP
		util.SrcDev = prevSrcDev
		util.DisableMPLS = prevDisableMPLS
		util.PowProviderParam = prevPowProvider
	}()

	if setup.NeedsLeoWS {
		if setup.PowProvider != "" {
			log.Printf("[deploy] LeoMoeAPI using custom PoW provider=%s", setup.PowProvider)
		} else {
			log.Printf("[deploy] LeoMoeAPI using default PoW provider")
		}
		util.PowProviderParam = setup.PowProvider
		ensureLeoMoeConnection()
	} else if setup.PowProvider != "" {
		log.Printf("[deploy] overriding PoW provider=%s", setup.PowProvider)
		util.PowProviderParam = setup.PowProvider
	} else {
		util.PowProviderParam = ""
	}

	util.SrcPort = setup.Req.SourcePort
	util.DstIP = setup.IP.String()
	if setup.Req.SourceDevice != "" {
		util.SrcDev = setup.Req.SourceDevice
	} else {
		util.SrcDev = ""
	}
	util.DisableMPLS = setup.Req.DisableMPLS

	configured := setup.Config
	log.Printf("[deploy] starting trace target=%s resolved=%s method=%s lang=%s queries=%d maxHops=%d", setup.Target, setup.IP.String(), string(setup.Method), configured.Lang, configured.NumMeasurements, configured.MaxHops)

	start := time.Now()
	res, err := trace.Traceroute(setup.Method, configured)
	duration := time.Since(start)
	if err != nil {
		log.Printf("[deploy] trace failed target=%s error=%v", setup.Target, err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	traceMapURL := ""
	if configured.Maptrace && shouldGenerateMap(setup.DataProvider) {
		if payload, err := json.Marshal(res); err == nil {
			if mapUrl, err := tracemap.GetMapUrl(string(payload)); err == nil {
				traceMapURL = mapUrl
				log.Printf("[deploy] trace map generated target=%s mapUrl=%s", setup.Target, traceMapURL)
			}
		}
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

	log.Printf("[deploy] trace completed target=%s hops=%d duration=%s", setup.Target, len(response.Hops), duration)
	c.JSON(200, response)
}

func buildTraceConfig(req traceRequest, ip net.IP, dataProvider string, port int) trace.Config {
	lang := strings.TrimSpace(req.Language)
	if lang == "" {
		lang = defaults["language"].(string)
	}

	timeout := req.TimeoutMs
	if timeout <= 0 {
		timeout = defaults["timeout_ms"].(int)
	}

	packetSize := req.PacketSize
	if packetSize <= 0 {
		packetSize = defaults["packet_size"].(int)
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
		BeginHop:         beginHop,
		MaxHops:          maxHops,
		NumMeasurements:  queries,
		MaxAttempts:      req.MaxAttempts,
		ParallelRequests: parallel,
		Timeout:          time.Duration(timeout) * time.Millisecond,
		DstIP:            ip,
		DstPort:          port,
		IPGeoSource:      ipgeo.GetSource(dataProvider),
		RDNS:             !req.DisableRDNS,
		AlwaysWaitRDNS:   alwaysWait,
		PacketInterval:   req.PacketInterval,
		TTLInterval:      req.TTLInterval,
		Lang:             lang,
		DN42:             req.DN42,
		PktSize:          packetSize,
		Maptrace:         !req.DisableMaptrace,
	}
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

func normalizeTarget(input string) (string, error) {
	target := strings.TrimSpace(input)
	if target == "" {
		return "", errors.New("target is required")
	}

	fallbackSource := target
	host := ""

	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil {
			return "", fmt.Errorf("invalid target format: %w", err)
		}
		if u.Host != "" {
			host = u.Host
		} else if u.Path != "" {
			fallbackSource = strings.TrimPrefix(target, u.Scheme+"://")
		}
	}

	if host == "" && strings.Contains(target, "/") {
		parseTarget := target
		if !strings.HasPrefix(parseTarget, "//") {
			parseTarget = "//" + parseTarget
		}
		if u, err := url.Parse(parseTarget); err == nil && u.Host != "" {
			host = u.Host
		} else {
			fallbackSource = target
		}
	}

	if host == "" && strings.Contains(fallbackSource, "/") {
		idx := strings.Index(fallbackSource, "/")
		if idx <= 0 {
			return "", errors.New("invalid target format")
		}
		candidate := strings.TrimSpace(fallbackSource[:idx])
		if candidate == "" {
			return "", errors.New("invalid target format")
		}
		host = candidate
	}

	if host != "" {
		target = host
	}

	if strings.Contains(target, "]") && strings.Contains(target, "[") {
		target = strings.Split(strings.Split(target, "]")[0], "[")[1]
	} else if strings.Count(target, ":") == 1 {
		if host, _, err := net.SplitHostPort(target); err == nil {
			target = host
		} else {
			target = strings.Split(target, ":")[0]
		}
	}

	return strings.TrimSpace(target), nil
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
