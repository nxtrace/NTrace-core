package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const deployAuthCookieName = "nexttrace_deploy_auth"

type deployAuth struct {
	Enabled bool
	Token   string
}

func (a deployAuth) tokenConfigured() bool {
	return strings.TrimSpace(a.Token) != ""
}

func deployAuthMiddleware(auth deployAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.Enabled || c.Request.Method == http.MethodOptions || isDeployAuthRoute(c.Request.URL.Path) {
			c.Next()
			return
		}
		if !auth.tokenConfigured() {
			writeDeployUnauthorized(c)
			c.Abort()
			return
		}
		if deployRequestAuthorized(c.Request, auth.Token) {
			c.Next()
			return
		}
		writeDeployUnauthorized(c)
		c.Abort()
	}
}

func isDeployAuthRoute(path string) bool {
	return path == "/auth/login"
}

func deployRequestAuthorized(r *http.Request, token string) bool {
	if deployTokenMatches(bearerToken(r.Header.Get("Authorization")), token) {
		return true
	}
	if deployTokenMatches(r.Header.Get("X-NextTrace-Token"), token) {
		return true
	}
	if cookie, err := r.Cookie(deployAuthCookieName); err == nil {
		return deployTokenMatches(cookie.Value, deployCookieValue(token))
	}
	return false
}

func bearerToken(header string) string {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func deployTokenMatches(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func deployCookieValue(token string) string {
	sum := sha256.Sum256([]byte("nexttrace-deploy-auth\x00" + token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func deployRequestIsHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if forwardedProtoIsHTTPS(r.Header.Get("X-Forwarded-Proto")) {
		return true
	}
	return forwardedHeaderProtoIsHTTPS(r.Header.Get("Forwarded"))
}

func forwardedProtoIsHTTPS(value string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), "https") {
			return true
		}
	}
	return false
}

func forwardedHeaderProtoIsHTTPS(value string) bool {
	for _, forwarded := range strings.Split(value, ",") {
		for _, part := range strings.Split(forwarded, ";") {
			key, val, ok := strings.Cut(strings.TrimSpace(part), "=")
			if !ok || !strings.EqualFold(key, "proto") {
				continue
			}
			if strings.EqualFold(strings.Trim(strings.TrimSpace(val), `"`), "https") {
				return true
			}
		}
	}
	return false
}

func registerDeployAuthRoutes(router *gin.Engine, auth deployAuth) {
	router.GET("/auth/login", func(c *gin.Context) {
		if !auth.Enabled {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if !auth.tokenConfigured() {
			writeDeployAuthConfigError(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(deployLoginPage("")))
	})
	router.POST("/auth/login", func(c *gin.Context) {
		if !auth.Enabled {
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
		if !auth.tokenConfigured() {
			writeDeployAuthConfigError(c)
			return
		}
		token := strings.TrimSpace(c.PostForm("token"))
		if token == "" {
			var body struct {
				Token string `json:"token"`
			}
			_ = c.ShouldBindJSON(&body)
			token = strings.TrimSpace(body.Token)
		}
		if !deployTokenMatches(token, auth.Token) {
			if acceptsHTML(c.Request) {
				c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(deployLoginPage("Invalid token")))
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     deployAuthCookieName,
			Value:    deployCookieValue(auth.Token),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   deployRequestIsHTTPS(c.Request),
		})
		if acceptsHTML(c.Request) {
			c.Redirect(http.StatusFound, "/")
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

func writeDeployAuthConfigError(c *gin.Context) {
	if c.Request.Method == http.MethodGet && acceptsHTML(c.Request) {
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(deployLoginPage("Deploy auth token is not configured")))
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "deploy auth token is not configured"})
}

func writeDeployUnauthorized(c *gin.Context) {
	if c.Request.Method == http.MethodGet && acceptsHTML(c.Request) {
		c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(deployLoginPage("")))
		return
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "deploy token required"})
}

func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func deployLoginPage(message string) string {
	msg := ""
	if strings.TrimSpace(message) != "" {
		msg = `<p class="error">` + html.EscapeString(message) + `</p>`
	}
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>NextTrace Login</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:0;min-height:100vh;display:grid;place-items:center;background:#f6f8fa;color:#1f2328}
main{width:min(360px,calc(100vw - 32px));background:#fff;border:1px solid #d0d7de;border-radius:8px;padding:24px;box-shadow:0 8px 24px rgba(140,149,159,.2)}
h1{font-size:20px;margin:0 0 16px}
label{display:block;font-size:14px;margin-bottom:8px}
input{box-sizing:border-box;width:100%;font:inherit;padding:10px 12px;border:1px solid #d0d7de;border-radius:6px}
button{margin-top:16px;width:100%;font:inherit;font-weight:600;padding:10px 12px;border:0;border-radius:6px;background:#0969da;color:white;cursor:pointer}
.error{color:#cf222e;margin:0 0 12px;font-size:14px}
</style>
</head>
<body>
<main>
<h1>NextTrace Web Console</h1>
` + msg + `
<form method="post" action="/auth/login">
<label for="token">Deploy token</label>
<input id="token" name="token" type="password" autocomplete="current-password" autofocus>
<button type="submit">Sign in</button>
</form>
</main>
</body>
</html>`
}
