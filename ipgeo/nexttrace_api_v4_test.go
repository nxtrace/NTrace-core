package ipgeo

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/util"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type closeIdleRoundTripper struct {
	closed *int32
}

func (rt *closeIdleRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"ip":"` + req.URL.Query().Get("ip") + `"}`)),
		Request:    req,
	}, nil
}

func (rt *closeIdleRoundTripper) CloseIdleConnections() {
	atomic.AddInt32(rt.closed, 1)
}

func withNextTraceAPIV4RetryDelays(t *testing.T, delays ...time.Duration) {
	t.Helper()
	old := nextTraceAPIV4RetryDelays
	nextTraceAPIV4RetryDelays = delays
	t.Cleanup(func() {
		nextTraceAPIV4RetryDelays = old
	})
}

func resetNextTraceAPIV4ClientCache() {
	nextTraceAPIV4ClientCacheMu.Lock()
	defer nextTraceAPIV4ClientCacheMu.Unlock()
	for _, client := range nextTraceAPIV4ClientCache {
		closeNextTraceAPIV4ClientIdleConnections(client)
	}
	nextTraceAPIV4ClientCache = make(map[nextTraceAPIV4ClientCacheKey]*NextTraceAPIV4Client)
	nextTraceAPIV4ClientCacheOrder = nil
}

func nextTraceAPIV4ClientCacheLen() int {
	nextTraceAPIV4ClientCacheMu.RLock()
	defer nextTraceAPIV4ClientCacheMu.RUnlock()
	return len(nextTraceAPIV4ClientCache)
}

func TestLeoIPNextTraceAPIV4HTTPNormalizesTimeout(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFactory := nextTraceAPIV4HTTPClientFactory
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4HTTPClientFactory = oldFactory
		resetNextTraceAPIV4ClientCache()
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"1.1.1.1"}`))
	}))
	defer srv.Close()
	nextTraceAPIV4GeoEndpoint = srv.URL

	tests := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{name: "below minimum", timeout: time.Second, want: 2 * time.Second},
		{name: "zero", timeout: 0, want: 2 * time.Second},
		{name: "above minimum", timeout: 3 * time.Second, want: 3 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetNextTraceAPIV4ClientCache()
			var got time.Duration
			nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
				got = timeout
				client := srv.Client()
				client.Timeout = timeout
				return client
			}
			geo, err := LeoIPNextTraceAPIV4HTTP("1.1.1.1", tt.timeout, "cn", false)
			if err != nil {
				t.Fatalf("LeoIPNextTraceAPIV4HTTP() error = %v", err)
			}
			if geo.IP != "1.1.1.1" {
				t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
			}
			if got != tt.want {
				t.Fatalf("timeout = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLeoIPNextTraceAPIV4HTTPUsesTimeoutAsTotalBudget(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	withNextTraceAPIV4RetryDelays(t, 3*time.Second, 3*time.Second)
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFactory := nextTraceAPIV4HTTPClientFactory
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4HTTPClientFactory = oldFactory
		resetNextTraceAPIV4ClientCache()
	})

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"temporary failure"}}`))
	}))
	defer srv.Close()
	nextTraceAPIV4GeoEndpoint = srv.URL
	nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
		client := srv.Client()
		client.Timeout = timeout
		return client
	}
	resetNextTraceAPIV4ClientCache()

	start := time.Now()
	_, err := LeoIPNextTraceAPIV4HTTP("1.1.1.1", 2*time.Second, "cn", false)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("LeoIPNextTraceAPIV4HTTP() error = nil, want error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 before timeout budget stops retry", attempts)
	}
	if elapsed >= 2900*time.Millisecond {
		t.Fatalf("elapsed = %s, want bounded by 2s timeout budget", elapsed)
	}
}

func TestLeoIPNextTraceAPIV4HTTPReusesCachedClientAndConnection(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFactory := nextTraceAPIV4HTTPClientFactory
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4HTTPClientFactory = oldFactory
		resetNextTraceAPIV4ClientCache()
	})

	var factoryCalls int32
	var newConns int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"` + r.URL.Query().Get("ip") + `"}`))
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			atomic.AddInt32(&newConns, 1)
		}
	}
	srv.Start()
	defer srv.Close()

	nextTraceAPIV4GeoEndpoint = srv.URL
	nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
		atomic.AddInt32(&factoryCalls, 1)
		client := srv.Client()
		client.Timeout = timeout
		return client
	}
	resetNextTraceAPIV4ClientCache()

	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"} {
		geo, err := LeoIPNextTraceAPIV4HTTP(ip, 2*time.Second, "cn", false)
		if err != nil {
			t.Fatalf("LeoIPNextTraceAPIV4HTTP(%s) error = %v", ip, err)
		}
		if geo.IP != ip {
			t.Fatalf("IP = %q, want %s", geo.IP, ip)
		}
	}

	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("factory calls = %d, want 1 cached client", got)
	}
	if got := atomic.LoadInt32(&newConns); got != 1 {
		t.Fatalf("new connections = %d, want 1 reused HTTP connection", got)
	}
}

func TestLeoIPNextTraceAPIV4HTTPCacheKeyIncludesTokenAndTimeout(t *testing.T) {
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFactory := nextTraceAPIV4HTTPClientFactory
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4HTTPClientFactory = oldFactory
		resetNextTraceAPIV4ClientCache()
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"` + r.URL.Query().Get("ip") + `"}`))
	}))
	defer srv.Close()

	var factoryCalls int32
	nextTraceAPIV4GeoEndpoint = srv.URL
	nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
		atomic.AddInt32(&factoryCalls, 1)
		client := srv.Client()
		client.Timeout = timeout
		return client
	}
	resetNextTraceAPIV4ClientCache()

	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "token-a")
	if _, err := LeoIPNextTraceAPIV4HTTP("1.1.1.1", 2*time.Second, "cn", false); err != nil {
		t.Fatalf("first lookup error = %v", err)
	}
	if _, err := LeoIPNextTraceAPIV4HTTP("8.8.8.8", time.Second, "cn", false); err != nil {
		t.Fatalf("same normalized timeout lookup error = %v", err)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("factory calls after same key = %d, want 1", got)
	}

	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "token-b")
	if _, err := LeoIPNextTraceAPIV4HTTP("9.9.9.9", 2*time.Second, "cn", false); err != nil {
		t.Fatalf("token change lookup error = %v", err)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 2 {
		t.Fatalf("factory calls after token change = %d, want 2", got)
	}

	if _, err := LeoIPNextTraceAPIV4HTTP("1.0.0.1", 3*time.Second, "cn", false); err != nil {
		t.Fatalf("timeout change lookup error = %v", err)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 3 {
		t.Fatalf("factory calls after timeout change = %d, want 3", got)
	}
}

func TestLeoIPNextTraceAPIV4HTTPCacheKeyIncludesGeoDNSResolver(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFactory := nextTraceAPIV4HTTPClientFactory
	oldResolver := util.CurrentGeoDNSResolver()
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4HTTPClientFactory = oldFactory
		util.SetGeoDNSResolver(oldResolver)
		resetNextTraceAPIV4ClientCache()
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"` + r.URL.Query().Get("ip") + `"}`))
	}))
	defer srv.Close()

	var factoryCalls int32
	nextTraceAPIV4GeoEndpoint = srv.URL
	nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
		atomic.AddInt32(&factoryCalls, 1)
		client := srv.Client()
		client.Timeout = timeout
		return client
	}
	util.SetGeoDNSResolver("")
	resetNextTraceAPIV4ClientCache()

	if _, err := LeoIPNextTraceAPIV4HTTP("1.1.1.1", 2*time.Second, "cn", false); err != nil {
		t.Fatalf("default resolver lookup error = %v", err)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("factory calls after default resolver = %d, want 1", got)
	}

	util.SetGeoDNSResolver("google")
	if _, err := LeoIPNextTraceAPIV4HTTP("8.8.8.8", 2*time.Second, "cn", false); err != nil {
		t.Fatalf("google resolver lookup error = %v", err)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 2 {
		t.Fatalf("factory calls after google resolver = %d, want 2", got)
	}

	util.SetGeoDNSResolver("cloudflare")
	if _, err := LeoIPNextTraceAPIV4HTTP("9.9.9.9", 2*time.Second, "cn", false); err != nil {
		t.Fatalf("cloudflare resolver lookup error = %v", err)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 3 {
		t.Fatalf("factory calls after cloudflare resolver = %d, want 3", got)
	}

	util.SetGeoDNSResolver("google")
	if _, err := LeoIPNextTraceAPIV4HTTP("1.0.0.1", 2*time.Second, "cn", false); err != nil {
		t.Fatalf("google resolver reuse lookup error = %v", err)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 3 {
		t.Fatalf("factory calls after returning to google = %d, want 3", got)
	}
}

func TestNextTraceAPIV4ClientCacheNormalizesEndpoint(t *testing.T) {
	oldFactory := nextTraceAPIV4HTTPClientFactory
	t.Cleanup(func() {
		nextTraceAPIV4HTTPClientFactory = oldFactory
		resetNextTraceAPIV4ClientCache()
	})

	var factoryCalls int32
	nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Client{
			Timeout: timeout,
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("unexpected request")
			}),
		}
	}
	resetNextTraceAPIV4ClientCache()

	endpoint := "https://api.example.test/v4/ipGeo"
	client := cachedNextTraceAPIV4Client(" "+endpoint+" ", "test-token", 2*time.Second)
	cached := cachedNextTraceAPIV4Client(endpoint, "test-token", 2*time.Second)

	if client != cached {
		t.Fatal("cached client differs for endpoint with surrounding spaces")
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("factory calls = %d, want 1", got)
	}
	if client.endpoint != endpoint {
		t.Fatalf("client endpoint = %q, want %q", client.endpoint, endpoint)
	}
}

func TestNextTraceAPIV4ClientCacheEvictsOldestEntry(t *testing.T) {
	oldFactory := nextTraceAPIV4HTTPClientFactory
	oldResolver := util.CurrentGeoDNSResolver()
	t.Cleanup(func() {
		nextTraceAPIV4HTTPClientFactory = oldFactory
		util.SetGeoDNSResolver(oldResolver)
		resetNextTraceAPIV4ClientCache()
	})

	var factoryCalls int32
	var closeIdleCalls int32
	nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Client{
			Timeout:   timeout,
			Transport: &closeIdleRoundTripper{closed: &closeIdleCalls},
		}
	}
	util.SetGeoDNSResolver("")
	resetNextTraceAPIV4ClientCache()

	endpoint := "https://api.example.test/v4/ipGeo"
	timeout := 2 * time.Second
	firstKey := nextTraceAPIV4ClientCacheKey{
		endpoint: endpoint,
		token:    "token-0",
		timeout:  timeout,
	}
	firstClient := cachedNextTraceAPIV4Client(endpoint, "token-0", timeout)
	for i := 1; i <= nextTraceAPIV4CacheMaxSize; i++ {
		_ = cachedNextTraceAPIV4Client(endpoint, "token-"+strconv.Itoa(i), timeout)
	}

	if got := nextTraceAPIV4ClientCacheLen(); got != nextTraceAPIV4CacheMaxSize {
		t.Fatalf("cache size = %d, want %d", got, nextTraceAPIV4CacheMaxSize)
	}
	nextTraceAPIV4ClientCacheMu.RLock()
	_, firstStillCached := nextTraceAPIV4ClientCache[firstKey]
	nextTraceAPIV4ClientCacheMu.RUnlock()
	if firstStillCached {
		t.Fatal("oldest cache entry still present after exceeding cache size")
	}
	if got := atomic.LoadInt32(&closeIdleCalls); got != 1 {
		t.Fatalf("CloseIdleConnections calls after first eviction = %d, want 1", got)
	}

	recreated := cachedNextTraceAPIV4Client(endpoint, "token-0", timeout)
	if recreated == firstClient {
		t.Fatal("evicted cache entry reused old client")
	}
	if got := nextTraceAPIV4ClientCacheLen(); got != nextTraceAPIV4CacheMaxSize {
		t.Fatalf("cache size after recreating evicted entry = %d, want %d", got, nextTraceAPIV4CacheMaxSize)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != int32(nextTraceAPIV4CacheMaxSize+2) {
		t.Fatalf("factory calls = %d, want %d", got, nextTraceAPIV4CacheMaxSize+2)
	}
	if got := atomic.LoadInt32(&closeIdleCalls); got != 2 {
		t.Fatalf("CloseIdleConnections calls after second eviction = %d, want 2", got)
	}
}

func TestNextTraceAPIV4ClientCacheEvictsMultipleOldestEntries(t *testing.T) {
	t.Cleanup(resetNextTraceAPIV4ClientCache)
	resetNextTraceAPIV4ClientCache()

	var closeIdleCalls int32
	endpoint := "https://api.example.test/v4/ipGeo"
	timeout := 2 * time.Second
	total := nextTraceAPIV4CacheMaxSize + 3
	keys := make([]nextTraceAPIV4ClientCacheKey, 0, total)

	nextTraceAPIV4ClientCacheMu.Lock()
	for i := 0; i < total; i++ {
		key := nextTraceAPIV4ClientCacheKey{
			endpoint: endpoint,
			token:    "token-" + strconv.Itoa(i),
			timeout:  timeout,
		}
		keys = append(keys, key)
		nextTraceAPIV4ClientCache[key] = &NextTraceAPIV4Client{
			endpoint: endpoint,
			token:    key.token,
			httpClient: &http.Client{
				Timeout:   timeout,
				Transport: &closeIdleRoundTripper{closed: &closeIdleCalls},
			},
		}
		nextTraceAPIV4ClientCacheOrder = append(nextTraceAPIV4ClientCacheOrder, key)
	}
	evictNextTraceAPIV4ClientCacheLocked()
	gotLen := len(nextTraceAPIV4ClientCache)
	gotOrder := append([]nextTraceAPIV4ClientCacheKey(nil), nextTraceAPIV4ClientCacheOrder...)
	evictedStillCached := false
	for _, key := range keys[:3] {
		if nextTraceAPIV4ClientCache[key] != nil {
			evictedStillCached = true
			break
		}
	}
	nextTraceAPIV4ClientCacheMu.Unlock()

	if gotLen != nextTraceAPIV4CacheMaxSize {
		t.Fatalf("cache size = %d, want %d", gotLen, nextTraceAPIV4CacheMaxSize)
	}
	if evictedStillCached {
		t.Fatal("one of the oldest cache entries remained cached")
	}
	if !reflect.DeepEqual(gotOrder, keys[3:]) {
		t.Fatalf("cache order = %#v, want %#v", gotOrder, keys[3:])
	}
	if got := atomic.LoadInt32(&closeIdleCalls); got != 3 {
		t.Fatalf("CloseIdleConnections calls = %d, want 3", got)
	}
}

func TestLeoIPNextTraceAPIV4HTTPCachesClientConcurrently(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFactory := nextTraceAPIV4HTTPClientFactory
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4HTTPClientFactory = oldFactory
		resetNextTraceAPIV4ClientCache()
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"` + r.URL.Query().Get("ip") + `"}`))
	}))
	defer srv.Close()

	var factoryCalls int32
	nextTraceAPIV4GeoEndpoint = srv.URL
	nextTraceAPIV4HTTPClientFactory = func(timeout time.Duration) *http.Client {
		atomic.AddInt32(&factoryCalls, 1)
		client := srv.Client()
		client.Timeout = timeout
		return client
	}
	resetNextTraceAPIV4ClientCache()

	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := LeoIPNextTraceAPIV4HTTP("1.1.1.1", 2*time.Second, "cn", false)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent lookup error = %v", err)
		}
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("factory calls = %d, want 1 cached client under concurrency", got)
	}
}

func TestNextTraceAPIV4ClientLookupBuildsRequestAndParsesResponse(t *testing.T) {
	expires := "2026-05-22T12:00:00Z"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v4/ipGeo" {
			t.Fatalf("path = %q, want /v4/ipGeo", r.URL.Path)
		}
		if got := r.URL.Query().Get("ip"); got != "2001:db8::1" {
			t.Fatalf("query ip = %q, want escaped IPv6 value", got)
		}
		if got := r.Header.Get(nextTraceAPIV4TokenHeader); got != "test-token" {
			t.Fatalf("%s = %q, want test-token", nextTraceAPIV4TokenHeader, got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("request body = %q, want empty", string(body))
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-NextTrace-Quota-Remaining", "999")
		w.Header().Set("X-NextTrace-Quota-Expires-At", expires)
		w.Header().Set("X-NextTrace-Quota-Cost", "6")
		w.Header().Set("X-NextTrace-Quota-Source", "db_hit")
		_, _ = w.Write([]byte(`{
			"ip":"2001:db8::1",
			"asnumber":"64512",
			"country":"中国",
			"country_en":"China",
			"prov":"上海",
			"prov_en":"Shanghai",
			"city":"上海",
			"city_en":"Shanghai",
			"district":"浦东",
			"owner":"Example Owner",
			"domain":"example.net",
			"isp":"Example ISP",
			"whois":"example whois",
			"lat":31.2304,
			"lng":121.4737,
			"prefix":"2001:db8::/32",
			"router":{"64512":["64513","64514"]}
		}`))
	}))
	defer srv.Close()

	client := NewNextTraceAPIV4Client(srv.URL+"/v4/ipGeo", "test-token", srv.Client())
	geo, quota, err := client.Lookup(context.Background(), "2001:db8::1")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if geo.IP != "2001:db8::1" || geo.Asnumber != "64512" || geo.Country != "中国" || geo.CountryEn != "China" {
		t.Fatalf("geo fields not decoded: %+v", geo)
	}
	if geo.Domain != "example.net" || geo.Isp != "Example ISP" || geo.Whois != "example whois" || geo.Prefix != "2001:db8::/32" {
		t.Fatalf("geo extended fields not decoded: %+v", geo)
	}
	if got := geo.Router["64512"]; !reflect.DeepEqual(got, []string{"64513", "64514"}) {
		t.Fatalf("router = %#v, want two ASNs", geo.Router)
	}
	if !quota.HasRemaining || quota.Remaining != 999 || !quota.HasCost || quota.Cost != 6 || quota.Source != "db_hit" {
		t.Fatalf("quota headers not parsed: %+v", quota)
	}
	if !quota.HasExpiresAt || quota.ExpiresAt.Format(time.RFC3339) != expires {
		t.Fatalf("quota expires = %+v, want %s", quota, expires)
	}
}

func TestNewNextTraceAPIV4ClientDefaultsBoundedHTTPClient(t *testing.T) {
	client := NewNextTraceAPIV4Client("", " test-token ", nil)
	if client.httpClient == nil {
		t.Fatal("httpClient = nil, want bounded default client")
	}
	if client.httpClient.Timeout < nextTraceAPIV4MinTimeout {
		t.Fatalf("httpClient.Timeout = %s, want at least %s", client.httpClient.Timeout, nextTraceAPIV4MinTimeout)
	}
	if client.httpClient == http.DefaultClient {
		t.Fatal("httpClient = http.DefaultClient, want bounded default client")
	}
	if client.endpoint != nextTraceAPIV4GeoEndpoint {
		t.Fatalf("endpoint = %q, want default endpoint", client.endpoint)
	}
	if client.token != "test-token" {
		t.Fatalf("token = %q, want trimmed token", client.token)
	}
}

func TestNewNextTraceAPIV4ClientTrimsEndpoint(t *testing.T) {
	endpoint := "https://api.example.test/v4/ipGeo"
	client := NewNextTraceAPIV4Client(" "+endpoint+" ", "test-token", http.DefaultClient)
	if client.endpoint != endpoint {
		t.Fatalf("endpoint = %q, want %q", client.endpoint, endpoint)
	}
}

func TestNextTraceAPIV4ClientLookupRejectsOversizedSuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"1.1.1.1","pad":"` + strings.Repeat("x", nextTraceAPIV4MaxGeoBody) + `"}`))
	}))
	defer srv.Close()

	client := NewNextTraceAPIV4Client(srv.URL, "secret-token", srv.Client())
	_, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err == nil {
		t.Fatal("Lookup() error = nil, want oversized response error")
	}
	if !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("error = %q, want body limit message", err.Error())
	}
}

func TestDecodeNextTraceAPIV4GeoAllowsMissingOrEmptyRouter(t *testing.T) {
	for _, body := range []string{
		`{"ip":"1.1.1.1","router":null}`,
		`{"ip":"1.1.1.1","router":""}`,
		`{"ip":"1.1.1.1"}`,
		`{"ip":"1.1.1.1","router":{}}`,
	} {
		geo, err := decodeNextTraceAPIV4Geo([]byte(body))
		if err != nil {
			t.Fatalf("decodeNextTraceAPIV4Geo(%s) error = %v", body, err)
		}
		if geo.IP != "1.1.1.1" {
			t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
		}
	}
}

func TestNextTraceAPIV4ClientLookupRetriesNetworkErrors(t *testing.T) {
	withNextTraceAPIV4RetryDelays(t, 0, 0)
	attempts := 0
	client := NewNextTraceAPIV4Client("https://api.nxtrace.org/v4/ipGeo", "secret-token", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("network down")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ip":"1.1.1.1"}`)),
				Request:    req,
			}, nil
		}),
	})

	geo, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if geo.IP != "1.1.1.1" {
		t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
	}
}

func TestNextTraceAPIV4ClientLookupRetriesTransientHTTPStatuses(t *testing.T) {
	for _, statusCode := range []int{
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			withNextTraceAPIV4RetryDelays(t, 0, 0)
			attempts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts < 3 {
					w.WriteHeader(statusCode)
					_, _ = w.Write([]byte(`{"error":{"message":"temporary failure"}}`))
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_, _ = w.Write([]byte(`{"ip":"1.1.1.1"}`))
			}))
			defer srv.Close()

			client := NewNextTraceAPIV4Client(srv.URL, "secret-token", srv.Client())
			geo, _, err := client.Lookup(context.Background(), "1.1.1.1")
			if err != nil {
				t.Fatalf("Lookup() error = %v", err)
			}
			if attempts != 3 {
				t.Fatalf("attempts = %d, want 3", attempts)
			}
			if geo.IP != "1.1.1.1" {
				t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
			}
		})
	}
}

func TestNextTraceAPIV4ClientLookupDoesNotRetryNonTransientHTTPStatuses(t *testing.T) {
	for _, statusCode := range []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusTooManyRequests,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			attempts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.WriteHeader(statusCode)
				_, _ = w.Write([]byte(`{"error":{"message":"non transient"}}`))
			}))
			defer srv.Close()

			client := NewNextTraceAPIV4Client(srv.URL, "secret-token", srv.Client())
			_, _, err := client.Lookup(context.Background(), "1.1.1.1")
			if err == nil {
				t.Fatal("Lookup() error = nil, want error")
			}
			if attempts != 1 {
				t.Fatalf("attempts = %d, want 1", attempts)
			}
			if !strings.Contains(err.Error(), "non transient") {
				t.Fatalf("error = %q, want non transient message", err.Error())
			}
		})
	}
}

func TestNextTraceAPIV4ClientLookupReturnsLastRetryErrorRedacted(t *testing.T) {
	withNextTraceAPIV4RetryDelays(t, 0, 0)
	attempts := 0
	client := NewNextTraceAPIV4Client("https://api.nxtrace.org/v4/ipGeo", "secret-token", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return nil, errors.New("network down attempt " + strconv.Itoa(attempts) + " secret-token")
		}),
	})
	_, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err == nil {
		t.Fatal("Lookup() error = nil, want error")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if !strings.Contains(err.Error(), "attempt 3") {
		t.Fatalf("error = %q, want final attempt error", err.Error())
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked token: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error = %q, want redaction marker", err.Error())
	}
}

func TestNextTraceAPIV4ClientLookupHTTPErrorMessages(t *testing.T) {
	withNextTraceAPIV4RetryDelays(t, 0, 0)
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "bad request", statusCode: http.StatusBadRequest, body: `{"error":{"message":"IPAddr cannot be empty"}}`, want: "HTTP 400 Bad Request: IPAddr cannot be empty"},
		{name: "unauthorized", statusCode: http.StatusUnauthorized, body: `{"error":{"message":"unauthorized"}}`, want: "HTTP 401 Unauthorized: unauthorized"},
		{name: "quota", statusCode: http.StatusTooManyRequests, body: `{"error":{"message":"quota exhausted"}}`, want: "HTTP 429 Too Many Requests: quota exhausted"},
		{name: "server error", statusCode: http.StatusInternalServerError, body: `{"error":{"message":"internal server error"}}`, want: "HTTP 500 Internal Server Error: internal server error"},
		{name: "non json", statusCode: http.StatusBadGateway, body: `upstream failed`, want: "HTTP 502 Bad Gateway: upstream failed"},
		{name: "missing error message", statusCode: http.StatusBadGateway, body: `{"error":{}}`, want: `HTTP 502 Bad Gateway: {"error":{}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewNextTraceAPIV4Client(srv.URL, "secret-token", srv.Client())
			_, _, err := client.Lookup(context.Background(), "1.1.1.1")
			if err == nil {
				t.Fatal("Lookup() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNextTraceAPIV4ClientLookupTruncatesLargeErrorBodyAndRedactsToken(t *testing.T) {
	withNextTraceAPIV4RetryDelays(t, 0, 0)
	body := strings.Repeat("a", 100) + " secret-token " + strings.Repeat("b", nextTraceAPIV4MaxErrorBody)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	client := NewNextTraceAPIV4Client(srv.URL, "secret-token", srv.Client())
	_, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err == nil {
		t.Fatal("Lookup() error = nil, want error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked token: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error = %q, want redaction marker", err.Error())
	}
	if !strings.Contains(err.Error(), "truncated at 512 bytes") {
		t.Fatalf("error = %q, want truncated marker", err.Error())
	}
	if strings.Contains(err.Error(), strings.Repeat("b", nextTraceAPIV4MaxErrorBody)) {
		t.Fatalf("error included unbounded body: %q", err.Error())
	}
}

func TestNextTraceAPIV4ClientLookupRedactsTokenFromErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad token secret-token"}}`))
	}))
	defer srv.Close()

	client := NewNextTraceAPIV4Client(srv.URL, "secret-token", srv.Client())
	_, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err == nil {
		t.Fatal("Lookup() error = nil, want error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked token: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error = %q, want redaction marker", err.Error())
	}
}

func TestNextTraceAPIV4ClientLookupNetworkErrorDoesNotFallback(t *testing.T) {
	withNextTraceAPIV4RetryDelays(t, 0, 0)
	attempts := 0
	client := NewNextTraceAPIV4Client("https://api.nxtrace.org/v4/ipGeo", "secret-token", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return nil, errors.New("network down secret-token")
		}),
	})
	_, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err == nil {
		t.Fatal("Lookup() error = nil, want network error")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if !strings.Contains(err.Error(), "network down") {
		t.Fatalf("error = %q, want network down", err.Error())
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked token: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error = %q, want redaction marker", err.Error())
	}
}
