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

const (
	NextTraceAPIV4TokenPageURL  = "https://api.nxtrace.org/v4/api-tokens"
	nextTraceAPIV4APIHost       = "api.nxtrace.org"
	nextTraceAPIV4DefaultPort   = "443"
	nextTraceAPIV4GeoPath       = "/v4/ipGeo"
	nextTraceAPIV4TokenHeader   = "X-NextTrace-Token"
	nextTraceAPIV4MaxErrorBody  = 512
	nextTraceAPIV4MaxGeoBody    = 1 << 20
	nextTraceAPIV4MinTimeout    = 2 * time.Second
	nextTraceAPIV4FastIPTimeout = time.Second
	nextTraceAPIV4MaxAttempts   = 3
	nextTraceAPIV4CacheMaxSize  = 32
)

var (
	nextTraceAPIV4GeoEndpoint      = "https://api.nxtrace.org/v4/ipGeo"
	nextTraceAPIV4RetryDelays      = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond}
	nextTraceAPIV4ClientCache      = make(map[nextTraceAPIV4ClientCacheKey]*NextTraceAPIV4Client)
	nextTraceAPIV4ClientCacheOrder []nextTraceAPIV4ClientCacheKey
	nextTraceAPIV4ClientCacheMu    sync.RWMutex
)

type nextTraceAPIV4HTTPClientFactoryFunc func(endpoint string, timeout time.Duration) *http.Client

var nextTraceAPIV4HTTPClientFactory nextTraceAPIV4HTTPClientFactoryFunc = newNextTraceAPIV4HTTPClient
var nextTraceAPIV4FastIPFn = util.GetFastIPWithContext

type nextTraceAPIV4ClientCacheKey struct {
	endpoint       string
	token          string
	timeout        time.Duration
	geoDNSResolver string
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
		httpClient = nextTraceAPIV4HTTPClientFactory(endpoint, nextTraceAPIV4MinTimeout)
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

func LeoMoeAPISource() Source {
	if NextTraceAPIV4TokenConfigured() {
		return LeoIPNextTraceAPIV4HTTP
	}
	return LeoIP
}

func LeoIPNextTraceAPIV4HTTP(ip string, timeout time.Duration, lang string, maptrace bool) (*IPGeoData, error) {
	_ = lang
	_ = maptrace
	timeout = normalizeNextTraceAPIV4Timeout(timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = prepareNextTraceAPIV4FastIP(ctx, nextTraceAPIV4GeoEndpoint, false)
	client := cachedNextTraceAPIV4Client(nextTraceAPIV4GeoEndpoint, util.GetNextTraceAPIV4Token(), timeout)
	geo, _, err := client.Lookup(ctx, ip)
	return geo, err
}

func cachedNextTraceAPIV4Client(endpoint string, token string, timeout time.Duration) *NextTraceAPIV4Client {
	endpoint = normalizeNextTraceAPIV4Endpoint(endpoint)
	token = strings.TrimSpace(token)
	timeout = normalizeNextTraceAPIV4Timeout(timeout)

	key := nextTraceAPIV4ClientCacheKey{
		endpoint:       endpoint,
		token:          token,
		timeout:        timeout,
		geoDNSResolver: util.CurrentGeoDNSResolver(),
	}

	nextTraceAPIV4ClientCacheMu.RLock()
	if client := nextTraceAPIV4ClientCache[key]; client != nil {
		nextTraceAPIV4ClientCacheMu.RUnlock()
		return client
	}
	nextTraceAPIV4ClientCacheMu.RUnlock()

	nextTraceAPIV4ClientCacheMu.Lock()
	defer nextTraceAPIV4ClientCacheMu.Unlock()
	if client := nextTraceAPIV4ClientCache[key]; client != nil {
		return client
	}

	client := NewNextTraceAPIV4Client(endpoint, token, nextTraceAPIV4HTTPClientFactory(endpoint, timeout))
	nextTraceAPIV4ClientCache[key] = client
	nextTraceAPIV4ClientCacheOrder = append(nextTraceAPIV4ClientCacheOrder, key)
	evictNextTraceAPIV4ClientCacheLocked()
	return client
}

func evictNextTraceAPIV4ClientCacheLocked() {
	evictCount := len(nextTraceAPIV4ClientCache) - nextTraceAPIV4CacheMaxSize
	if evictCount <= 0 {
		return
	}
	if evictCount > len(nextTraceAPIV4ClientCacheOrder) {
		evictCount = len(nextTraceAPIV4ClientCacheOrder)
	}

	evictedKeys := append([]nextTraceAPIV4ClientCacheKey(nil), nextTraceAPIV4ClientCacheOrder[:evictCount]...)
	copy(nextTraceAPIV4ClientCacheOrder, nextTraceAPIV4ClientCacheOrder[evictCount:])
	for i := len(nextTraceAPIV4ClientCacheOrder) - evictCount; i < len(nextTraceAPIV4ClientCacheOrder); i++ {
		nextTraceAPIV4ClientCacheOrder[i] = nextTraceAPIV4ClientCacheKey{}
	}
	nextTraceAPIV4ClientCacheOrder = nextTraceAPIV4ClientCacheOrder[:len(nextTraceAPIV4ClientCacheOrder)-evictCount]

	for _, key := range evictedKeys {
		client := nextTraceAPIV4ClientCache[key]
		delete(nextTraceAPIV4ClientCache, key)
		closeNextTraceAPIV4ClientIdleConnections(client)
	}
}

func closeNextTraceAPIV4ClientIdleConnections(client *NextTraceAPIV4Client) {
	if client == nil || client.httpClient == nil {
		return
	}
	client.httpClient.CloseIdleConnections()
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
	host, port, ok := nextTraceAPIV4APIEndpointHostPort(endpoint)
	if !ok || nextTraceAPIV4ProxyConfigured(endpoint) {
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

func nextTraceAPIV4ProxyConfigured(endpoint string) bool {
	if util.GetProxy() != nil {
		return true
	}
	u, err := url.Parse(normalizeNextTraceAPIV4Endpoint(endpoint))
	if err != nil {
		return false
	}
	proxyURL, err := httpproxy.FromEnvironment().ProxyFunc()(u)
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

func newNextTraceAPIV4HTTPClient(endpoint string, timeout time.Duration) *http.Client {
	client := util.NewGeoHTTPClient(timeout)
	host, port, ok := nextTraceAPIV4APIEndpointHostPort(endpoint)
	if !ok {
		return client
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		return client
	}

	if proxyURL := util.GetProxy(); proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
		return client
	}

	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialNextTraceAPIV4(ctx, dialer, network, addr, host, port)
	}
	return client
}

func dialNextTraceAPIV4(ctx context.Context, dialer *net.Dialer, network string, addr string, apiHost string, apiPort string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return dialer.DialContext(ctx, network, addr)
	}
	if strings.EqualFold(host, apiHost) && port == apiPort {
		if fastIP := strings.Trim(util.GetFastIPCache(), "[]"); fastIP != "" {
			if conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(fastIP, port)); dialErr == nil {
				return conn, nil
			}
		}
	}
	return dialGeoResolved(ctx, dialer, network, host, port)
}

func dialGeoResolved(ctx context.Context, dialer *net.Dialer, network string, host string, port string) (net.Conn, error) {
	ips, err := util.LookupHostForGeo(ctx, host)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		return nil, fmt.Errorf("geo DNS returned no IPs for host %q", host)
	}
	return nil, lastErr
}

func (c *NextTraceAPIV4Client) Lookup(ctx context.Context, ip string) (*IPGeoData, NextTraceAPIV4Quota, error) {
	if c == nil {
		return nil, NextTraceAPIV4Quota{}, errors.New("NextTrace API v4 GeoIP client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
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
	attemptCtx, cancel := c.lookupAttemptContext(ctx)
	defer cancel()

	req, err := c.newLookupRequest(attemptCtx, ip)
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

func (c *NextTraceAPIV4Client) lookupAttemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
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
	if timeout < nextTraceAPIV4MinTimeout {
		return nextTraceAPIV4MinTimeout
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
		Owner:     wire.Owner,
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
