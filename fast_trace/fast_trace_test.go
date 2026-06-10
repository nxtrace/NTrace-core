package fastTrace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
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

func TestPromptFastTraceChoiceDeadlineExceededContext(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	choice, ok := promptFastTraceChoice(ctx, "请选择选项：", "1")
	if ok {
		t.Fatal("promptFastTraceChoice ok = true, want false for deadline exceeded context")
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

func TestTestFileSkipsFastTraceWSWhenRuntimePrepared(t *testing.T) {
	file := emptyFastTraceFile(t)
	oldInit := initFastTraceWSFn
	oldClose := closeFastTraceWSFn
	var initCalls int
	var closeCalls int
	initFastTraceWSFn = func(context.Context) *wshandle.WsConn {
		initCalls++
		return nil
	}
	closeFastTraceWSFn = func(*wshandle.WsConn) {
		closeCalls++
	}
	t.Cleanup(func() {
		initFastTraceWSFn = oldInit
		closeFastTraceWSFn = oldClose
	})

	testFile(ParamsFastTrace{
		Context:         context.Background(),
		File:            file,
		RuntimePrepared: true,
	}, trace.ICMPTrace)

	if initCalls != 0 {
		t.Fatalf("initFastTraceWS calls = %d, want 0 when runtime is prepared", initCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("closeFastTraceWS calls = %d, want 0 when runtime is prepared", closeCalls)
	}
}

func TestTestFileInitializesFastTraceWSByDefault(t *testing.T) {
	isolateFastTraceNextTraceAPIV4TokenFiles(t)
	file := emptyFastTraceFile(t)
	oldInit := initFastTraceWSFn
	oldClose := closeFastTraceWSFn
	var initCalls int
	var closeCalls int
	initFastTraceWSFn = func(context.Context) *wshandle.WsConn {
		initCalls++
		return nil
	}
	closeFastTraceWSFn = func(*wshandle.WsConn) {
		closeCalls++
	}
	t.Cleanup(func() {
		initFastTraceWSFn = oldInit
		closeFastTraceWSFn = oldClose
	})

	testFile(ParamsFastTrace{
		Context: context.Background(),
		File:    file,
	}, trace.ICMPTrace)

	if initCalls != 1 {
		t.Fatalf("initFastTraceWS calls = %d, want 1 by default", initCalls)
	}
	if closeCalls != 1 {
		t.Fatalf("closeFastTraceWS calls = %d, want 1 by default", closeCalls)
	}
}

func TestTestFileSkipsFastTraceWSWhenAPIV4TokenConfigured(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "v4-token")
	file := emptyFastTraceFile(t)
	oldInit := initFastTraceWSFn
	oldClose := closeFastTraceWSFn
	var initCalls int
	var closeCalls int
	initFastTraceWSFn = func(context.Context) *wshandle.WsConn {
		initCalls++
		return nil
	}
	closeFastTraceWSFn = func(*wshandle.WsConn) {
		closeCalls++
	}
	t.Cleanup(func() {
		initFastTraceWSFn = oldInit
		closeFastTraceWSFn = oldClose
	})

	testFile(ParamsFastTrace{
		Context: context.Background(),
		File:    file,
	}, trace.ICMPTrace)

	if initCalls != 0 {
		t.Fatalf("initFastTraceWS calls = %d, want 0 when API v4 token is configured", initCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("closeFastTraceWS calls = %d, want 0 when API v4 token is configured", closeCalls)
	}
}

func emptyFastTraceFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "targets.txt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile targets: %v", err)
	}
	return path
}

func isolateFastTraceNextTraceAPIV4TokenFiles(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "")
}
