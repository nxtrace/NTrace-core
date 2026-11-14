package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
)

func TestRunNaliModeV4WaitsForSlowLookup(t *testing.T) {
	for _, tc := range []struct {
		name      string
		timeoutMs int
		delay     time.Duration
	}{
		{name: "short explicit timeout", timeoutMs: 1000, delay: 1200 * time.Millisecond},
		{name: "longer explicit timeout", timeoutMs: 3500, delay: 2200 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v4/ipGeo" || r.URL.Query().Get("ip") != "218.33.109.164" {
					t.Errorf("unexpected GeoIP request: %s", r.URL)
				}
				time.Sleep(tc.delay)
				w.Header().Set("Connection", "close")
				_, _ = io.WriteString(w, `{"ip":"218.33.109.164","asnumber":"153371","country":"日本"}`)
			}))
			defer upstream.Close()
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodConnect || r.Host != "api.nxtrace.org:443" {
					t.Errorf("unexpected proxy request: %s %s", r.Method, r.Host)
					http.Error(w, "unexpected proxy request", http.StatusBadRequest)
					return
				}
				remote, err := net.Dial("tcp", upstream.Listener.Addr().String())
				if err != nil {
					t.Error(err)
					return
				}
				defer remote.Close()
				client, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				defer client.Close()
				_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
				go func() { _, _ = io.Copy(remote, client) }()
				_, _ = io.Copy(client, remote)
			}))
			defer proxy.Close()

			oldProxy, oldTransport := util.EnvProxyURL, http.DefaultTransport
			oldPolicy := util.CurrentGeoDNSPolicy()
			proxyURL, err := url.Parse(proxy.URL)
			if err != nil {
				t.Fatal(err)
			}
			transport := http.DefaultTransport.(*http.Transport).Clone()
			// All TLS connections are tunneled to the local test server.
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			transport.Proxy = http.ProxyURL(proxyURL)
			http.DefaultTransport = transport
			util.EnvProxyURL = proxy.URL
			t.Setenv(util.EnvNextTraceAPIV4TokenKey, "test-token")
			trace.ClearCaches()
			t.Cleanup(func() {
				util.EnvProxyURL, http.DefaultTransport = oldProxy, oldTransport
				util.SetGeoDNSResolver(oldPolicy.Resolver())
				transport.CloseIdleConnections()
				trace.ClearCaches()
			})

			var out bytes.Buffer
			err = runNaliMode(context.Background(), naliRunOptions{
				stdout: &out, data: ipgeo.NextTraceAPIProvider, pow: "api.nxtrace.org",
				dot: strings.TrimPrefix(proxy.URL, "http://"), lang: "cn",
				timeoutMs: tc.timeoutMs, target: "218.33.109.164",
			})
			if err != nil {
				t.Fatalf("runNaliMode() error = %v", err)
			}
			if got, want := out.String(), "218.33.109.164 [AS153371, 日本]\n"; got != want {
				t.Fatalf("runNaliMode() output = %q, want %q", got, want)
			}
		})
	}
}
