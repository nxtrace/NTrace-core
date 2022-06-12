package fastTrace

import (
	"testing"

	"github.com/xgadget-lab/nexttrace/trace"
)

// ICMP Use Too Many Time to Wait So we don't test it.
func TestTCPTrace(t *testing.T) {
	ft := FastTracer{}
	ft.TracerouteMethod = trace.TCPTrace
	ft.testCM()
	ft.testEDU()
}
