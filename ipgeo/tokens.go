package ipgeo

import "github.com/nxtrace/NTrace-core/util"

type tokenData struct {
	ipinsight string
	ipinfo    string
	ipleo     string
	baseUrl   string
}

func (t *tokenData) BaseOrDefault(def string) string {
	if t.baseUrl == "" {
		return def
	}
	return t.baseUrl
}

var token = tokenData{
	ipinsight: util.GetenvDefault("NEXTTRACE_IPINSIGHT_TOKEN", ""),
	ipinfo:    util.GetenvDefault("NEXTTRACE_IPINFO_TOKEN", ""),
	baseUrl:   util.GetenvDefault("NEXTTRACE_IPAPI_BASE", ""),
	ipleo:     "NextTraceDemo",
}
