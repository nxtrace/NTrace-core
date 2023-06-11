package example

import (
	"log"
	"net"
	"testing"
	"time"

	"github.com/sjlleo/nexttrace-core/trace"
)

func traceroute() {
	var test_config = trace.Config{
		DestIP:           net.IPv4(1, 1, 1, 1),
		DestPort:         443,
		ParallelRequests: 30,
		NumMeasurements:  1,
		BeginHop:         1,
		MaxHops:          30,
		TTLInterval:      1 * time.Millisecond,
		Timeout:          2 * time.Second,
		TraceMethod:      trace.ICMPTrace,
	}
	traceInstance, err := trace.NewTracer(test_config)
	if err != nil {
		log.Println(err)
		return
	}

	res, err := traceInstance.Traceroute()
	if err != nil {
		log.Println(err)
	}
	log.Println(res)
}

func TestTraceToCloudflareDNS(t *testing.T) {
	traceroute()
}
