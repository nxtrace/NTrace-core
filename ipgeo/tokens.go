package ipgeo

import "github.com/xgadget-lab/nexttrace/config"

type tokenData struct {
	ipinsight string
	ipinfo    string
	ipleo     string
}

var token = tokenData{
	ipinsight: "",
	ipinfo:    "",
	ipleo:     "NextTraceDemo",
}


func SetToken(c config.Token) {
	token.ipleo = c.LeoMoeAPI
	token.ipinfo = c.IPInfo
}
