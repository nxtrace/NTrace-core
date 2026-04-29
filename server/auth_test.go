package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newDeployAuthTestRouter(auth deployAuth) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerDeployAuthRoutes(router, auth)
	router.Use(deployAuthMiddleware(auth))
	router.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "index") })
	router.GET("/assets/app.js", func(c *gin.Context) { c.String(http.StatusOK, "asset") })
	router.GET("/api/options", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	router.GET("/ws/trace", func(c *gin.Context) { c.String(http.StatusOK, "ws") })
	router.Any("/mcp", func(c *gin.Context) { c.String(http.StatusOK, "mcp") })
	return router
}

func TestDeployAuthDisabledAllowsRequests(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{})
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/options", nil)

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
}

func TestDeployAuthRejectsProtectedRoutesWithoutToken(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	for _, path := range []string{"/", "/assets/app.js", "/api/options", "/ws/trace", "/mcp"} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept", "application/json")

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, resp.Code)
		}
	}
}

func TestDeployAuthAcceptsBearerHeaderAndNextTraceTokenHeader(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"bearer", "Authorization", "Bearer secret"},
		{"nexttrace", "X-NextTrace-Token", "secret"},
	}
	for _, tt := range tests {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.Header.Set(tt.header, tt.value)

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", tt.name, resp.Code)
		}
	}
}

func TestDeployAuthLoginSetsCookieForBrowserAccess(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	form := url.Values{"token": {"secret"}}
	loginResp := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Set("Accept", "text/html")

	router.ServeHTTP(loginResp, loginReq)

	if loginResp.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302", loginResp.Code)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set cookie")
	}

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200", resp.Code)
	}
}

func TestDeployAuthDoesNotAcceptQueryToken(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/options?token=secret", nil)
	req.Header.Set("Accept", "application/json")

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}
