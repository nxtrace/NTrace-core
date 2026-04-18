package ipgeo

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/wshandle"
)

func TestLeoIPWaitsForConnectionBeforeSending(t *testing.T) {
	oldGet := getLeoWsConn
	oldPools := IPPools.pool
	oldOnce := receiveParseOnce
	defer func() {
		getLeoWsConn = oldGet
		IPPools.pool = oldPools
		receiveParseOnce = oldOnce
	}()

	IPPools.pool = make(map[string]chan IPGeoData)
	receiveParseOnce = sync.Once{}

	conn := &wshandle.WsConn{
		MsgSendCh:    make(chan string, 1),
		MsgReceiveCh: make(chan string, 1),
		Interrupt:    make(chan os.Signal, 1),
	}
	getLeoWsConn = func() *wshandle.WsConn { return conn }

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

	conn.Connected = true

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
