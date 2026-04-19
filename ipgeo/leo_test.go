package ipgeo

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/wshandle"
)

func TestLeoIPWaitsForConnectionBeforeSending(t *testing.T) {
	oldGet := getLeoWsConn
	oldPools := IPPools.pool
	oldRunning, oldRestart := receiveParseState()
	var conn *wshandle.WsConn
	receiveParseStarted := make(chan struct{})
	var getConnCalls int32
	var closeStarted sync.Once
	defer func() {
		if conn != nil && conn.MsgReceiveCh != nil {
			close(conn.MsgReceiveCh)
		}
		waitForReceiveParseStart(t, receiveParseStarted)
		waitForReceiveParseStop(t)
		getLeoWsConn = oldGet
		IPPools.pool = oldPools
		setReceiveParseState(oldRunning, oldRestart)
	}()

	IPPools.pool = make(map[string]chan IPGeoData)
	setReceiveParseState(false, false)

	conn = &wshandle.WsConn{
		MsgSendCh:    make(chan string, 1),
		MsgReceiveCh: make(chan string, 1),
		Interrupt:    make(chan os.Signal, 1),
	}
	getLeoWsConn = func() *wshandle.WsConn {
		if atomic.AddInt32(&getConnCalls, 1) >= 3 {
			closeStarted.Do(func() { close(receiveParseStarted) })
		}
		return conn
	}

	sent := make(chan string, 1)
	go func() {
		msg := <-conn.MsgSendCh
		sent <- msg
		conn.MsgReceiveCh <- `{"ip":"1.1.1.1","asnumber":"13335"}`
	}()

	var (
		gotGeo *IPGeoData
		gotErr error
	)
	done := make(chan struct{})
	go func() {
		gotGeo, gotErr = LeoIP("1.1.1.1", 300*time.Millisecond, "en", false)
		close(done)
	}()

	select {
	case <-sent:
		t.Fatal("LeoIP sent request before websocket became connected")
	case <-time.After(60 * time.Millisecond):
	}

	conn.SetConnected(true)

	select {
	case msg := <-sent:
		if msg != "1.1.1.1" {
			t.Fatalf("sent request = %q, want 1.1.1.1", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("LeoIP did not send request after websocket became connected")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("LeoIP did not complete")
	}

	if gotErr != nil {
		t.Fatalf("LeoIP error = %v, want nil", gotErr)
	}
	if gotGeo == nil || gotGeo.Asnumber != "13335" {
		t.Fatalf("LeoIP geo = %+v, want ASN 13335", gotGeo)
	}
}

func TestLeoIPUsesSingleTimeoutBudget(t *testing.T) {
	oldGet := getLeoWsConn
	oldPools := IPPools.pool
	oldRunning, oldRestart := receiveParseState()
	var conn *wshandle.WsConn
	receiveParseStarted := make(chan struct{})
	var getConnCalls int32
	var closeStarted sync.Once
	defer func() {
		if conn != nil && conn.MsgReceiveCh != nil {
			close(conn.MsgReceiveCh)
		}
		waitForReceiveParseStart(t, receiveParseStarted)
		waitForReceiveParseStop(t)
		getLeoWsConn = oldGet
		IPPools.pool = oldPools
		setReceiveParseState(oldRunning, oldRestart)
	}()

	IPPools.pool = make(map[string]chan IPGeoData)
	setReceiveParseState(false, false)

	conn = &wshandle.WsConn{
		MsgSendCh:    make(chan string, 1),
		MsgReceiveCh: make(chan string),
		Interrupt:    make(chan os.Signal, 1),
	}
	getLeoWsConn = func() *wshandle.WsConn {
		if atomic.AddInt32(&getConnCalls, 1) >= 3 {
			closeStarted.Do(func() { close(receiveParseStarted) })
		}
		return conn
	}

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := LeoIP("1.1.1.1", 2*time.Second, "en", false)
		done <- err
	}()

	time.Sleep(1500 * time.Millisecond)
	conn.SetConnected(true)

	select {
	case err := <-done:
		if err == nil || err.Error() != "TimeOut" {
			t.Fatalf("LeoIP error = %v, want TimeOut", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("LeoIP exceeded expected shared timeout budget")
	}

	if elapsed := time.Since(start); elapsed > 2800*time.Millisecond {
		t.Fatalf("LeoIP elapsed = %s, want <= 2.8s", elapsed)
	}
}

func TestReceiveParseReturnsWhenWebsocketMissing(t *testing.T) {
	oldGet := getLeoWsConn
	defer func() { getLeoWsConn = oldGet }()

	getLeoWsConn = func() *wshandle.WsConn { return nil }

	done := make(chan struct{})
	go func() {
		receiveParse()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("receiveParse should return when websocket is nil")
	}
}

func TestReceiveParseContinuesAfterWebsocketReplacement(t *testing.T) {
	oldGet := getLeoWsConn
	oldPools := IPPools.pool
	defer func() {
		getLeoWsConn = oldGet
		IPPools.pool = oldPools
	}()

	oldConn := &wshandle.WsConn{
		MsgReceiveCh: make(chan string),
		Interrupt:    make(chan os.Signal, 1),
	}
	newConn := &wshandle.WsConn{
		MsgReceiveCh: make(chan string),
		Interrupt:    make(chan os.Signal, 1),
	}
	current := oldConn
	getLeoWsConn = func() *wshandle.WsConn {
		return current
	}

	oldResult := make(chan IPGeoData, 1)
	newResult := make(chan IPGeoData, 1)
	IPPools.pool = map[string]chan IPGeoData{
		"1.1.1.1": oldResult,
		"2.2.2.2": newResult,
	}

	done := make(chan struct{})
	go func() {
		receiveParse()
		close(done)
	}()

	select {
	case oldConn.MsgReceiveCh <- `{"ip":"1.1.1.1","asnumber":"13335"}`:
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not consume from the original websocket")
	}
	select {
	case geo := <-oldResult:
		if geo.Asnumber != "13335" {
			t.Fatalf("old websocket geo ASN = %q, want 13335", geo.Asnumber)
		}
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not dispatch original websocket data")
	}

	current = newConn
	close(oldConn.MsgReceiveCh)

	select {
	case newConn.MsgReceiveCh <- `{"ip":"2.2.2.2","asnumber":"64512"}`:
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not switch to replacement websocket")
	}
	select {
	case geo := <-newResult:
		if geo.Asnumber != "64512" {
			t.Fatalf("new websocket geo ASN = %q, want 64512", geo.Asnumber)
		}
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not dispatch replacement websocket data")
	}

	close(newConn.MsgReceiveCh)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not exit after replacement websocket closed")
	}
}

func TestStartReceiveParseRestartsAfterQueuedStart(t *testing.T) {
	oldGet := getLeoWsConn
	oldPools := IPPools.pool
	oldRunning, oldRestart := receiveParseState()
	var closeOld, closeNew sync.Once
	var oldConn, newConn *wshandle.WsConn
	defer func() {
		if oldConn != nil {
			closeOld.Do(func() { close(oldConn.MsgReceiveCh) })
		}
		if newConn != nil {
			closeNew.Do(func() { close(newConn.MsgReceiveCh) })
		}
		waitForReceiveParseStop(t)
		getLeoWsConn = oldGet
		IPPools.pool = oldPools
		setReceiveParseState(oldRunning, oldRestart)
	}()

	oldConn = &wshandle.WsConn{
		MsgReceiveCh: make(chan string),
		Interrupt:    make(chan os.Signal, 1),
	}
	newConn = &wshandle.WsConn{
		MsgReceiveCh: make(chan string),
		Interrupt:    make(chan os.Signal, 1),
	}
	afterOldClose := atomic.Bool{}
	afterOldCloseCalls := atomic.Int32{}
	getLeoWsConn = func() *wshandle.WsConn {
		if afterOldClose.Load() {
			if afterOldCloseCalls.Add(1) == 1 {
				return oldConn
			}
			return newConn
		}
		return oldConn
	}

	oldResult := make(chan IPGeoData, 1)
	newResult := make(chan IPGeoData, 1)
	IPPools.pool = map[string]chan IPGeoData{
		"1.1.1.1": oldResult,
		"2.2.2.2": newResult,
	}
	setReceiveParseState(false, false)

	startReceiveParse()
	select {
	case oldConn.MsgReceiveCh <- `{"ip":"1.1.1.1","asnumber":"13335"}`:
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not consume from the original websocket")
	}
	select {
	case geo := <-oldResult:
		if geo.Asnumber != "13335" {
			t.Fatalf("old websocket geo ASN = %q, want 13335", geo.Asnumber)
		}
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not dispatch original websocket data")
	}

	startReceiveParse()
	afterOldClose.Store(true)
	closeOld.Do(func() { close(oldConn.MsgReceiveCh) })

	select {
	case newConn.MsgReceiveCh <- `{"ip":"2.2.2.2","asnumber":"64512"}`:
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not restart for a queued start")
	}
	select {
	case geo := <-newResult:
		if geo.Asnumber != "64512" {
			t.Fatalf("new websocket geo ASN = %q, want 64512", geo.Asnumber)
		}
	case <-time.After(time.Second):
		t.Fatal("receiveParse did not dispatch restarted websocket data")
	}

	closeNew.Do(func() { close(newConn.MsgReceiveCh) })
}

func TestSendIPRequestHonorsContextWhenQueueIsFull(t *testing.T) {
	oldGet := getLeoWsConn
	defer func() { getLeoWsConn = oldGet }()

	conn := &wshandle.WsConn{
		MsgSendCh: make(chan string, 1),
		Interrupt: make(chan os.Signal, 1),
	}
	conn.MsgSendCh <- "blocked"
	getLeoWsConn = func() *wshandle.WsConn {
		return conn
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if sendIPRequest(ctx, conn, "1.1.1.1") {
		t.Fatal("sendIPRequest() = true, want false when context expires")
	}
}

func TestSendIPRequestUsesProvidedConnection(t *testing.T) {
	oldGet := getLeoWsConn
	defer func() { getLeoWsConn = oldGet }()

	conn := &wshandle.WsConn{
		MsgSendCh: make(chan string, 1),
		Interrupt: make(chan os.Signal, 1),
	}
	getLeoWsConn = func() *wshandle.WsConn { return nil }

	if !sendIPRequest(context.Background(), conn, "1.1.1.1") {
		t.Fatal("sendIPRequest() = false, want true")
	}
	select {
	case got := <-conn.MsgSendCh:
		if got != "1.1.1.1" {
			t.Fatalf("sent IP = %q, want 1.1.1.1", got)
		}
	default:
		t.Fatal("provided connection did not receive request")
	}
}

func TestDispatchLeoMessageReplacesStaleBufferedResponse(t *testing.T) {
	oldPools := IPPools.pool
	defer func() { IPPools.pool = oldPools }()

	ch := make(chan IPGeoData, 1)
	ch <- IPGeoData{Asnumber: "STALE"}
	IPPools.pool = map[string]chan IPGeoData{"1.1.1.1": ch}

	dispatchLeoMessage(`{"ip":"1.1.1.1","asnumber":"13335"}`)

	select {
	case geo := <-ch:
		if geo.Asnumber != "13335" {
			t.Fatalf("buffered geo ASN = %q, want latest response", geo.Asnumber)
		}
	default:
		t.Fatal("expected latest response to be buffered")
	}
}

func waitForReceiveParseStart(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("waitForReceiveParseStart: receiveParse did not start")
	}
}

func waitForReceiveParseStop(t *testing.T) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		running, _ := receiveParseState()
		if !running {
			return
		}
		select {
		case <-deadline:
			t.Fatal("waitForReceiveParseStop: receiveParse did not stop")
		case <-ticker.C:
		}
	}
}

func receiveParseState() (bool, bool) {
	receiveParseMu.Lock()
	defer receiveParseMu.Unlock()
	return receiveParseRunning, receiveParseRestart
}

func setReceiveParseState(running, restart bool) {
	receiveParseMu.Lock()
	defer receiveParseMu.Unlock()
	receiveParseRunning = running
	receiveParseRestart = restart
}
