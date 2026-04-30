package server

import (
	"context"
	"crypto/tls"
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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/options", nil)

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
}

func TestDeployAuthRejectsProtectedRoutesWithoutToken(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	for _, path := range []string{"/", "/assets/app.js", "/api/options", "/ws/trace", "/mcp"} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		req.Header.Set("Accept", "application/json")

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, resp.Code)
		}
	}
}

func TestDeployAuthEnabledWithoutTokenFailsClosed(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true})
	for _, path := range []string{"/", "/assets/app.js", "/api/options", "/ws/trace", "/mcp"} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		req.Header.Set("Accept", "application/json")

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", path, resp.Code)
		}
	}
}

func TestDeployAuthLoginEnabledWithoutTokenFailsClosed(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true})
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), method, "/auth/login", nil)
		req.Header.Set("Accept", "application/json")

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("%s status = %d, want 500", method, resp.Code)
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
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/mcp", nil)
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
	loginReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", strings.NewReader(form.Encode()))
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
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200", resp.Code)
	}
}

func TestDeployAuthLoginAcceptDefaultsToHTML(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})

	invalidResp := httptest.NewRecorder()
	invalidForm := url.Values{"token": {"wrong"}}
	invalidReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", strings.NewReader(invalidForm.Encode()))
	invalidReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(invalidResp, invalidReq)

	if invalidResp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid status = %d, want 401", invalidResp.Code)
	}
	if got := invalidResp.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("invalid content-type = %q, want text/html", got)
	}

	loginResp := httptest.NewRecorder()
	loginForm := url.Values{"token": {"secret"}}
	loginReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", strings.NewReader(loginForm.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(loginResp, loginReq)

	if loginResp.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302", loginResp.Code)
	}
}

func TestDeployAuthLoginJSONAcceptUsesJSON(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	invalidResp := httptest.NewRecorder()
	invalidForm := url.Values{"token": {"wrong"}}
	invalidReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", strings.NewReader(invalidForm.Encode()))
	invalidReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalidReq.Header.Set("Accept", "application/json")

	router.ServeHTTP(invalidResp, invalidReq)

	if invalidResp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid status = %d, want 401", invalidResp.Code)
	}
	if got := invalidResp.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("invalid content-type = %q, want application/json", got)
	}

	loginResp := httptest.NewRecorder()
	loginForm := url.Values{"token": {"secret"}}
	loginReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", strings.NewReader(loginForm.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Set("Accept", "application/json")

	router.ServeHTTP(loginResp, loginReq)

	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.Code)
	}
	if got := loginResp.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("login content-type = %q, want application/json", got)
	}
}

func TestDeployAuthLoginCookieSecureFollowsHTTPS(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	form := url.Values{"token": {"secret"}}
	tests := []struct {
		name       string
		target     string
		headerName string
		headerVal  string
		wantSecure bool
	}{
		{name: "plain http", target: "/auth/login"},
		{name: "direct https", target: "https://nexttrace.local/auth/login", wantSecure: true},
		{name: "forwarded proto", target: "/auth/login", headerName: "X-Forwarded-Proto", headerVal: "https", wantSecure: true},
		{name: "forwarded header", target: "/auth/login", headerName: "Forwarded", headerVal: "for=192.0.2.1;proto=https", wantSecure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.target, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if strings.HasPrefix(tt.target, "https://") {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.headerName != "" {
				req.Header.Set(tt.headerName, tt.headerVal)
			}

			router.ServeHTTP(resp, req)

			cookies := resp.Result().Cookies()
			if len(cookies) == 0 {
				t.Fatal("login did not set cookie")
			}
			authCookie := findDeployAuthCookie(t, cookies)
			if got := authCookie.Secure; got != tt.wantSecure {
				t.Fatalf("cookie Secure = %t, want %t", got, tt.wantSecure)
			}
		})
	}
}

func findDeployAuthCookie(t *testing.T, cookies []*http.Cookie) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == deployAuthCookieName {
			return cookie
		}
	}
	t.Fatalf("login did not set %s cookie; got %#v", deployAuthCookieName, cookies)
	return nil
}

func TestDeployAuthDoesNotAcceptQueryToken(t *testing.T) {
	router := newDeployAuthTestRouter(deployAuth{Enabled: true, Token: "secret"})
	resp := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/options?token=secret", nil)
	req.Header.Set("Accept", "application/json")

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}
