package ipgeo

type IPGeoData struct {
	Asnumber string
	Country  string
	Prov     string
	City     string
	District string
	Owner    string
	Isp      string
}

type Source = func(ip string) (*IPGeoData, error)
