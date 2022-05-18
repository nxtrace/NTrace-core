package reporter

import (
	"testing"
	"time"

	"github.com/xgadget-lab/nexttrace/ipgeo"
	"github.com/xgadget-lab/nexttrace/trace"
	"github.com/xgadget-lab/nexttrace/util"
)

func TestPrint(t *testing.T) {
	ip := util.DomainLookUp("213.226.68.73")
	var m trace.Method = "tcp"
	var conf = trace.Config{
		DestIP:           ip,
		DestPort:         80,
		MaxHops:          30,
		NumMeasurements:  1,
		ParallelRequests: 1,
		RDns:             true,
		IPGeoSource:      ipgeo.GetSource("LeoMoeAPI"),
		Timeout:          2 * time.Second,

		//Quic:    false,
	}

	res, _ := trace.Traceroute(m, conf)
	r := New(res, ip.String())
	r.Print()
}
