package ipgeo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/util"
	"golang.org/x/net/http/httpproxy"
)

// NextTraceAPIV4MinTimeout is the minimum total budget for a v4 GeoIP lookup.
const NextTraceAPIV4MinTimeout = 2 * time.Second

const (
	NextTraceAPIV4TokenPageURL  = "https://api.nxtrace.org/v4/api-tokens"
	nextTraceAPIV4APIHost       = "api.nxtrace.org"
	nextTraceAPIV4DefaultPort   = "443"
	nextTraceAPIV4GeoPath       = "/v4/ipGeo"
	nextTraceAPIV4TokenHeader   = "X-NextTrace-Token"
	nextTraceAPIV4MaxErrorBody  = 512
	nextTraceAPIV4MaxGeoBody    = 1 << 20
	nextTraceAPIV4FastIPTimeout = time.Second
	nextTraceAPIV4MaxAttempts   = 3
	nextTraceAPIV4CacheMaxSize  = 32
)

var (
	nextTraceAPIV4GeoEndpoint         = "https://api.nxtrace.org/v4/ipGeo"
	nextTraceAPIV4RetryDelays         = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond}
	nextTraceAPIV4TransportCache      = make(map[nextTraceAPIV4TransportCacheKey]*http.Transport)
	nextTraceAPIV4TransportCacheOrder []nextTraceAPIV4TransportCacheKey
	nextTraceAPIV4TransportCacheMu    sync.RWMutex
)

type nextTraceAPIV4TransportFactoryFunc func(endpoint string, policy util.GeoDNSPolicy, proxy nextTraceAPIV4ProxyPolicy) *http.Transport

var nextTraceAPIV4TransportFactory nextTraceAPIV4TransportFactoryFunc = newNextTraceAPIV4Transport
var nextTraceAPIV4CloseIdleConnections = (*http.Transport).CloseIdleConnections
var nextTraceAPIV4FastIPFn = util.GetFastIPWithContext

type nextTraceAPIV4TransportCacheKey struct {
	endpoint      string
	geoDNSPolicy  util.GeoDNSPolicy
	proxyIdentity string
}

type nextTraceAPIV4ProxyPolicy struct {
	identity string
	proxy    func(*http.Request) (*url.URL, error)
}

type NextTraceAPIV4Quota struct {
	Remaining    uint64
	HasRemaining bool
	ExpiresAt    time.Time
	HasExpiresAt bool
	Cost         uint64
	HasCost      bool
	Source       string
}

type NextTraceAPIV4Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

func NewNextTraceAPIV4Client(endpoint string, token string, httpClient *http.Client) *NextTraceAPIV4Client {
	endpoint = normalizeNextTraceAPIV4Endpoint(endpoint)
	if httpClient == nil {
		httpClient = defaultNextTraceAPIV4HTTPClient(endpoint, NextTraceAPIV4MinTimeout)
	}
	return &NextTraceAPIV4Client{
		endpoint:   endpoint,
		token:      strings.TrimSpace(token),
		httpClient: httpClient,
	}
}

func NextTraceAPIV4TokenConfigured() bool {
	return strings.TrimSpace(util.GetNextTraceAPIV4Token()) != ""
}

// NextTraceAPISource selects v4 when a token is configured and otherwise uses v3.
func NextTraceAPISource() Source {
	if NextTraceAPIV4TokenConfigured() {
		return NextTraceAPIV4GeoIP
	}
	return NextTraceAPIV3GeoIP
}

// NextTraceAPIV4GeoIP queries the NextTrace API v4 HTTP service.
func NextTraceAPIV4GeoIP(ip string, timeout time.Duration, lang string, maptrace bool) (*IPGeoData, error) {
	_ = lang
	_ = maptrace
	timeout = normalizeNextTraceAPIV4Timeout(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	endpoint := normalizeNextTraceAPIV4Endpoint(nextTraceAPIV4GeoEndpoint)
	policy := util.CurrentGeoDNSPolicy()
	proxy := captureNextTraceAPIV4Proxy(endpoint)
	_ = prepareNextTraceAPIV4FastIPWithProxy(ctx, endpoint, false, proxy)
	client := newNextTraceAPIV4ClientForPolicy(endpoint, util.GetNextTraceAPIV4Token(), timeout, policy, proxy)
	geo, _, err := client.Lookup(ctx, ip)
	return geo, err
}

// LeoMoeAPISource is kept for source compatibility.
// Deprecated: use NextTraceAPISource.
func LeoMoeAPISource() Source {
	return NextTraceAPISource()
}

// LeoIPNextTraceAPIV4HTTP is kept for source compatibility.
// Deprecated: use NextTraceAPIV4GeoIP.
func LeoIPNextTraceAPIV4HTTP(ip string, timeout time.Duration, lang string, maptrace bool) (*IPGeoData, error) {
	return NextTraceAPIV4GeoIP(ip, timeout, lang, maptrace)
}

func newNextTraceAPIV4ClientForPolicy(endpoint string, token string, timeout time.Duration, policy util.GeoDNSPolicy, proxy nextTraceAPIV4ProxyPolicy) *NextTraceAPIV4Client {
	return NewNextTraceAPIV4Client(endpoint, token, newNextTraceAPIV4HTTPClient(endpoint, timeout, policy, proxy))
}

func defaultNextTraceAPIV4HTTPClient(endpoint string, timeout time.Duration) *http.Client {
	policy := util.CurrentGeoDNSPolicy()
	proxy := captureNextTraceAPIV4Proxy(endpoint)
	return newNextTraceAPIV4HTTPClient(endpoint, timeout, policy, proxy)
}

func newNextTraceAPIV4HTTPClient(endpoint string, timeout time.Duration, policy util.GeoDNSPolicy, proxy nextTraceAPIV4ProxyPolicy) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: cachedNextTraceAPIV4Transport(endpoint, policy, proxy),
	}
}

func cachedNextTraceAPIV4Transport(endpoint string, policy util.GeoDNSPolicy, proxy nextTraceAPIV4ProxyPolicy) *http.Transport {
	policy = util.NewGeoDNSPolicy(policy.Resolver(), policy.Fallback())
	key := nextTraceAPIV4TransportCacheKey{
		endpoint:      normalizeNextTraceAPIV4Endpoint(endpoint),
		geoDNSPolicy:  policy,
		proxyIdentity: proxy.identity,
	}

	nextTraceAPIV4TransportCacheMu.RLock()
	if transport := nextTraceAPIV4TransportCache[key]; transport != nil {
		nextTraceAPIV4TransportCacheMu.RUnlock()
		return transport
	}
	nextTraceAPIV4TransportCacheMu.RUnlock()

	nextTraceAPIV4TransportCacheMu.Lock()
	if transport := nextTraceAPIV4TransportCache[key]; transport != nil {
		nextTraceAPIV4TransportCacheMu.Unlock()
		return transport
	}

	transport := nextTraceAPIV4TransportFactory(key.endpoint, policy, proxy)
	nextTraceAPIV4TransportCache[key] = transport
	nextTraceAPIV4TransportCacheOrder = append(nextTraceAPIV4TransportCacheOrder, key)
	evicted := evictNextTraceAPIV4TransportCacheLocked()
	nextTraceAPIV4TransportCacheMu.Unlock()
	closeNextTraceAPIV4TransportIdleConnections(evicted)
	return transport
}

func evictNextTraceAPIV4TransportCacheLocked() []*http.Transport {
	evictCount := len(nextTraceAPIV4TransportCache) - nextTraceAPIV4CacheMaxSize
	if evictCount <= 0 {
		return nil
	}
	if evictCount > len(nextTraceAPIV4TransportCacheOrder) {
		evictCount = len(nextTraceAPIV4TransportCacheOrder)
	}

	evictedKeys := append([]nextTraceAPIV4TransportCacheKey(nil), nextTraceAPIV4TransportCacheOrder[:evictCount]...)
	copy(nextTraceAPIV4TransportCacheOrder, nextTraceAPIV4TransportCacheOrder[evictCount:])
	for i := len(nextTraceAPIV4TransportCacheOrder) - evictCount; i < len(nextTraceAPIV4TransportCacheOrder); i++ {
		nextTraceAPIV4TransportCacheOrder[i] = nextTraceAPIV4TransportCacheKey{}
	}
	nextTraceAPIV4TransportCacheOrder = nextTraceAPIV4TransportCacheOrder[:len(nextTraceAPIV4TransportCacheOrder)-evictCount]

	evicted := make([]*http.Transport, 0, len(evictedKeys))
	for _, key := range evictedKeys {
		transport := nextTraceAPIV4TransportCache[key]
		delete(nextTraceAPIV4TransportCache, key)
		if transport != nil {
			evicted = append(evicted, transport)
		}
	}
	return evicted
}

func closeNextTraceAPIV4TransportIdleConnections(transports []*http.Transport) {
	for _, transport := range transports {
		nextTraceAPIV4CloseIdleConnections(transport)
	}
}

func normalizeNextTraceAPIV4Endpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nextTraceAPIV4GeoEndpoint
	}
	return endpoint
}

func PrepareNextTraceAPIV4FastIP(ctx context.Context, enableOutput bool) error {
	return prepareNextTraceAPIV4FastIP(ctx, nextTraceAPIV4GeoEndpoint, enableOutput)
}

func prepareNextTraceAPIV4FastIP(ctx context.Context, endpoint string, enableOutput bool) error {
	return prepareNextTraceAPIV4FastIPWithProxy(ctx, endpoint, enableOutput, captureNextTraceAPIV4Proxy(endpoint))
}

func prepareNextTraceAPIV4FastIPWithProxy(ctx context.Context, endpoint string, enableOutput bool, proxy nextTraceAPIV4ProxyPolicy) error {
	host, port, ok := nextTraceAPIV4APIEndpointHostPort(endpoint)
	if !ok || nextTraceAPIV4ProxyApplies(proxy, endpoint) {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	prewarmCtx, cancel := context.WithTimeout(ctx, nextTraceAPIV4FastIPTimeout)
	defer cancel()
	_, err := nextTraceAPIV4FastIPFn(prewarmCtx, host, port, enableOutput)
	return err
}

func captureNextTraceAPIV4Proxy(_ string) nextTraceAPIV4ProxyPolicy {
	if proxyURL := util.GetProxy(); proxyURL != nil {
		cloned := *proxyURL
		return nextTraceAPIV4ProxyPolicy{
			identity: "explicit:" + proxyURL.String(),
			proxy:    http.ProxyURL(&cloned),
		}
	}

	config := httpproxy.FromEnvironment()
	proxyForURL := config.ProxyFunc()
	return nextTraceAPIV4ProxyPolicy{
		identity: strings.Join([]string{
			"environment",
			config.HTTPProxy,
			config.HTTPSProxy,
			config.NoProxy,
			strconv.FormatBool(config.CGI),
		}, "\x00"),
		proxy: func(req *http.Request) (*url.URL, error) {
			return proxyForURL(req.URL)
		},
	}
}

func nextTraceAPIV4ProxyApplies(policy nextTraceAPIV4ProxyPolicy, endpoint string) bool {
	if policy.proxy == nil {
		return false
	}
	u, err := url.Parse(normalizeNextTraceAPIV4Endpoint(endpoint))
	if err != nil {
		return true
	}
	proxyURL, err := policy.proxy(&http.Request{URL: u})
	return err != nil || proxyURL != nil
}

func nextTraceAPIV4APIEndpointHostPort(endpoint string) (string, string, bool) {
	u, err := url.Parse(normalizeNextTraceAPIV4Endpoint(endpoint))
	if err != nil {
		return "", "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return "", "", false
	}
	if !strings.EqualFold(u.Hostname(), nextTraceAPIV4APIHost) || u.EscapedPath() != nextTraceAPIV4GeoPath {
		return "", "", false
	}
	port := u.Port()
	if port == "" {
		if scheme == "http" {
			port = "80"
		} else {
			port = nextTraceAPIV4DefaultPort
		}
	}
	return nextTraceAPIV4APIHost, port, true
}

func newNextTraceAPIV4Transport(endpoint string, policy util.GeoDNSPolicy, proxy nextTraceAPIV4ProxyPolicy) *http.Transport {
	baseClient := util.NewSharedGeoHTTPClientWithPolicy(0, policy)
	transport := &http.Transport{}
	if base, ok := baseClient.Transport.(*http.Transport); ok && base != nil {
		transport = base.Clone()
	} else if base, ok := http.DefaultTransport.(*http.Transport); ok && base != nil {
		transport = base.Clone()
	}

	transport.Proxy = proxy.proxy

	host, port, ok := nextTraceAPIV4APIEndpointHostPort(endpoint)
	if !ok || nextTraceAPIV4ProxyApplies(proxy, endpoint) {
		return transport
	}

	fallbackDial := transport.DialContext
	if fallbackDial == nil {
		fallbackDial = (&net.Dialer{KeepAlive: 30 * time.Second}).DialContext
	}
	fastIPDialer := &net.Dialer{KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialNextTraceAPIV4(ctx, fastIPDialer, fallbackDial, network, addr, host, port)
	}
	return transport
}

func dialNextTraceAPIV4(ctx context.Context, fastIPDialer *net.Dialer, fallbackDial func(context.Context, string, string) (net.Conn, error), network string, addr string, apiHost string, apiPort string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fallbackDial(ctx, network, addr)
	}
	if strings.EqualFold(host, apiHost) && port == apiPort {
		if fastIP := strings.Trim(util.GetFastIPCache(), "[]"); fastIP != "" {
			if conn, dialErr := fastIPDialer.DialContext(ctx, network, net.JoinHostPort(fastIP, port)); dialErr == nil {
				return conn, nil
			}
		}
	}
	return fallbackDial(ctx, network, addr)
}

func (c *NextTraceAPIV4Client) Lookup(ctx context.Context, ip string) (*IPGeoData, NextTraceAPIV4Quota, error) {
	if c == nil {
		return nil, NextTraceAPIV4Quota{}, errors.New("NextTrace API v4 GeoIP client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := c.totalLookupContext(ctx)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < nextTraceAPIV4MaxAttempts; attempt++ {
		geo, quota, err := c.lookupOnce(ctx, ip)
		if err == nil {
			return geo, quota, nil
		}
		lastErr = err
		if !shouldRetryNextTraceAPIV4Lookup(err) || attempt == nextTraceAPIV4MaxAttempts-1 || ctx.Err() != nil {
			return nil, NextTraceAPIV4Quota{}, lastErr
		}
		if err := sleepBeforeNextTraceAPIV4Retry(ctx, nextTraceAPIV4RetryDelay(attempt)); err != nil {
			return nil, NextTraceAPIV4Quota{}, lastErr
		}
	}
	return nil, NextTraceAPIV4Quota{}, lastErr
}

func (c *NextTraceAPIV4Client) lookupOnce(ctx context.Context, ip string) (*IPGeoData, NextTraceAPIV4Quota, error) {
	req, err := c.newLookupRequest(ctx, ip)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, retryableNextTraceAPIV4Error("NextTrace API v4 GeoIP request failed: %s", err, c.token)
	}
	defer resp.Body.Close()

	bodyLimit := int64(nextTraceAPIV4MaxGeoBody)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyLimit = nextTraceAPIV4MaxErrorBody
	}
	body, truncated, err := readNextTraceAPIV4Body(resp.Body, bodyLimit)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, retryableNextTraceAPIV4Error("NextTrace API v4 GeoIP read failed: %s", err, c.token)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NextTraceAPIV4Quota{}, c.httpError(resp.StatusCode, resp.Status, body, truncated)
	}
	if truncated {
		return nil, NextTraceAPIV4Quota{}, fmt.Errorf("NextTrace API v4 GeoIP response body exceeds %d bytes", nextTraceAPIV4MaxGeoBody)
	}

	geo, err := decodeNextTraceAPIV4Geo(body)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, fmt.Errorf("NextTrace API v4 GeoIP returned invalid JSON: %w", err)
	}
	return geo, parseNextTraceAPIV4Quota(resp.Header), nil
}

func (c *NextTraceAPIV4Client) totalLookupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.httpClient == nil || c.httpClient.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.httpClient.Timeout)
}

func (c *NextTraceAPIV4Client) newLookupRequest(ctx context.Context, ip string) (*http.Request, error) {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("NextTrace API v4 GeoIP endpoint is invalid: %w", err)
	}
	q := u.Query()
	q.Set("ip", ip)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("NextTrace API v4 GeoIP request build failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", util.UserAgent)
	req.Header.Set(nextTraceAPIV4TokenHeader, c.token)
	return req, nil
}

func readNextTraceAPIV4Body(r io.Reader, limit int64) ([]byte, bool, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) <= limit {
		return body, false, nil
	}
	return body[:limit], true, nil
}

func (c *NextTraceAPIV4Client) httpError(statusCode int, status string, body []byte, truncated bool) error {
	msg := parseNextTraceAPIV4ErrorMessage(body)
	if msg == "" {
		msg = boundedNextTraceAPIV4Body(body, truncated)
	}
	if msg == "" {
		msg = status
	}
	msg = redactNextTraceAPIV4Token(msg, c.token)
	return &nextTraceAPIV4HTTPError{
		statusCode: statusCode,
		status:     status,
		message:    msg,
	}
}

type nextTraceAPIV4HTTPError struct {
	statusCode int
	status     string
	message    string
}

func (e *nextTraceAPIV4HTTPError) Error() string {
	return fmt.Sprintf("NextTrace API v4 GeoIP returned HTTP %s: %s", e.status, e.message)
}

type nextTraceAPIV4RetryableError struct {
	message string
}

func (e *nextTraceAPIV4RetryableError) Error() string {
	return e.message
}

func retryableNextTraceAPIV4Error(format string, err error, token string) error {
	return &nextTraceAPIV4RetryableError{
		message: fmt.Sprintf(format, redactNextTraceAPIV4Token(err.Error(), token)),
	}
}

func shouldRetryNextTraceAPIV4Lookup(err error) bool {
	if err == nil {
		return false
	}
	var retryable *nextTraceAPIV4RetryableError
	if errors.As(err, &retryable) {
		return true
	}
	var httpErr *nextTraceAPIV4HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.statusCode == http.StatusRequestTimeout || httpErr.statusCode >= 500
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func nextTraceAPIV4RetryDelay(attempt int) time.Duration {
	if attempt < 0 || attempt >= len(nextTraceAPIV4RetryDelays) {
		return 0
	}
	return nextTraceAPIV4RetryDelays[attempt]
}

func sleepBeforeNextTraceAPIV4Retry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func normalizeNextTraceAPIV4Timeout(timeout time.Duration) time.Duration {
	if timeout < NextTraceAPIV4MinTimeout {
		return NextTraceAPIV4MinTimeout
	}
	return timeout
}

type nextTraceAPIV4ErrorBody struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func parseNextTraceAPIV4ErrorMessage(body []byte) string {
	var parsed nextTraceAPIV4ErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Error.Message)
}

func boundedNextTraceAPIV4Body(body []byte, truncated bool) string {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) > nextTraceAPIV4MaxErrorBody {
		body = body[:nextTraceAPIV4MaxErrorBody]
	}
	msg := string(body)
	if truncated {
		if msg == "" {
			return fmt.Sprintf("response body truncated at %d bytes", nextTraceAPIV4MaxErrorBody)
		}
		return fmt.Sprintf("%s... [truncated at %d bytes]", msg, nextTraceAPIV4MaxErrorBody)
	}
	return msg
}

func redactNextTraceAPIV4Token(s string, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[REDACTED]")
}

type nextTraceAPIV4GeoWire struct {
	IP        string          `json:"ip"`
	Asnumber  string          `json:"asnumber"`
	Country   string          `json:"country"`
	CountryEn string          `json:"country_en"`
	Prov      string          `json:"prov"`
	ProvEn    string          `json:"prov_en"`
	City      string          `json:"city"`
	CityEn    string          `json:"city_en"`
	District  string          `json:"district"`
	Owner     string          `json:"owner"`
	Isp       string          `json:"isp"`
	Domain    string          `json:"domain"`
	Whois     string          `json:"whois"`
	Lat       float64         `json:"lat"`
	Lng       float64         `json:"lng"`
	Prefix    string          `json:"prefix"`
	Router    json.RawMessage `json:"router"`
	Source    string          `json:"source"`
}

func decodeNextTraceAPIV4Geo(body []byte) (*IPGeoData, error) {
	var wire nextTraceAPIV4GeoWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, err
	}
	router, err := decodeNextTraceAPIV4Router(wire.Router)
	if err != nil {
		return nil, err
	}
	owner := wire.Domain
	if owner == "" {
		owner = wire.Owner
	}
	return &IPGeoData{
		IP:        wire.IP,
		Asnumber:  wire.Asnumber,
		Country:   wire.Country,
		CountryEn: wire.CountryEn,
		Prov:      wire.Prov,
		ProvEn:    wire.ProvEn,
		City:      wire.City,
		CityEn:    wire.CityEn,
		District:  wire.District,
		Owner:     owner,
		Isp:       wire.Isp,
		Domain:    wire.Domain,
		Whois:     wire.Whois,
		Lat:       wire.Lat,
		Lng:       wire.Lng,
		Prefix:    wire.Prefix,
		Router:    router,
		Source:    wire.Source,
	}, nil
}

func decodeNextTraceAPIV4Router(raw json.RawMessage) (map[string][]string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == `""` {
		return nil, nil
	}
	var router map[string][]string
	if err := json.Unmarshal(raw, &router); err != nil {
		return nil, fmt.Errorf("decode router: %w", err)
	}
	return router, nil
}

func parseNextTraceAPIV4Quota(header http.Header) NextTraceAPIV4Quota {
	var quota NextTraceAPIV4Quota
	if value, ok := parseNextTraceAPIV4UintHeader(header, "X-NextTrace-Quota-Remaining"); ok {
		quota.Remaining = value
		quota.HasRemaining = true
	}
	if value, ok := parseNextTraceAPIV4UintHeader(header, "X-NextTrace-Quota-Cost"); ok {
		quota.Cost = value
		quota.HasCost = true
	}
	if expires := strings.TrimSpace(header.Get("X-NextTrace-Quota-Expires-At")); expires != "" {
		if value, err := time.Parse(time.RFC3339, expires); err == nil {
			quota.ExpiresAt = value
			quota.HasExpiresAt = true
		}
	}
	quota.Source = strings.TrimSpace(header.Get("X-NextTrace-Quota-Source"))
	return quota
}

func parseNextTraceAPIV4UintHeader(header http.Header, key string) (uint64, bool) {
	raw := strings.TrimSpace(header.Get(key))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
