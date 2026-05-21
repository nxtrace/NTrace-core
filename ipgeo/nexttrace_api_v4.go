package ipgeo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/util"
)

const (
	NextTraceAPIV4TokenPageURL = "https://api.nxtrace.org/v4/api-tokens"
	nextTraceAPIV4TokenHeader  = "X-NextTrace-Token"
	nextTraceAPIV4MaxErrorBody = 512
)

var (
	nextTraceAPIV4GeoEndpoint       = "https://api.nxtrace.org/v4/ipGeo"
	nextTraceAPIV4HTTPClientFactory = util.NewGeoHTTPClient
)

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
	if strings.TrimSpace(endpoint) == "" {
		endpoint = nextTraceAPIV4GeoEndpoint
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
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
	if timeout <= 0 {
		timeout = time.Second
	}
	client := NewNextTraceAPIV4Client(nextTraceAPIV4GeoEndpoint, util.GetNextTraceAPIV4Token(), nextTraceAPIV4HTTPClientFactory(timeout))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	geo, _, err := client.Lookup(ctx, ip)
	return geo, err
}

func (c *NextTraceAPIV4Client) Lookup(ctx context.Context, ip string) (*IPGeoData, NextTraceAPIV4Quota, error) {
	if c == nil {
		return nil, NextTraceAPIV4Quota{}, errors.New("NextTrace API v4 GeoIP client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := c.newLookupRequest(ctx, ip)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, fmt.Errorf("NextTrace API v4 GeoIP request failed: %s", redactNextTraceAPIV4Token(err.Error(), c.token))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, fmt.Errorf("NextTrace API v4 GeoIP read failed: %s", redactNextTraceAPIV4Token(err.Error(), c.token))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, NextTraceAPIV4Quota{}, c.httpError(resp.Status, body)
	}

	geo, err := decodeNextTraceAPIV4Geo(body)
	if err != nil {
		return nil, NextTraceAPIV4Quota{}, fmt.Errorf("NextTrace API v4 GeoIP returned invalid JSON: %w", err)
	}
	return geo, parseNextTraceAPIV4Quota(resp.Header), nil
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

func (c *NextTraceAPIV4Client) httpError(status string, body []byte) error {
	msg := parseNextTraceAPIV4ErrorMessage(body)
	if msg == "" {
		msg = boundedNextTraceAPIV4Body(body)
	}
	if msg == "" {
		msg = status
	}
	msg = redactNextTraceAPIV4Token(msg, c.token)
	return fmt.Errorf("NextTrace API v4 GeoIP returned HTTP %s: %s", status, msg)
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

func boundedNextTraceAPIV4Body(body []byte) string {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) > nextTraceAPIV4MaxErrorBody {
		body = body[:nextTraceAPIV4MaxErrorBody]
	}
	return string(body)
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
