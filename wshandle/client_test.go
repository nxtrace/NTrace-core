package wshandle

import (
	"errors"
	"os"
	"testing"
	"time"

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

func saveAndRestoreGlobalWsConn(t *testing.T) {
	t.Helper()

	wsconnMu.RLock()
	oldWsconn := wsconn
	wsconnMu.RUnlock()

	t.Cleanup(func() {
		wsconnMu.Lock()
		current := wsconn
		wsconn = oldWsconn
		wsconnMu.Unlock()
		if current != nil && current != oldWsconn {
			current.Close()
		}
	})
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
	saveAndRestoreGlobalWsConn(t)

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

func TestGetWsConnDoesNotBlockWhileNewClosesPreviousConn(t *testing.T) {
	oldCreateFn := createWsConnFn
	defer func() {
		createWsConnFn = oldCreateFn
	}()
	saveAndRestoreGlobalWsConn(t)

	release := make(chan struct{})
	oldConn := newWsConn(nil, make(chan os.Signal, 1))
	oldConn.setDoneChan(make(chan struct{}))
	oldConn.startLoop(func() {
		<-release
	})

	wsconnMu.Lock()
	wsconn = oldConn
	wsconnMu.Unlock()

	newConn := newStartedTestWsConn()
	defer newConn.Close()
	createWsConnFn = func() *WsConn {
		return newConn
	}

	newResult := make(chan *WsConn, 1)
	go func() {
		newResult <- New()
	}()

	select {
	case <-oldConn.closeCh:
	case <-time.After(time.Second):
		t.Fatal("New did not start closing the previous connection")
	}

	getResult := make(chan *WsConn, 1)
	go func() {
		getResult <- GetWsConn()
	}()

	select {
	case got := <-getResult:
		if got != newConn {
			t.Fatalf("GetWsConn returned %p, want %p", got, newConn)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("GetWsConn blocked while New was waiting for old Close")
	}

	close(release)

	select {
	case got := <-newResult:
		if got != newConn {
			t.Fatalf("New returned %p, want %p", got, newConn)
		}
	case <-time.After(time.Second):
		t.Fatal("New did not finish after releasing old Close")
	}
}

func TestSendQueuedMessageDoesNotBlockWhenDisconnectedAndReceiveQueueIsUnavailable(t *testing.T) {
	conn := newWsConn(nil, make(chan os.Signal, 1))
	defer conn.Close()
	conn.MsgReceiveCh = make(chan string)

	done := make(chan struct{})
	go func() {
		conn.sendQueuedMessage("1.1.1.1")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("sendQueuedMessage blocked while disconnected")
	}
}

func TestSendQueuedMessageDoesNotBlockWhenEnqueueWriteFails(t *testing.T) {
	conn := newWsConn(nil, make(chan os.Signal, 1))
	defer conn.Close()
	conn.MsgReceiveCh = make(chan string)
	conn.setConnectionState(true, false)

	conn.lifecycleMu.Lock()
	conn.closed = true
	conn.lifecycleMu.Unlock()

	done := make(chan struct{})
	go func() {
		conn.sendQueuedMessage("1.1.1.1")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("sendQueuedMessage blocked after enqueueWrite failure")
	}
}

func TestMessageReceiveHandlerCloseRaceDoesNotPanic(t *testing.T) {
	for i := 0; i < 50; i++ {
		conn := newWsConn(nil, make(chan os.Signal, 1))
		conn.setDoneChan(make(chan struct{}))
		conn.setConnectionState(false, false)

		started := make(chan struct{})
		conn.startLoop(func() {
			close(started)
			conn.messageReceiveHandler()
		})

		<-started

		done := make(chan struct{})
		go func() {
			conn.Close()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Close hung while messageReceiveHandler was exiting")
		}
	}
}
