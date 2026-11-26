package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/nxtrace/NTrace-core/util"
)

func TestTraceUpgraderCheckOrigin_DefaultsToSameOriginOnly(t *testing.T) {
	t.Setenv(util.EnvAllowCrossOriginKey, "0")

	if !traceUpgrader.CheckOrigin(&http.Request{Host: "127.0.0.1:1080"}) {
		t.Fatal("requests without Origin header should be allowed")
	}

	if !traceUpgrader.CheckOrigin(&http.Request{
		Host:   "127.0.0.1:1080",
		Header: http.Header{"Origin": []string{"http://127.0.0.1:1080"}},
	}) {
		t.Fatal("same-origin websocket request should be allowed")
	}

	if traceUpgrader.CheckOrigin(&http.Request{
		Host:   "127.0.0.1:1080",
		Header: http.Header{"Origin": []string{"https://evil.example"}},
	}) {
		t.Fatal("cross-origin websocket request should be rejected by default")
	}
}

func TestTraceUpgraderCheckOrigin_CanBeRelaxedViaEnv(t *testing.T) {
	t.Setenv(util.EnvAllowCrossOriginKey, "1")

	if !traceUpgrader.CheckOrigin(&http.Request{
		Host:   "127.0.0.1:1080",
		Header: http.Header{"Origin": []string{"https://evil.example"}},
	}) {
		t.Fatal("cross-origin websocket request should be allowed when env is enabled")
	}
}

func TestBrowserAccessMiddleware_DefaultRejectsCrossOriginHTTP(t *testing.T) {
	t.Setenv(util.EnvAllowCrossOriginKey, "0")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(browserAccessMiddleware(true))
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Host = "127.0.0.1:1080"
	req.Header.Set("Origin", "https://evil.example")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusForbidden)
	}
}

func TestBrowserAccessMiddleware_CanEnableCORSViaEnv(t *testing.T) {
	t.Setenv(util.EnvAllowCrossOriginKey, "1")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(browserAccessMiddleware(true))
	router.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/trace", nil)
	req.Host = "127.0.0.1:1080"
	req.Header.Set("Origin", "https://evil.example")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNoContent)
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "https://evil.example" {
		t.Fatalf("allow-origin = %q, want %q", got, "https://evil.example")
	}
	if got := resp.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, DELETE, OPTIONS" {
		t.Fatalf("allow-methods = %q, want DELETE included", got)
	}

	req.Host = "example.test:1080"
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden || resp.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("non-loopback preflight status = %d, headers = %v", resp.Code, resp.Header())
	}
}

func TestDeployRouterTokenlessHostPolicy(t *testing.T) {
	t.Setenv(util.EnvAllowCrossOriginKey, "0")
	gin.SetMode(gin.TestMode)
	router := newDeployRouter(deployAuth{}, true)

	for _, path := range []string{"/", "/api/options", "/ws/trace", "/mcp"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "example.test:1080"
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusForbidden {
			t.Errorf("GET %s with non-loopback Host: status = %d, want 403", path, resp.Code)
		}
	}

	for _, host := range []string{"LOCALHOST", "127.0.0.2:1080", "[::1]:1080", "[::1]"} {
		req := httptest.NewRequest(http.MethodGet, "/api/options", nil)
		req.Host = host
		req.Header.Set("Origin", "http://"+host)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Errorf("loopback Host %q: status = %d, want 200", host, resp.Code)
		}
	}
}

func TestDeployRouterAllowsNonLoopbackHostWithToken(t *testing.T) {
	t.Setenv(util.EnvAllowCrossOriginKey, "0")
	gin.SetMode(gin.TestMode)
	router := newDeployRouter(deployAuth{Enabled: true, Token: "test-token"}, false)
	req := httptest.NewRequest(http.MethodGet, "/api/options", nil)
	req.Host = "nexttrace.example.test"
	req.Header.Set("Origin", "https://nexttrace.example.test")
	req.Header.Set("Authorization", "Bearer test-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("token-authenticated request: status = %d, want 200", resp.Code)
	}
}
