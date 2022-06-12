package fastTrace

import (
	"testing"

	"github.com/xgadget-lab/nexttrace/trace"
)

func TestICMPTrace(t *testing.T) {
	ft := FastTracer{}
	ft.TracerouteMethod = trace.ICMPTrace
	ft.testCM()
}

func TestTCPTrace(t *testing.T) {
	ft := FastTracer{}
	ft.TracerouteMethod = trace.TCPTrace
	ft.testCM()
}
