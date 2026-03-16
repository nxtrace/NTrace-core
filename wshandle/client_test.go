package wshandle

import (
	"errors"
	"os"
	"testing"

	"github.com/gorilla/websocket"
)

func newStartedTestWsConn() *WsConn {
	c := newWsConn(nil, make(chan os.Signal, 1))
	c.setDoneChan(make(chan struct{}))
	c.setConnectionState(false, false)
	c.startLoop(c.keepAlive)
	c.startLoop(c.messageSendHandler)
	return c
}

func TestWsConnCloseStopsBackgroundLoops(t *testing.T) {
	conn := newStartedTestWsConn()
	doneCh := conn.getDoneChan()

	conn.Close()
	conn.Close()

	if !conn.isClosed() {
		t.Fatal("connection should be marked closed")
	}
	if err := conn.enqueueWrite(wsWriteJob{msgType: websocket.TextMessage, data: []byte("ping")}); !errors.Is(err, errWriteLoopStopped) {
		t.Fatalf("enqueueWrite error=%v, want %v", err, errWriteLoopStopped)
	}
	select {
	case <-doneCh:
	default:
		t.Fatal("done channel should be closed by Close")
	}
}

func TestNewClosesPreviousGlobalWsConn(t *testing.T) {
	oldCreateFn := createWsConnFn
	defer func() {
		createWsConnFn = oldCreateFn
	}()

	oldConn := newStartedTestWsConn()
	wsconnMu.Lock()
	wsconn = oldConn
	wsconnMu.Unlock()

	createWsConnFn = func() *WsConn {
		return newStartedTestWsConn()
	}

	newConn := New()
	defer newConn.Close()

	if newConn == oldConn {
		t.Fatal("New should replace the previous global connection")
	}
	if GetWsConn() != newConn {
		t.Fatal("GetWsConn should return the replacement connection")
	}
	if !oldConn.isClosed() {
		t.Fatal("previous global connection should be closed before replacement")
	}
	if err := oldConn.enqueueWrite(wsWriteJob{msgType: websocket.TextMessage, data: []byte("ping")}); !errors.Is(err, errWriteLoopStopped) {
		t.Fatalf("old enqueueWrite error=%v, want %v", err, errWriteLoopStopped)
	}
}
