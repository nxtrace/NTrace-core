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
	router.Use(browserAccessMiddleware())
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
	router.Use(browserAccessMiddleware())
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
}
