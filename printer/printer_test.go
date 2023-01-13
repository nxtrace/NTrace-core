package printer

// func TestPrintTraceRouteNav(t *testing.T) {
// 	PrintTraceRouteNav(util.DomainLookUp("1.1.1.1", false), "1.1.1.1", "dataOrigin")
// }

// var testGeo = &ipgeo.IPGeoData{
// 	Asnumber: "TestAsnumber",
// 	Country:  "TestCountry",
// 	Prov:     "TestProv",
// 	City:     "TestCity",
// 	District: "TestDistrict",
// 	Owner:    "TestOwner",
// 	Isp:      "TestIsp",
// }

// var testResult = &trace.Result{
// 	Hops: [][]trace.Hop{
// 		{
// 			{
// 				Success:  true,
// 				Address:  &net.IPAddr{IP: net.ParseIP("192.168.3.1")},
// 				Hostname: "test",
// 				TTL:      0,
// 				RTT:      10 * time.Millisecond,
// 				Error:    nil,
// 				Geo:      testGeo,
// 			},
// 			{
// 				Success:  true,
// 				Address:  &net.IPAddr{IP: net.ParseIP("192.168.3.1")},
// 				Hostname: "test",
// 				TTL:      0,
// 				RTT:      10 * time.Millisecond,
// 				Error:    nil,
// 				Geo:      testGeo,
// 			},
// 		},
// 		{
// 			{
// 				Success:  false,
// 				Address:  nil,
// 				Hostname: "",
// 				TTL:      0,
// 				RTT:      0,
// 				Error:    errors.New("test error"),
// 				Geo:      nil,
// 			},
// 			{
// 				Success:  true,
// 				Address:  &net.IPAddr{IP: net.ParseIP("192.168.3.1")},
// 				Hostname: "test",
// 				TTL:      0,
// 				RTT:      10 * time.Millisecond,
// 				Error:    nil,
// 				Geo:      nil,
// 			},
// 		},
// 		{
// 			{
// 				Success:  true,
// 				Address:  &net.IPAddr{IP: net.ParseIP("192.168.3.1")},
// 				Hostname: "test",
// 				TTL:      0,
// 				RTT:      0,
// 				Error:    nil,
// 				Geo:      &ipgeo.IPGeoData{},
// 			},
// 			{
// 				Success:  true,
// 				Address:  &net.IPAddr{IP: net.ParseIP("192.168.3.1")},
// 				Hostname: "",
// 				TTL:      0,
// 				RTT:      10 * time.Millisecond,
// 				Error:    nil,
// 				Geo:      testGeo,
// 			},
// 		},
// 	},
// }

// // func TestTraceroutePrinter(t *testing.T) {
// // 	TraceroutePrinter(testResult)
// // }

// func TestTracerouteTablePrinter(t *testing.T) {
// 	TracerouteTablePrinter(testResult)
// }

// func TestRealtimePrinter(t *testing.T) {
// 	RealtimePrinter(testResult, 0)
// 	// RealtimePrinter(testResult, 1)
// 	// RealtimePrinter(testResult, 2)
// }
