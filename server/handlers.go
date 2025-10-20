package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	supportedProtocols = []string{"icmp", "udp", "tcp"}
	dataProviders      = []string{
		"LeoMoeAPI",
		"IP.SB",
		"IPInsight",
		"IPInfo",
		"IPInfoLocal",
		"IPAPI.com",
		"Ip2region",
		"chunzhen",
		"DN42",
		"disable-geoip",
		"ipdb.one",
	}
	defaults = map[string]any{
		"protocol":          "icmp",
		"queries":           3,
		"max_hops":          30,
		"timeout_ms":        1000,
		"packet_size":       52,
		"parallel_requests": 18,
		"begin_hop":         1,
		"language":          "cn",
		"data_provider":     "LeoMoeAPI",
		"disable_maptrace":  false,
	}
)

func optionsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"protocols":      supportedProtocols,
		"dataProviders":  dataProviders,
		"defaultOptions": defaults,
	})
}
