package fastTrace

import (
	"context"
	"testing"
)

func TestTrace(t *testing.T) {
	//pFastTrace := ParamsFastTrace{
	//	SrcDev:         "",
	//	SrcAddr:        "",
	//	BeginHop:       1,
	//	MaxHops:        30,
	//	RDNS:           false,
	//	AlwaysWaitRDNS: false,
	//	Lang:           "",
	//	PktSize:        52,
	//}
	//ft := FastTracer{ParamsFastTrace: pFastTrace}
	//// 建立 WebSocket 连接
	//w := wshandle.New()
	//w.Interrupt = make(chan os.Signal, 1)
	//signal.Notify(w.Interrupt, os.Interrupt)
	//defer func() {
	//	w.Conn.Close()
	//}()
	//fmt.Println("TCP v4")
	//ft.TracerouteMethod = trace.TCPTrace
	//ft.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("TCP v6")
	//ft.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("ICMP v4")
	//ft.TracerouteMethod = trace.ICMPTrace
	//ft.tracert(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
	//fmt.Println("ICMP v6")
	//ft.tracert_v6(TestIPsCollection.Beijing.Location, TestIPsCollection.Beijing.EDU)
}

func TestPromptFastTraceChoiceCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	choice, ok := promptFastTraceChoice(ctx, "请选择选项：", "1")
	if ok {
		t.Fatal("promptFastTraceChoice ok = true, want false for canceled context")
	}
	if choice != "" {
		t.Fatalf("promptFastTraceChoice choice = %q, want empty", choice)
	}
}

func TestReadFastTestv6ChoiceCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	choice, ok := readFastTestv6Choice(ctx)
	if ok {
		t.Fatal("readFastTestv6Choice ok = true, want false for canceled context")
	}
	if choice != "" {
		t.Fatalf("readFastTestv6Choice choice = %q, want empty", choice)
	}
}
