package ipgeo

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
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

func withNextTraceAPIV4RetryDelays(t *testing.T, delays ...time.Duration) {
	t.Helper()
	old := nextTraceAPIV4RetryDelays
	nextTraceAPIV4RetryDelays = delays
	t.Cleanup(func() {
		nextTraceAPIV4RetryDelays = old
	})
}

func resetNextTraceAPIV4TransportCache() {
	nextTraceAPIV4TransportCacheMu.Lock()
	transports := make([]*http.Transport, 0, len(nextTraceAPIV4TransportCache))
	for _, transport := range nextTraceAPIV4TransportCache {
		transports = append(transports, transport)
	}
	nextTraceAPIV4TransportCache = make(map[nextTraceAPIV4TransportCacheKey]*http.Transport)
	nextTraceAPIV4TransportCacheOrder = nil
	nextTraceAPIV4TransportCacheMu.Unlock()
	closeNextTraceAPIV4TransportIdleConnections(transports)
}

func nextTraceAPIV4TransportCacheLen() int {
	nextTraceAPIV4TransportCacheMu.RLock()
	defer nextTraceAPIV4TransportCacheMu.RUnlock()
	return len(nextTraceAPIV4TransportCache)
}

func TestNextTraceAPIV4GeoIPNormalizesTimeout(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		resetNextTraceAPIV4TransportCache()
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"1.1.1.1"}`))
	}))
	defer srv.Close()
	nextTraceAPIV4GeoEndpoint = srv.URL
	resetNextTraceAPIV4TransportCache()

	geo, err := NextTraceAPIV4GeoIP("1.1.1.1", time.Millisecond, "cn", false)
	if err != nil {
		t.Fatalf("NextTraceAPIV4GeoIP() error = %v", err)
	}
	if geo.IP != "1.1.1.1" {
		t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
	}
}

func TestNextTraceAPIV4GeoIPUsesTimeoutAsTotalBudget(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	withNextTraceAPIV4RetryDelays(t, 3*time.Second, 3*time.Second)
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		resetNextTraceAPIV4TransportCache()
	})

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"temporary failure"}}`))
	}))
	defer srv.Close()
	nextTraceAPIV4GeoEndpoint = srv.URL
	resetNextTraceAPIV4TransportCache()

	start := time.Now()
	_, err := NextTraceAPIV4GeoIP("1.1.1.1", 2*time.Second, "cn", false)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("NextTraceAPIV4GeoIP() error = nil, want error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 before timeout budget stops retry", attempts)
	}
	if elapsed >= 2900*time.Millisecond {
		t.Fatalf("elapsed = %s, want bounded by 2s timeout budget", elapsed)
	}
}

func TestNextTraceAPIV4GeoIPSharesTimeoutWithFastIPPrewarm(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFastIPFn := nextTraceAPIV4FastIPFn
	oldProxy := util.EnvProxyURL
	oldCache := util.GetFastIPCache()
	oldMeta := util.GetFastIPMetaCache()
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4FastIPFn = oldFastIPFn
		util.EnvProxyURL = oldProxy
		util.SetFastIPCacheState(oldCache, oldMeta)
		resetNextTraceAPIV4TransportCache()
	})

	util.EnvProxyURL = ""
	util.SetFastIPCacheState("", util.FastIPMeta{})
	requestStarted := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted <- struct{}{}
		<-r.Context().Done()
	}))
	defer srv.Close()
	port := strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)
	nextTraceAPIV4GeoEndpoint = "http://" + nextTraceAPIV4APIHost + ":" + port + nextTraceAPIV4GeoPath
	nextTraceAPIV4FastIPFn = func(ctx context.Context, _ string, _ string, enableOutput bool) (string, error) {
		if enableOutput {
			t.Fatal("FastIP enableOutput = true, want false for lookup fallback")
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("FastIP context has no deadline")
		}
		timer := time.NewTimer(750 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		util.SetFastIPCacheState("127.0.0.1", util.FastIPMeta{IP: "127.0.0.1"})
		return "127.0.0.1", nil
	}

	timeout := 2 * time.Second
	resetNextTraceAPIV4TransportCache()

	start := time.Now()
	if _, err := NextTraceAPIV4GeoIP("1.1.1.1", timeout, "cn", false); err == nil {
		t.Fatal("NextTraceAPIV4GeoIP() error = nil, want deadline error")
	}
	elapsed := time.Since(start)
	if elapsed < 1700*time.Millisecond || elapsed > 2700*time.Millisecond {
		t.Fatalf("elapsed = %s, want one shared %s budget including FastIP", elapsed, timeout)
	}
	select {
	case <-requestStarted:
	default:
		t.Fatal("lookup did not start after FastIP prewarm")
	}
}

func TestNextTraceAPIV4GeoIPReusesTransportAndConnection(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		resetNextTraceAPIV4TransportCache()
	})

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
	resetNextTraceAPIV4TransportCache()

	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"} {
		geo, err := NextTraceAPIV4GeoIP(ip, 2*time.Second, "cn", false)
		if err != nil {
			t.Fatalf("NextTraceAPIV4GeoIP(%s) error = %v", ip, err)
		}
		if geo.IP != ip {
			t.Fatalf("IP = %q, want %s", geo.IP, ip)
		}
	}

	if got := nextTraceAPIV4TransportCacheLen(); got != 1 {
		t.Fatalf("transport cache size = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&newConns); got != 1 {
		t.Fatalf("new connections = %d, want 1 reused HTTP connection", got)
	}
}

func TestNextTraceAPIV4GeoIPUsesFastIPForAPIEndpoint(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFastIPFn := nextTraceAPIV4FastIPFn
	oldProxy := util.EnvProxyURL
	oldCache := util.GetFastIPCache()
	oldMeta := util.GetFastIPMetaCache()
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4FastIPFn = oldFastIPFn
		util.EnvProxyURL = oldProxy
		util.SetFastIPCacheState(oldCache, oldMeta)
		resetNextTraceAPIV4TransportCache()
	})

	util.EnvProxyURL = ""
	util.SetFastIPCacheState("", util.FastIPMeta{})
	var fastIPCalls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Host; !strings.HasPrefix(got, nextTraceAPIV4APIHost+":") {
			t.Fatalf("Host = %q, want api host with port", got)
		}
		if got := r.URL.Path; got != nextTraceAPIV4GeoPath {
			t.Fatalf("path = %q, want %s", got, nextTraceAPIV4GeoPath)
		}
		if got := r.URL.Query().Get("ip"); got != "1.1.1.1" {
			t.Fatalf("query ip = %q, want 1.1.1.1", got)
		}
		if got := r.Header.Get(nextTraceAPIV4TokenHeader); got != "test-token" {
			t.Fatalf("%s = %q, want test-token", nextTraceAPIV4TokenHeader, got)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"1.1.1.1"}`))
	}))
	defer srv.Close()
	port := strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)
	nextTraceAPIV4GeoEndpoint = "http://" + nextTraceAPIV4APIHost + ":" + port + nextTraceAPIV4GeoPath
	nextTraceAPIV4FastIPFn = func(ctx context.Context, domain string, gotPort string, enableOutput bool) (string, error) {
		atomic.AddInt32(&fastIPCalls, 1)
		if domain != nextTraceAPIV4APIHost {
			t.Fatalf("FastIP domain = %q, want %s", domain, nextTraceAPIV4APIHost)
		}
		if gotPort != port {
			t.Fatalf("FastIP port = %q, want %s", gotPort, port)
		}
		if enableOutput {
			t.Fatal("FastIP enableOutput = true, want false for lookup fallback")
		}
		util.SetFastIPCacheState("127.0.0.1", util.FastIPMeta{IP: "127.0.0.1"})
		return "127.0.0.1", nil
	}
	resetNextTraceAPIV4TransportCache()

	geo, err := NextTraceAPIV4GeoIP("1.1.1.1", 2*time.Second, "cn", false)
	if err != nil {
		t.Fatalf("NextTraceAPIV4GeoIP() error = %v", err)
	}
	if geo.IP != "1.1.1.1" {
		t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
	}
	if got := atomic.LoadInt32(&fastIPCalls); got != 1 {
		t.Fatalf("FastIP calls = %d, want 1", got)
	}
}

func TestNextTraceAPIV4FastIPDialPreservesTLSServerName(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldProxy := util.EnvProxyURL
	oldCache := util.GetFastIPCache()
	oldMeta := util.GetFastIPMetaCache()
	t.Cleanup(func() {
		util.EnvProxyURL = oldProxy
		util.SetFastIPCacheState(oldCache, oldMeta)
	})
	util.EnvProxyURL = ""

	sniCh := make(chan string, 1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sniCh <- r.TLS.ServerName
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"1.1.1.1"}`))
	}))
	defer srv.Close()
	listenerHost, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	util.SetFastIPCacheState(listenerHost, util.FastIPMeta{IP: listenerHost})

	endpoint := "https://" + net.JoinHostPort(nextTraceAPIV4APIHost, port) + nextTraceAPIV4GeoPath
	policy := util.NewGeoDNSPolicy("", true)
	transport := newNextTraceAPIV4Transport(endpoint, policy, nextTraceAPIV4ProxyPolicy{identity: "direct"})
	tlsTransport, ok := srv.Client().Transport.(*http.Transport)
	if !ok || tlsTransport.TLSClientConfig == nil {
		t.Fatal("test server client has no TLS config")
	}
	transport.TLSClientConfig = tlsTransport.TLSClientConfig.Clone()
	transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // The local test verifies SNI, not certificate trust.
	defer transport.CloseIdleConnections()

	client := NewNextTraceAPIV4Client(endpoint, "test-token", &http.Client{Timeout: 2 * time.Second, Transport: transport})
	geo, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if geo.IP != "1.1.1.1" {
		t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
	}
	select {
	case got := <-sniCh:
		if got != nextTraceAPIV4APIHost {
			t.Fatalf("TLS ServerName = %q, want %q", got, nextTraceAPIV4APIHost)
		}
	case <-time.After(time.Second):
		t.Fatal("TLS server did not observe a handshake")
	}
}

func TestPrepareNextTraceAPIV4FastIPUsesBoundedChildContext(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldFastIPFn := nextTraceAPIV4FastIPFn
	oldProxy := util.EnvProxyURL
	t.Cleanup(func() {
		nextTraceAPIV4FastIPFn = oldFastIPFn
		util.EnvProxyURL = oldProxy
	})

	util.EnvProxyURL = ""
	var calls int32
	nextTraceAPIV4FastIPFn = func(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
		atomic.AddInt32(&calls, 1)
		if domain != nextTraceAPIV4APIHost {
			t.Fatalf("FastIP domain = %q, want %s", domain, nextTraceAPIV4APIHost)
		}
		if port != nextTraceAPIV4DefaultPort {
			t.Fatalf("FastIP port = %q, want %s", port, nextTraceAPIV4DefaultPort)
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("FastIP context has no deadline")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > nextTraceAPIV4FastIPTimeout+200*time.Millisecond {
			t.Fatalf("FastIP context remaining = %s, want bounded by %s", remaining, nextTraceAPIV4FastIPTimeout)
		}
		return "127.0.0.1", nil
	}

	if err := prepareNextTraceAPIV4FastIP(context.Background(), nextTraceAPIV4GeoEndpoint, false); err != nil {
		t.Fatalf("prepareNextTraceAPIV4FastIP() error = %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("FastIP calls = %d, want 1", got)
	}
}

func TestNextTraceAPIV4APIEndpointHostPortDefaultsHTTPPort(t *testing.T) {
	_, port, ok := nextTraceAPIV4APIEndpointHostPort("http://" + nextTraceAPIV4APIHost + nextTraceAPIV4GeoPath)
	if !ok {
		t.Fatal("http endpoint should be recognized")
	}
	if port != "80" {
		t.Fatalf("http implicit port = %q, want 80", port)
	}

	_, port, ok = nextTraceAPIV4APIEndpointHostPort("https://" + nextTraceAPIV4APIHost + nextTraceAPIV4GeoPath)
	if !ok {
		t.Fatal("https endpoint should be recognized")
	}
	if port != nextTraceAPIV4DefaultPort {
		t.Fatalf("https implicit port = %q, want %s", port, nextTraceAPIV4DefaultPort)
	}
}

func TestDialNextTraceAPIV4FallsBackWhenCachedFastIPFails(t *testing.T) {
	oldCache := util.GetFastIPCache()
	oldMeta := util.GetFastIPMetaCache()
	t.Cleanup(func() {
		util.SetFastIPCacheState(oldCache, oldMeta)
	})

	listenConfig := net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Fatalf("listener.Close() error = %v", err)
		}
	}()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	util.SetFastIPCacheState("127.0.0.2", util.FastIPMeta{IP: "127.0.0.2"})
	dialer := &net.Dialer{Timeout: time.Second}
	originalAddr := net.JoinHostPort("127.0.0.1", port)
	var fallbackAddr string
	fallbackDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		fallbackAddr = addr
		return dialer.DialContext(ctx, network, addr)
	}
	conn, err := dialNextTraceAPIV4(context.Background(), dialer, fallbackDial, "tcp", originalAddr, "127.0.0.1", port)
	if err != nil {
		t.Fatalf("dialNextTraceAPIV4() error = %v", err)
	}
	_ = conn.Close()
	if fallbackAddr != originalAddr {
		t.Fatalf("fallback addr = %q, want captured policy dialer addr %q", fallbackAddr, originalAddr)
	}

	select {
	case serverConn := <-accepted:
		_ = serverConn.Close()
	case <-time.After(time.Second):
		t.Fatal("fallback dial did not reach original host listener")
	}
}

func TestDialNextTraceAPIV4CancellationReachesFallback(t *testing.T) {
	oldCache := util.GetFastIPCache()
	oldMeta := util.GetFastIPMetaCache()
	t.Cleanup(func() {
		util.SetFastIPCacheState(oldCache, oldMeta)
	})

	util.SetFastIPCacheState("192.0.2.1", util.FastIPMeta{IP: "192.0.2.1"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var fallbackCalls int32
	fallbackDial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		atomic.AddInt32(&fallbackCalls, 1)
		return nil, ctx.Err()
	}
	_, err := dialNextTraceAPIV4(ctx, &net.Dialer{}, fallbackDial, "tcp", "api.nxtrace.org:443", nextTraceAPIV4APIHost, nextTraceAPIV4DefaultPort)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("dialNextTraceAPIV4() error = %v, want context canceled", err)
	}
	if got := atomic.LoadInt32(&fallbackCalls); got != 1 {
		t.Fatalf("fallback calls = %d, want 1", got)
	}
}

func TestPrepareNextTraceAPIV4FastIPSkipsStandardProxyEnv(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldFastIPFn := nextTraceAPIV4FastIPFn
	oldProxy := util.EnvProxyURL
	t.Cleanup(func() {
		nextTraceAPIV4FastIPFn = oldFastIPFn
		util.EnvProxyURL = oldProxy
	})

	util.EnvProxyURL = ""
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:65535")
	var calls int32
	nextTraceAPIV4FastIPFn = func(context.Context, string, string, bool) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "127.0.0.1", nil
	}

	if err := prepareNextTraceAPIV4FastIP(context.Background(), nextTraceAPIV4GeoEndpoint, false); err != nil {
		t.Fatalf("prepareNextTraceAPIV4FastIP() error = %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("FastIP calls = %d, want 0 with HTTPS_PROXY", got)
	}
}

func TestNewNextTraceAPIV4TransportSkipsFastIPDialForStandardProxyEnv(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldProxy := util.EnvProxyURL
	t.Cleanup(func() {
		util.EnvProxyURL = oldProxy
	})

	util.EnvProxyURL = ""
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:65535")
	policy := util.CurrentGeoDNSPolicy()
	proxy := captureNextTraceAPIV4Proxy(nextTraceAPIV4GeoEndpoint)
	transport := newNextTraceAPIV4Transport(nextTraceAPIV4GeoEndpoint, policy, proxy)
	if transport == nil || transport.DialContext == nil {
		t.Fatalf("transport = %#v, want http.Transport with DialContext", transport)
	}
	dialName := runtime.FuncForPC(reflect.ValueOf(transport.DialContext).Pointer()).Name()
	if strings.Contains(dialName, "newNextTraceAPIV4Transport") {
		t.Fatalf("DialContext = %s, want util.NewGeoHTTPClient dialer when HTTPS_PROXY is set", dialName)
	}
}

func TestNextTraceAPIV4EnvironmentProxyReevaluatesRedirectURL(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:18080")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:18443")
	t.Setenv("NO_PROXY", "origin.example,bypass.example")

	policy := captureNextTraceAPIV4Proxy("http://origin.example/v4/ipGeo")
	tests := []struct {
		name       string
		requestURL string
		wantProxy  string
	}{
		{name: "initial host bypasses proxy", requestURL: "http://origin.example/v4/ipGeo"},
		{name: "https redirect uses https proxy", requestURL: "https://secure.example/v4/ipGeo", wantProxy: "http://127.0.0.1:18443"},
		{name: "http redirect uses http proxy", requestURL: "http://plain.example/v4/ipGeo", wantProxy: "http://127.0.0.1:18080"},
		{name: "redirected bypass host stays direct", requestURL: "https://bypass.example/v4/ipGeo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.requestURL, nil)
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			proxyURL, err := policy.proxy(req)
			if err != nil {
				t.Fatalf("proxy() error = %v", err)
			}
			got := ""
			if proxyURL != nil {
				got = proxyURL.String()
			}
			if got != tt.wantProxy {
				t.Fatalf("proxy for %s = %q, want %q", tt.requestURL, got, tt.wantProxy)
			}
		})
	}
}

func TestNextTraceAPIV4ExplicitProxyRemainsFixedAcrossRedirects(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	const explicitProxy = "http://127.0.0.1:19090"
	util.EnvProxyURL = explicitProxy

	policy := captureNextTraceAPIV4Proxy("http://origin.example/v4/ipGeo")
	for _, requestURL := range []string{
		"http://origin.example/v4/ipGeo",
		"https://redirected.example/v4/ipGeo",
	} {
		req, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}
		proxyURL, err := policy.proxy(req)
		if err != nil {
			t.Fatalf("proxy() error = %v", err)
		}
		if proxyURL == nil || proxyURL.String() != explicitProxy {
			t.Fatalf("proxy for %s = %v, want %s", requestURL, proxyURL, explicitProxy)
		}
	}
}

func TestPrepareNextTraceAPIV4FastIPEvaluatesOnlyInitialEndpoint(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:18443")
	t.Setenv("NO_PROXY", nextTraceAPIV4APIHost)
	oldFastIPFn := nextTraceAPIV4FastIPFn
	t.Cleanup(func() {
		nextTraceAPIV4FastIPFn = oldFastIPFn
	})

	var calls int32
	nextTraceAPIV4FastIPFn = func(context.Context, string, string, bool) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "127.0.0.1", nil
	}
	policy := captureNextTraceAPIV4Proxy(nextTraceAPIV4GeoEndpoint)
	if err := prepareNextTraceAPIV4FastIPWithProxy(context.Background(), nextTraceAPIV4GeoEndpoint, false, policy); err != nil {
		t.Fatalf("prepareNextTraceAPIV4FastIPWithProxy() error = %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("FastIP calls = %d, want 1 for initial NO_PROXY endpoint", got)
	}

	redirectReq, err := http.NewRequest(http.MethodGet, "https://redirected.example/v4/ipGeo", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	proxyURL, err := policy.proxy(redirectReq)
	if err != nil {
		t.Fatalf("proxy() error = %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:18443" {
		t.Fatalf("redirect proxy = %v, want captured HTTPS_PROXY", proxyURL)
	}
}

func TestNextTraceAPIV4HTTPSProxyPreservesCONNECTHostAndTLSServerName(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)

	sniCh := make(chan string, 1)
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sniCh <- r.TLS.ServerName:
		default:
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"1.1.1.1"}`))
	}))
	defer target.Close()

	connectHostCh := make(chan string, 1)
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		select {
		case connectHostCh <- r.Host:
		default:
		}
		upstream, err := net.DialTimeout("tcp", target.Listener.Addr().String(), time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			_ = upstream.Close()
			http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
			return
		}
		downstream, rw, err := hijacker.Hijack()
		if err != nil {
			_ = upstream.Close()
			return
		}
		if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			_ = downstream.Close()
			_ = upstream.Close()
			return
		}
		if err := rw.Flush(); err != nil {
			_ = downstream.Close()
			_ = upstream.Close()
			return
		}
		go func() {
			_, _ = io.Copy(upstream, downstream)
			_ = upstream.Close()
		}()
		go func() {
			_, _ = io.Copy(downstream, upstream)
			_ = downstream.Close()
		}()
	}))
	defer proxyServer.Close()
	t.Setenv("HTTPS_PROXY", proxyServer.URL)

	_, port, err := net.SplitHostPort(target.Listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	endpoint := "https://" + net.JoinHostPort(nextTraceAPIV4APIHost, port) + nextTraceAPIV4GeoPath
	policy := util.NewGeoDNSPolicy("", true)
	transport := newNextTraceAPIV4Transport(endpoint, policy, captureNextTraceAPIV4Proxy(endpoint))
	tlsTransport, ok := target.Client().Transport.(*http.Transport)
	if !ok || tlsTransport.TLSClientConfig == nil {
		t.Fatal("test server client has no TLS config")
	}
	transport.TLSClientConfig = tlsTransport.TLSClientConfig.Clone()
	transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // The local test verifies CONNECT and SNI, not certificate trust.
	defer transport.CloseIdleConnections()

	client := NewNextTraceAPIV4Client(endpoint, "test-token", &http.Client{Timeout: 2 * time.Second, Transport: transport})
	geo, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if geo.IP != "1.1.1.1" {
		t.Fatalf("IP = %q, want 1.1.1.1", geo.IP)
	}
	wantConnectHost := net.JoinHostPort(nextTraceAPIV4APIHost, port)
	select {
	case got := <-connectHostCh:
		if got != wantConnectHost {
			t.Fatalf("CONNECT host = %q, want %q", got, wantConnectHost)
		}
	case <-time.After(time.Second):
		t.Fatal("proxy did not observe CONNECT")
	}
	select {
	case got := <-sniCh:
		if got != nextTraceAPIV4APIHost {
			t.Fatalf("TLS ServerName = %q, want %q", got, nextTraceAPIV4APIHost)
		}
	case <-time.After(time.Second):
		t.Fatal("TLS server did not observe a handshake")
	}
}

func TestNextTraceAPIV4GeoIPSkipsFastIPForCustomEndpoint(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFastIPFn := nextTraceAPIV4FastIPFn
	oldCache := util.GetFastIPCache()
	oldMeta := util.GetFastIPMetaCache()
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4FastIPFn = oldFastIPFn
		util.SetFastIPCacheState(oldCache, oldMeta)
		resetNextTraceAPIV4TransportCache()
	})

	var fastIPCalls int32
	nextTraceAPIV4FastIPFn = func(context.Context, string, string, bool) (string, error) {
		atomic.AddInt32(&fastIPCalls, 1)
		return "127.0.0.1", nil
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"` + r.URL.Query().Get("ip") + `"}`))
	}))
	defer srv.Close()
	nextTraceAPIV4GeoEndpoint = srv.URL
	resetNextTraceAPIV4TransportCache()

	geo, err := NextTraceAPIV4GeoIP("8.8.8.8", 2*time.Second, "cn", false)
	if err != nil {
		t.Fatalf("NextTraceAPIV4GeoIP() error = %v", err)
	}
	if geo.IP != "8.8.8.8" {
		t.Fatalf("IP = %q, want 8.8.8.8", geo.IP)
	}
	if got := atomic.LoadInt32(&fastIPCalls); got != 0 {
		t.Fatalf("FastIP calls = %d, want 0 for custom endpoint", got)
	}
}

func TestNextTraceAPIV4GeoIPUsesProxyInsteadOfFastIP(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFastIPFn := nextTraceAPIV4FastIPFn
	oldProxy := util.EnvProxyURL
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4FastIPFn = oldFastIPFn
		util.EnvProxyURL = oldProxy
		resetNextTraceAPIV4TransportCache()
	})

	var proxyRequests int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&proxyRequests, 1)
		if got := r.URL.Scheme; got != "http" {
			t.Fatalf("proxy request scheme = %q, want http", got)
		}
		if got := r.URL.Host; got != nextTraceAPIV4APIHost+":65535" {
			t.Fatalf("proxy request host = %q, want api.nxtrace.org:65535", got)
		}
		if got := r.Header.Get(nextTraceAPIV4TokenHeader); got != "test-token" {
			t.Fatalf("%s = %q, want test-token", nextTraceAPIV4TokenHeader, got)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"` + r.URL.Query().Get("ip") + `"}`))
	}))
	defer proxy.Close()

	var fastIPCalls int32
	nextTraceAPIV4FastIPFn = func(context.Context, string, string, bool) (string, error) {
		atomic.AddInt32(&fastIPCalls, 1)
		return "127.0.0.1", nil
	}
	util.EnvProxyURL = proxy.URL
	nextTraceAPIV4GeoEndpoint = "http://" + nextTraceAPIV4APIHost + ":65535" + nextTraceAPIV4GeoPath
	resetNextTraceAPIV4TransportCache()

	geo, err := NextTraceAPIV4GeoIP("9.9.9.9", 2*time.Second, "cn", false)
	if err != nil {
		t.Fatalf("NextTraceAPIV4GeoIP() error = %v", err)
	}
	if geo.IP != "9.9.9.9" {
		t.Fatalf("IP = %q, want 9.9.9.9", geo.IP)
	}
	if got := atomic.LoadInt32(&proxyRequests); got != 1 {
		t.Fatalf("proxy requests = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&fastIPCalls); got != 0 {
		t.Fatalf("FastIP calls = %d, want 0 when proxy is configured", got)
	}
}

func TestNextTraceAPIV4ClientsShareTransportAcrossTokenAndTimeout(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldFactory := nextTraceAPIV4TransportFactory
	t.Cleanup(func() {
		nextTraceAPIV4TransportFactory = oldFactory
		resetNextTraceAPIV4TransportCache()
	})

	var factoryCalls int32
	nextTraceAPIV4TransportFactory = func(string, util.GeoDNSPolicy, nextTraceAPIV4ProxyPolicy) *http.Transport {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Transport{}
	}
	resetNextTraceAPIV4TransportCache()

	endpoint := "https://api.example.test/v4/ipGeo"
	policy := util.CurrentGeoDNSPolicy()
	proxy := captureNextTraceAPIV4Proxy(endpoint)
	first := newNextTraceAPIV4ClientForPolicy(endpoint, "token-a", 2*time.Second, policy, proxy)
	second := newNextTraceAPIV4ClientForPolicy(endpoint, "token-b", 3*time.Second, policy, proxy)
	if first == second {
		t.Fatal("clients were reused, want one lightweight client per construction")
	}
	if first.httpClient.Transport != second.httpClient.Transport {
		t.Fatal("transport differs across token and timeout")
	}
	if first.token != "token-a" || second.token != "token-b" {
		t.Fatalf("tokens = %q, %q, want per-client values", first.token, second.token)
	}
	if first.httpClient.Timeout != 2*time.Second || second.httpClient.Timeout != 3*time.Second {
		t.Fatalf("timeouts = %s, %s, want per-client values", first.httpClient.Timeout, second.httpClient.Timeout)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("transport factory calls = %d, want 1", got)
	}
}

func TestNextTraceAPIV4TransportKeyIncludesFullGeoDNSPolicy(t *testing.T) {
	oldFactory := nextTraceAPIV4TransportFactory
	t.Cleanup(func() {
		nextTraceAPIV4TransportFactory = oldFactory
		resetNextTraceAPIV4TransportCache()
	})

	var factoryCalls int32
	nextTraceAPIV4TransportFactory = func(string, util.GeoDNSPolicy, nextTraceAPIV4ProxyPolicy) *http.Transport {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Transport{}
	}
	resetNextTraceAPIV4TransportCache()

	endpoint := "https://api.example.test/v4/ipGeo"
	proxy := nextTraceAPIV4ProxyPolicy{identity: "direct"}
	googleFallback := util.NewGeoDNSPolicy(" GOOGLE ", true)
	googleStrict := util.NewGeoDNSPolicy("google", false)
	cloudflareFallback := util.NewGeoDNSPolicy("cloudflare", true)

	first := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, googleFallback, proxy)
	strict := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, googleStrict, proxy)
	cloudflare := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, cloudflareFallback, proxy)
	reused := newNextTraceAPIV4ClientForPolicy(endpoint, "other-token", 3*time.Second, googleFallback, proxy)

	if first.httpClient.Transport == strict.httpClient.Transport {
		t.Fatal("fallback change reused transport")
	}
	if first.httpClient.Transport == cloudflare.httpClient.Transport {
		t.Fatal("resolver change reused transport")
	}
	if first.httpClient.Transport != reused.httpClient.Transport {
		t.Fatal("equivalent normalized policy did not reuse transport")
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 3 {
		t.Fatalf("transport factory calls = %d, want 3 policies", got)
	}
}

func TestNextTraceAPIV4TransportKeyIncludesProxyIdentity(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldFactory := nextTraceAPIV4TransportFactory
	oldProxy := util.EnvProxyURL
	t.Cleanup(func() {
		nextTraceAPIV4TransportFactory = oldFactory
		util.EnvProxyURL = oldProxy
		resetNextTraceAPIV4TransportCache()
	})

	var factoryCalls int32
	nextTraceAPIV4TransportFactory = func(string, util.GeoDNSPolicy, nextTraceAPIV4ProxyPolicy) *http.Transport {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Transport{}
	}
	resetNextTraceAPIV4TransportCache()

	endpoint := "https://api.example.test/v4/ipGeo"
	policy := util.NewGeoDNSPolicy("", true)
	util.EnvProxyURL = ""
	direct := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, policy, captureNextTraceAPIV4Proxy(endpoint))
	util.EnvProxyURL = "http://127.0.0.1:10001"
	proxyOne := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, policy, captureNextTraceAPIV4Proxy(endpoint))
	util.EnvProxyURL = "http://127.0.0.1:10002"
	proxyTwo := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, policy, captureNextTraceAPIV4Proxy(endpoint))
	util.EnvProxyURL = "http://127.0.0.1:10001"
	proxyOneAgain := newNextTraceAPIV4ClientForPolicy(endpoint, "other-token", 3*time.Second, policy, captureNextTraceAPIV4Proxy(endpoint))

	if direct.httpClient.Transport == proxyOne.httpClient.Transport {
		t.Fatal("direct and proxied requests reused transport")
	}
	if proxyOne.httpClient.Transport == proxyTwo.httpClient.Transport {
		t.Fatal("different proxies reused transport")
	}
	if proxyOne.httpClient.Transport != proxyOneAgain.httpClient.Transport {
		t.Fatal("same proxy identity did not reuse transport")
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 3 {
		t.Fatalf("transport factory calls = %d, want 3 proxy identities", got)
	}
}

func TestNextTraceAPIV4TransportKeyIncludesFullEnvironmentProxyConfig(t *testing.T) {
	isolateNextTraceAPIV4ProxyState(t)
	oldFactory := nextTraceAPIV4TransportFactory
	t.Cleanup(func() {
		nextTraceAPIV4TransportFactory = oldFactory
		resetNextTraceAPIV4TransportCache()
	})

	var factoryCalls int32
	nextTraceAPIV4TransportFactory = func(string, util.GeoDNSPolicy, nextTraceAPIV4ProxyPolicy) *http.Transport {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Transport{}
	}
	resetNextTraceAPIV4TransportCache()

	endpoint := "https://api.example.test/v4/ipGeo"
	policy := util.NewGeoDNSPolicy("", true)
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:18443")
	t.Setenv("NO_PROXY", "api.example.test")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:18080")
	first := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, policy, captureNextTraceAPIV4Proxy(endpoint))
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:18081")
	second := newNextTraceAPIV4ClientForPolicy(endpoint, "token", 2*time.Second, policy, captureNextTraceAPIV4Proxy(endpoint))
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:18080")
	firstAgain := newNextTraceAPIV4ClientForPolicy(endpoint, "other-token", 3*time.Second, policy, captureNextTraceAPIV4Proxy(endpoint))

	if first.httpClient.Transport == second.httpClient.Transport {
		t.Fatal("different environment proxy configurations reused transport")
	}
	if first.httpClient.Transport != firstAgain.httpClient.Transport {
		t.Fatal("same environment proxy configuration did not reuse transport")
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 2 {
		t.Fatalf("transport factory calls = %d, want 2 configurations", got)
	}
}

func TestNewNextTraceAPIV4TransportClonesSharedPolicyTransport(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldProxy := util.EnvProxyURL
	t.Cleanup(func() {
		util.EnvProxyURL = oldProxy
	})
	util.EnvProxyURL = ""

	policy := util.NewGeoDNSPolicy("", true)
	shared, ok := util.NewSharedGeoHTTPClientWithPolicy(0, policy).Transport.(*http.Transport)
	if !ok || shared == nil {
		t.Fatal("shared policy transport is not *http.Transport")
	}
	derived := newNextTraceAPIV4Transport("https://api.example.test/v4/ipGeo", policy, captureNextTraceAPIV4Proxy("https://api.example.test/v4/ipGeo"))
	if derived == shared {
		t.Fatal("v4 transport reused mutable shared policy transport")
	}
	originalMaxIdle := shared.MaxIdleConns
	derived.MaxIdleConns = originalMaxIdle + 1
	if shared.MaxIdleConns != originalMaxIdle {
		t.Fatal("mutating v4 transport changed shared policy transport")
	}
}

func TestNextTraceAPIV4TransportCacheNormalizesEndpoint(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	oldFactory := nextTraceAPIV4TransportFactory
	t.Cleanup(func() {
		nextTraceAPIV4TransportFactory = oldFactory
		resetNextTraceAPIV4TransportCache()
	})

	var factoryCalls int32
	nextTraceAPIV4TransportFactory = func(string, util.GeoDNSPolicy, nextTraceAPIV4ProxyPolicy) *http.Transport {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Transport{}
	}
	resetNextTraceAPIV4TransportCache()

	endpoint := "https://api.example.test/v4/ipGeo"
	policy := util.CurrentGeoDNSPolicy()
	proxy := captureNextTraceAPIV4Proxy(endpoint)
	client := newNextTraceAPIV4ClientForPolicy(" "+endpoint+" ", "test-token", 2*time.Second, policy, proxy)
	cached := newNextTraceAPIV4ClientForPolicy(endpoint, "test-token", 2*time.Second, policy, proxy)

	if client == cached {
		t.Fatal("client was reused, want lightweight client per construction")
	}
	if client.httpClient.Transport != cached.httpClient.Transport {
		t.Fatal("transport differs for endpoint with surrounding spaces")
	}
	if got := atomic.LoadInt32(&factoryCalls); got != 1 {
		t.Fatalf("factory calls = %d, want 1", got)
	}
	if client.endpoint != endpoint {
		t.Fatalf("client endpoint = %q, want %q", client.endpoint, endpoint)
	}
}

func TestNextTraceAPIV4TransportCacheEvictsOldestEntry(t *testing.T) {
	oldFactory := nextTraceAPIV4TransportFactory
	oldCloseIdle := nextTraceAPIV4CloseIdleConnections
	t.Cleanup(func() {
		nextTraceAPIV4TransportFactory = oldFactory
		nextTraceAPIV4CloseIdleConnections = oldCloseIdle
		resetNextTraceAPIV4TransportCache()
	})

	var factoryCalls int32
	var closeIdleCalls int32
	nextTraceAPIV4TransportFactory = func(string, util.GeoDNSPolicy, nextTraceAPIV4ProxyPolicy) *http.Transport {
		atomic.AddInt32(&factoryCalls, 1)
		return &http.Transport{}
	}
	nextTraceAPIV4CloseIdleConnections = func(*http.Transport) {
		atomic.AddInt32(&closeIdleCalls, 1)
	}
	resetNextTraceAPIV4TransportCache()

	policy := util.NewGeoDNSPolicy("", true)
	proxy := nextTraceAPIV4ProxyPolicy{identity: "direct"}
	firstEndpoint := "https://api-0.example.test/v4/ipGeo"
	firstKey := nextTraceAPIV4TransportCacheKey{
		endpoint:      firstEndpoint,
		geoDNSPolicy:  policy,
		proxyIdentity: proxy.identity,
	}
	firstTransport := cachedNextTraceAPIV4Transport(firstEndpoint, policy, proxy)
	for i := 1; i < nextTraceAPIV4CacheMaxSize; i++ {
		endpoint := "https://api-" + strconv.Itoa(i) + ".example.test/v4/ipGeo"
		_ = cachedNextTraceAPIV4Transport(endpoint, policy, proxy)
	}
	if cachedNextTraceAPIV4Transport(firstEndpoint, policy, proxy) != firstTransport {
		t.Fatal("cache hit did not reuse first transport")
	}
	lastEndpoint := "https://api-" + strconv.Itoa(nextTraceAPIV4CacheMaxSize) + ".example.test/v4/ipGeo"
	_ = cachedNextTraceAPIV4Transport(lastEndpoint, policy, proxy)

	if got := nextTraceAPIV4TransportCacheLen(); got != nextTraceAPIV4CacheMaxSize {
		t.Fatalf("cache size = %d, want %d", got, nextTraceAPIV4CacheMaxSize)
	}
	nextTraceAPIV4TransportCacheMu.RLock()
	_, firstStillCached := nextTraceAPIV4TransportCache[firstKey]
	nextTraceAPIV4TransportCacheMu.RUnlock()
	if firstStillCached {
		t.Fatal("oldest cache entry still present after exceeding cache size")
	}
	if got := atomic.LoadInt32(&closeIdleCalls); got != 1 {
		t.Fatalf("CloseIdleConnections calls after first eviction = %d, want 1", got)
	}

	recreated := cachedNextTraceAPIV4Transport(firstEndpoint, policy, proxy)
	if recreated == firstTransport {
		t.Fatal("evicted cache entry reused old transport")
	}
	if got := nextTraceAPIV4TransportCacheLen(); got != nextTraceAPIV4CacheMaxSize {
		t.Fatalf("cache size after recreating evicted entry = %d, want %d", got, nextTraceAPIV4CacheMaxSize)
	}
	if got := atomic.LoadInt32(&factoryCalls); got != int32(nextTraceAPIV4CacheMaxSize+2) {
		t.Fatalf("factory calls = %d, want %d", got, nextTraceAPIV4CacheMaxSize+2)
	}
	if got := atomic.LoadInt32(&closeIdleCalls); got != 2 {
		t.Fatalf("CloseIdleConnections calls after second eviction = %d, want 2", got)
	}
}

func TestNextTraceAPIV4TransportCacheEvictsMultipleOldestEntries(t *testing.T) {
	oldCloseIdle := nextTraceAPIV4CloseIdleConnections
	t.Cleanup(func() {
		nextTraceAPIV4CloseIdleConnections = oldCloseIdle
		resetNextTraceAPIV4TransportCache()
	})
	resetNextTraceAPIV4TransportCache()

	var closeIdleCalls int32
	total := nextTraceAPIV4CacheMaxSize + 3
	keys := make([]nextTraceAPIV4TransportCacheKey, 0, total)
	policy := util.NewGeoDNSPolicy("", true)
	proxy := nextTraceAPIV4ProxyPolicy{identity: "direct"}
	nextTraceAPIV4CloseIdleConnections = func(*http.Transport) {
		atomic.AddInt32(&closeIdleCalls, 1)
	}

	nextTraceAPIV4TransportCacheMu.Lock()
	for i := 0; i < total; i++ {
		key := nextTraceAPIV4TransportCacheKey{
			endpoint:      "https://api-" + strconv.Itoa(i) + ".example.test/v4/ipGeo",
			geoDNSPolicy:  policy,
			proxyIdentity: proxy.identity,
		}
		keys = append(keys, key)
		nextTraceAPIV4TransportCache[key] = &http.Transport{}
		nextTraceAPIV4TransportCacheOrder = append(nextTraceAPIV4TransportCacheOrder, key)
	}
	evicted := evictNextTraceAPIV4TransportCacheLocked()
	gotLen := len(nextTraceAPIV4TransportCache)
	gotOrder := append([]nextTraceAPIV4TransportCacheKey(nil), nextTraceAPIV4TransportCacheOrder...)
	evictedStillCached := false
	for _, key := range keys[:3] {
		if nextTraceAPIV4TransportCache[key] != nil {
			evictedStillCached = true
			break
		}
	}
	nextTraceAPIV4TransportCacheMu.Unlock()
	closeNextTraceAPIV4TransportIdleConnections(evicted)

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

func TestNextTraceAPIV4GeoIPCachesTransportConcurrently(t *testing.T) {
	isolateNextTraceAPIV4ProxyEnv(t)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
	oldEndpoint := nextTraceAPIV4GeoEndpoint
	oldFactory := nextTraceAPIV4TransportFactory
	oldProxy := util.EnvProxyURL
	t.Cleanup(func() {
		nextTraceAPIV4GeoEndpoint = oldEndpoint
		nextTraceAPIV4TransportFactory = oldFactory
		util.EnvProxyURL = oldProxy
		resetNextTraceAPIV4TransportCache()
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ip":"` + r.URL.Query().Get("ip") + `"}`))
	}))
	defer srv.Close()

	var factoryCalls int32
	util.EnvProxyURL = ""
	nextTraceAPIV4GeoEndpoint = srv.URL
	nextTraceAPIV4TransportFactory = func(string, util.GeoDNSPolicy, nextTraceAPIV4ProxyPolicy) *http.Transport {
		atomic.AddInt32(&factoryCalls, 1)
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		return transport
	}
	resetNextTraceAPIV4TransportCache()

	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := NextTraceAPIV4GeoIP("1.1.1.1", 2*time.Second, "cn", false)
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
		t.Fatalf("transport factory calls = %d, want 1 under concurrency", got)
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
	if client.httpClient.Timeout < NextTraceAPIV4MinTimeout {
		t.Fatalf("httpClient.Timeout = %s, want at least %s", client.httpClient.Timeout, NextTraceAPIV4MinTimeout)
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

func TestDecodeNextTraceAPIV4GeoPrefersDomainForOwner(t *testing.T) {
	body := []byte(`{
		"ip":"203.0.113.8",
		"asnumber":"4808",
		"country":"中国",
		"country_en":"China",
		"prov":"北京",
		"prov_en":"Beijing",
		"city":"北京",
		"city_en":"Beijing",
		"district":"海淀",
		"owner":"中国联通",
		"domain":"chinaunicom.cn 联通",
		"isp":"联通",
		"whois":"China Unicom Beijing Province Network",
		"lat":39.9042,
		"lng":116.4074,
		"prefix":"203.0.113.0/24",
		"router":{"4808":["4837"]},
		"source":"nexttrace"
	}`)

	geo, err := decodeNextTraceAPIV4Geo(body)
	if err != nil {
		t.Fatalf("decodeNextTraceAPIV4Geo() error = %v", err)
	}
	want := &IPGeoData{
		IP:        "203.0.113.8",
		Asnumber:  "4808",
		Country:   "中国",
		CountryEn: "China",
		Prov:      "北京",
		ProvEn:    "Beijing",
		City:      "北京",
		CityEn:    "Beijing",
		District:  "海淀",
		Owner:     "chinaunicom.cn 联通",
		Isp:       "联通",
		Domain:    "chinaunicom.cn 联通",
		Whois:     "China Unicom Beijing Province Network",
		Lat:       39.9042,
		Lng:       116.4074,
		Prefix:    "203.0.113.0/24",
		Router:    map[string][]string{"4808": {"4837"}},
		Source:    "nexttrace",
	}
	if !reflect.DeepEqual(geo, want) {
		t.Fatalf("geo = %+v, want %+v", geo, want)
	}
}

func TestDecodeNextTraceAPIV4GeoFallsBackToRawOwner(t *testing.T) {
	geo, err := decodeNextTraceAPIV4Geo([]byte(`{
		"ip":"203.0.113.8",
		"owner":"中国联通",
		"domain":""
	}`))
	if err != nil {
		t.Fatalf("decodeNextTraceAPIV4Geo() error = %v", err)
	}
	if geo.Owner != "中国联通" {
		t.Fatalf("Owner = %q, want 中国联通", geo.Owner)
	}
	if geo.Domain != "" {
		t.Fatalf("Domain = %q, want empty", geo.Domain)
	}
}

func TestDecodeNextTraceAPIV4GeoOwnerMatchesV3(t *testing.T) {
	const ip = "203.0.113.8"
	raw := []byte(`{
		"ip":"203.0.113.8",
		"owner":"中国联通",
		"domain":"chinaunicom.cn 联通"
	}`)

	ch := make(chan IPGeoData, 1)
	IPPools.poolMux.Lock()
	oldPool := IPPools.pool
	IPPools.pool = map[string]chan IPGeoData{ip: ch}
	IPPools.poolMux.Unlock()
	defer func() {
		IPPools.poolMux.Lock()
		IPPools.pool = oldPool
		IPPools.poolMux.Unlock()
	}()

	dispatchNextTraceAPIV3Message(string(raw))
	var v3Geo IPGeoData
	select {
	case v3Geo = <-ch:
	default:
		t.Fatal("v3 parser did not deliver geo data")
	}
	v4Geo, err := decodeNextTraceAPIV4Geo(raw)
	if err != nil {
		t.Fatalf("decodeNextTraceAPIV4Geo() error = %v", err)
	}
	if v4Geo.Owner != v3Geo.Owner {
		t.Fatalf("v4 Owner = %q, want v3 Owner %q", v4Geo.Owner, v3Geo.Owner)
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
