package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

var (
	supportedProtocols = []string{"icmp", "udp", "tcp"}
	dataProviders      = []string{
		ipgeo.NextTraceAPIProvider,
		"IP.SB",
		"IPInsight",
		"IPInfo",
		"IPInfoLocal",
		"ip-api.com",
		"chunzhen",
		"DN42",
		"disable-geoip",
		"ipdb.one",
	}
	defaults = map[string]any{
		"protocol":          "icmp",
		"queries":           3,
		"max_hops":          30,
		"timeout_ms":        3000,
		"packet_size":       nil,
		"tos":               0,
		"parallel_requests": 18,
		"begin_hop":         1,
		"language":          "cn",
		"data_provider":     ipgeo.NextTraceAPIProvider,
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
