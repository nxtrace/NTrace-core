package fastTrace

import (
	"fmt"
	"os"
	"os/signal"
	"testing"

	"github.com/xgadget-lab/nexttrace/trace"
	"github.com/xgadget-lab/nexttrace/wshandle"
)

func TestTrace(t *testing.T) {
	ft := FastTracer{}
	// 建立 WebSocket 连接
	w := wshandle.New()
	w.Interrupt = make(chan os.Signal, 1)
	signal.Notify(w.Interrupt, os.Interrupt)
	defer func() {
		w.Conn.Close()
	}()
	fmt.Println("TCP v4")
	ft.TracerouteMethod = trace.TCPTrace
	ft.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("TCP v6")
	//ft.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("ICMP v4")
	//ft.TracerouteMethod = trace.ICMPTrace
	//ft.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("ICMP v6")
	//ft.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
}
