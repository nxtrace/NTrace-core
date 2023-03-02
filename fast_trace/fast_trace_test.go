package fastTrace

import (
	"os"
	"os/signal"
	"testing"

	"github.com/xgadget-lab/nexttrace/trace"
	"github.com/xgadget-lab/nexttrace/wshandle"
)

// ICMP Use Too Many Time to Wait So we don't test it.
func TestTCPTrace(t *testing.T) {
	ft := FastTracer{}
	// 建立 WebSocket 连接
	w := wshandle.New()
	w.Interrupt = make(chan os.Signal, 1)
	signal.Notify(w.Interrupt, os.Interrupt)
	defer func() {
		err := w.Conn.Close()
		if err != nil {
			return
		}
	}()
	ft.TracerouteMethod = trace.TCPTrace
	ft.testCM()
	ft.testEDU()
}
