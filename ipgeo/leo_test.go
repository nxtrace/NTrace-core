package ipgeo

import (
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
	oldOnce := receiveParseOnce
	var conn *wshandle.WsConn
	receiveParseStarted := make(chan struct{})
	var getConnCalls int32
	var closeStarted sync.Once
	defer func() {
		if conn != nil && conn.MsgReceiveCh != nil {
			close(conn.MsgReceiveCh)
		}
		waitForReceiveParseStart(receiveParseStarted)
		getLeoWsConn = oldGet
		IPPools.pool = oldPools
		receiveParseOnce = oldOnce
	}()

	IPPools.pool = make(map[string]chan IPGeoData)
	receiveParseOnce = &sync.Once{}

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
	oldOnce := receiveParseOnce
	var conn *wshandle.WsConn
	receiveParseStarted := make(chan struct{})
	var getConnCalls int32
	var closeStarted sync.Once
	defer func() {
		if conn != nil && conn.MsgReceiveCh != nil {
			close(conn.MsgReceiveCh)
		}
		waitForReceiveParseStart(receiveParseStarted)
		getLeoWsConn = oldGet
		IPPools.pool = oldPools
		receiveParseOnce = oldOnce
	}()

	IPPools.pool = make(map[string]chan IPGeoData)
	receiveParseOnce = &sync.Once{}

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

func waitForReceiveParseStart(started <-chan struct{}) {
	select {
	case <-started:
	case <-time.After(time.Second):
	}
}
