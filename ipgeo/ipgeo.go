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

func GetSource(s string) Source {
	switch s {
	case "LeoMoeAPI":
		return LeoIP
	case "IP.SB":
		return IPSB
	case "IPInsight":
		return IPInSight
	default:
		return nil
	}
}
