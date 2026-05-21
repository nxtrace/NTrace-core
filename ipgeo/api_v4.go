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
	APIV4TokenPageURL = "https://api.nxtrace.org/v4/api-tokens"
	apiV4TokenHeader  = "X-NextTrace-Token"
	apiV4MaxErrorBody = 512
)

var (
	apiV4GeoEndpoint       = "https://api.nxtrace.org/v4/ipGeo"
	apiV4HTTPClientFactory = util.NewGeoHTTPClient
)

type APIV4Quota struct {
	Remaining    uint64
	HasRemaining bool
	ExpiresAt    time.Time
	HasExpiresAt bool
	Cost         uint64
	HasCost      bool
	Source       string
}

type APIV4Client struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

func NewAPIV4Client(endpoint string, token string, httpClient *http.Client) *APIV4Client {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = apiV4GeoEndpoint
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &APIV4Client{
		endpoint:   endpoint,
		token:      strings.TrimSpace(token),
		httpClient: httpClient,
	}
}

func APIV4TokenConfigured() bool {
	return strings.TrimSpace(util.GetAPIV4Token()) != ""
}

func LeoMoeAPISource() Source {
	if APIV4TokenConfigured() {
		return LeoIPV4HTTP
	}
	return LeoIP
}

func LeoIPV4HTTP(ip string, timeout time.Duration, lang string, maptrace bool) (*IPGeoData, error) {
	_ = lang
	_ = maptrace
	if timeout <= 0 {
		timeout = time.Second
	}
	client := NewAPIV4Client(apiV4GeoEndpoint, util.GetAPIV4Token(), apiV4HTTPClientFactory(timeout))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	geo, _, err := client.Lookup(ctx, ip)
	return geo, err
}

func (c *APIV4Client) Lookup(ctx context.Context, ip string) (*IPGeoData, APIV4Quota, error) {
	if c == nil {
		return nil, APIV4Quota{}, errors.New("v4 GeoIP API client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := c.newLookupRequest(ctx, ip)
	if err != nil {
		return nil, APIV4Quota{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, APIV4Quota{}, fmt.Errorf("v4 GeoIP API request failed: %s", redactAPIV4Token(err.Error(), c.token))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, APIV4Quota{}, fmt.Errorf("v4 GeoIP API read failed: %s", redactAPIV4Token(err.Error(), c.token))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, APIV4Quota{}, c.httpError(resp.Status, body)
	}

	geo, err := decodeAPIV4Geo(body)
	if err != nil {
		return nil, APIV4Quota{}, fmt.Errorf("v4 GeoIP API returned invalid JSON: %w", err)
	}
	return geo, parseAPIV4Quota(resp.Header), nil
}

func (c *APIV4Client) newLookupRequest(ctx context.Context, ip string) (*http.Request, error) {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("v4 GeoIP API endpoint is invalid: %w", err)
	}
	q := u.Query()
	q.Set("ip", ip)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("v4 GeoIP API request build failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", util.UserAgent)
	req.Header.Set(apiV4TokenHeader, c.token)
	return req, nil
}

func (c *APIV4Client) httpError(status string, body []byte) error {
	msg := parseAPIV4ErrorMessage(body)
	if msg == "" {
		msg = boundedAPIV4Body(body)
	}
	if msg == "" {
		msg = status
	}
	msg = redactAPIV4Token(msg, c.token)
	return fmt.Errorf("v4 GeoIP API returned HTTP %s: %s", status, msg)
}

type apiV4ErrorBody struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func parseAPIV4ErrorMessage(body []byte) string {
	var parsed apiV4ErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Error.Message)
}

func boundedAPIV4Body(body []byte) string {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) > apiV4MaxErrorBody {
		body = body[:apiV4MaxErrorBody]
	}
	return string(body)
}

func redactAPIV4Token(s string, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[REDACTED]")
}

type apiV4GeoWire struct {
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

func decodeAPIV4Geo(body []byte) (*IPGeoData, error) {
	var wire apiV4GeoWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, err
	}
	router, err := decodeAPIV4Router(wire.Router)
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

func decodeAPIV4Router(raw json.RawMessage) (map[string][]string, error) {
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

func parseAPIV4Quota(header http.Header) APIV4Quota {
	var quota APIV4Quota
	if value, ok := parseAPIV4UintHeader(header, "X-NextTrace-Quota-Remaining"); ok {
		quota.Remaining = value
		quota.HasRemaining = true
	}
	if value, ok := parseAPIV4UintHeader(header, "X-NextTrace-Quota-Cost"); ok {
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

func parseAPIV4UintHeader(header http.Header, key string) (uint64, bool) {
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
