package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nxtrace/NTrace-core/util"
)

const (
	maxTraceRequestBodyBytes = 64 << 10
	maxWSInitMessageBytes    = 64 << 10
)

func browserOriginAllowed(r *http.Request) bool {
	if util.AllowCrossOriginBrowserAccess() {
		return true
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}

	return strings.EqualFold(u.Host, r.Host)
}

func isLoopbackRequestHost(authority string) bool {
	host, _, err := net.SplitHostPort(authority)
	if err != nil {
		host = authority
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			return net.ParseIP(host[1 : len(host)-1]).IsLoopback()
		}
	}
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

func browserAccessMiddleware(requireLoopbackHost bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Matching Origin and Host alone does not protect tokenless deploy from DNS rebinding.
		if requireLoopbackHost && !isLoopbackRequestHost(c.Request.Host) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "deploy without a token requires a loopback host"})
			return
		}
		if !browserOriginAllowed(c.Request) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin browser access is disabled"})
			return
		}

		if util.AllowCrossOriginBrowserAccess() {
			if origin := strings.TrimSpace(c.Request.Header.Get("Origin")); origin != "" {
				h := c.Writer.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-NextTrace-Token")
			}
		}

		c.Next()
	}
}
