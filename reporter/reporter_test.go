package reporter

import (
	"net"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

var testResult = &trace.Result{
	Hops: [][]trace.Hop{
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("192.168.3.1")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "4808",
					Country:  "中国",
					Prov:     "北京市",
					City:     "北京市",
					District: "北京市",
					Owner:    "",
					Isp:      "中国联通",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("114.249.16.1")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "4808",
					Country:  "中国",
					Prov:     "北京市",
					City:     "北京市",
					District: "北京市",
					Owner:    "",
					Isp:      "中国联通",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("219.158.5.150")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "4837",
					Country:  "中国",
					Prov:     "",
					City:     "",
					District: "",
					Owner:    "",
					Isp:      "中国联通",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("62.115.125.160")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "1299",
					Country:  "Sweden",
					Prov:     "Stockholm County",
					City:     "Stockholm",
					District: "",
					Owner:    "",
					Isp:      "Telia Company AB",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("213.226.68.73")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "56630",
					Country:  "Germany",
					Prov:     "Hesse, Frankfurt",
					City:     "",
					District: "",
					Owner:    "",
					Isp:      "Melbikomas UAB",
				},
			},
		},
	},
}

func TestPrint(t *testing.T) {
	r := New(testResult, "213.226.68.73")
	r.Print()
}
