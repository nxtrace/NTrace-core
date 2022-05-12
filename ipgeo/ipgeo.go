package ipgeo

type IPGeoData struct {
	Asnumber string `json:"asnumber"`
	Country  string `json:"country"`
	Prov     string `json:"prov"`
	City     string `json:"city"`
	District string `json:"district"`
	Owner    string `json:"owner"`
	Isp      string `json:"isp"`
}

type Source = func(ip string) (IPGeoData, error)
