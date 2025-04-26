package ipgeo

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/config"
	"github.com/nxtrace/NTrace-core/util"

	"github.com/tidwall/gjson"
)

// Language mapping for IPDB.One API
var LangMap = map[string]string{
	"en": "en",
	"cn": "zh",
}

// IPDBOneConfig holds the configuration for IPDB.One service
type IPDBOneConfig struct {
	BaseURL string
	ApiID   string
	ApiKey  string
}

// GetDefaultConfig returns the default configuration with fallback values
func GetDefaultConfig() *IPDBOneConfig {
	return &IPDBOneConfig{
		BaseURL: util.GetenvDefault("IPDBONE_BASE_URL", "https://api.ipdb.one"),
		ApiID:   util.GetenvDefault("IPDBONE_API_ID", ""),
		ApiKey:  util.GetenvDefault("IPDBONE_API_KEY", ""),
	}
}

// IPDBOneTokenCache manages the caching of auth tokens
type IPDBOneTokenCache struct {
	token     string
	expiresAt time.Time
	mutex     sync.RWMutex
}

// GetToken retrieves cached token if valid, otherwise returns empty string
func (c *IPDBOneTokenCache) GetToken() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.token == "" || time.Now().After(c.expiresAt) {
		return ""
	}
	return c.token
}

// SetToken updates the token with its expiration time
func (c *IPDBOneTokenCache) SetToken(token string, expiresIn time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.token = token
	c.expiresAt = time.Now().Add(expiresIn)
}

// IPDBOneClient handles communication with IPDB.One API
type IPDBOneClient struct {
	config     *IPDBOneConfig
	tokenCache *IPDBOneTokenCache
	tokenInit  sync.Once
	httpClient *http.Client
}

// NewIPDBOneClient creates a new client for IPDB.One with default configuration
func NewIPDBOneClient() *IPDBOneClient {
	return &IPDBOneClient{
		config:     GetDefaultConfig(),
		tokenCache: &IPDBOneTokenCache{},
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// fetchToken requests a new authentication token from the API
func (c *IPDBOneClient) fetchToken() error {
	authURL := c.config.BaseURL + "/auth/requestToken/query"

	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NextTrace/"+config.Version)
	req.Header.Set("x-api-id", c.config.ApiID)
	req.Header.Set("x-api-key", c.config.ApiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	statusCode := gjson.Get(string(body), "code").Int()
	statusMessage := gjson.Get(string(body), "message").String()

	if statusCode != 200 {
		return errors.New("failed to authenticate: " + statusMessage)
	}

	token := gjson.Get(string(body), "data.token").String()
	if token == "" {
		return errors.New("authentication failed: empty token received")
	}

	// Cache token with a 30-second expiration
	c.tokenCache.SetToken(token, 30*time.Second)
	return nil
}

// ensureToken makes sure a valid token is available, fetching a new one if needed
func (c *IPDBOneClient) ensureToken() error {
	var initErr error

	// Ensure API credentials are set
	if c.config.ApiID == "" || c.config.ApiKey == "" {
		return errors.New("api id or api key is not set")
	}

	// Initialize token the first time this is called
	c.tokenInit.Do(func() {
		initErr = c.fetchToken()
	})

	if initErr != nil {
		return initErr
	}

	// If token expired or not available, get a new one
	if c.tokenCache.GetToken() == "" {
		return c.fetchToken()
	}

	return nil
}

// LookupIP queries the IP information from IPDB.One
func (c *IPDBOneClient) LookupIP(ip string, lang string) (*IPGeoData, error) {

	// Ensure we have a valid token
	if err := c.ensureToken(); err != nil {
		return &IPGeoData{}, nil
	}

	// Map language code if needed
	langCode, ok := LangMap[lang]
	if !ok {
		langCode = "en" // Default to English
	}

	// Query the IP information
	queryURL := c.config.BaseURL + "/query/" + ip + "?lang=" + langCode

	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NextTrace/"+config.Version)
	req.Header.Set("Authorization", "Bearer "+c.tokenCache.GetToken())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	statusCode := gjson.Get(string(body), "code").Int()
	if statusCode != 200 {
		return nil, errors.New("failed to get IP info: " + gjson.Get(string(body), "message").String())
	}

	return parseIPDBOneResponse(ip, body)
}

// parseIPDBOneResponse converts the API response to an IPGeoData struct
func parseIPDBOneResponse(ip string, responseBody []byte) (*IPGeoData, error) {
	data := gjson.Get(string(responseBody), "data")
	geoData := data.Get("geo")
	routingData := data.Get("routing")

	result := &IPGeoData{
		IP: ip,
	}

	// Parse geo information if available
	if geoData.Exists() {
		coordinate := geoData.Get("coordinate")
		if coordinate.Exists() && coordinate.Type != gjson.Null && coordinate.IsArray() && len(coordinate.Array()) >= 2 {
			result.Lat = coordinate.Array()[0].Float()
			result.Lng = coordinate.Array()[1].Float()
		}

		if geoData.Get("country").Exists() && geoData.Get("country").Type != gjson.Null {
			result.Country = geoData.Get("country").String()
		}

		if geoData.Get("region").Exists() && geoData.Get("region").Type != gjson.Null {
			result.Prov = geoData.Get("region").String()
		}

		if geoData.Get("city").Exists() && geoData.Get("city").Type != gjson.Null {
			result.City = geoData.Get("city").String()
		}
	}

	// Parse routing information if available
	if routingData.Exists() {
		asnData := routingData.Get("asn")
		if asnData.Get("number").Exists() && asnData.Get("number").Type != gjson.Null {
			result.Asnumber = strconv.FormatInt(asnData.Get("number").Int(), 10)
		}

		if routingData.Get("asn.name").Exists() && routingData.Get("asn.name").Type != gjson.Null {
			result.Owner = routingData.Get("asn.name").String()
		}

		// Get domain, override owner
		if routingData.Get("asn.domain").Exists() && routingData.Get("asn.domain").Type != gjson.Null {
			result.Owner = routingData.Get("asn.domain").String()
		}

		// Get asname as Whois
		if routingData.Get("asn.asname").Exists() && routingData.Get("asn.asname").Type != gjson.Null {
			result.Whois = routingData.Get("asn.asname").String()
		}
	}

	return result, nil
}

// Global client instance for backward compatibility
var defaultClient = NewIPDBOneClient()

// IPDBOne looks up IP information from IPDB.One (maintains backward compatibility)
func IPDBOne(ip string, timeout time.Duration, lang string, _ bool) (*IPGeoData, error) {
	// Override timeout if specified
	if timeout > 0 {
		defaultClient.httpClient.Timeout = timeout
	}

	return defaultClient.LookupIP(ip, lang)
}
