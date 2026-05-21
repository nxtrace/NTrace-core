package ipgeo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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

func TestNextTraceAPIV4ClientLookupHTTPErrorMessages(t *testing.T) {
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
	client := NewNextTraceAPIV4Client("https://api.nxtrace.org/v4/ipGeo", "secret-token", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network down secret-token")
		}),
	})
	_, _, err := client.Lookup(context.Background(), "1.1.1.1")
	if err == nil {
		t.Fatal("Lookup() error = nil, want network error")
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
