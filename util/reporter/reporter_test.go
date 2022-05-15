package reporter

import (
	"testing"
	"time"

	"github.com/xgadget-lab/nexttrace/methods"
	"github.com/xgadget-lab/nexttrace/methods/tcp"
	"github.com/xgadget-lab/nexttrace/util"
)

func TestPrint(t *testing.T) {
	ip := util.DomainLookUp("213.226.68.73")
	tcpTraceroute := tcp.New(ip, methods.TracerouteConfig{
		MaxHops:          uint16(30),
		NumMeasurements:  uint16(1),
		ParallelRequests: uint16(12),
		Port:             80,
		Timeout:          time.Second / 2,
	})
	res, _ := tcpTraceroute.Start()
	util.Printer(&util.PrinterConfig{
		IP:          ip,
		DisplayMode: "classic",
		DataOrigin:  "LeoMoeAPI",
		Rdnsenable:  true,
		Results:     *res,
	})
	r := New(*res, ip.String())
	r.Print()
}
