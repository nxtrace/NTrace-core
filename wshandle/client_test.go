package wshandle

// Tests in this file mutate package-level websocket globals; do not add
// t.Parallel without isolating that state first.

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nxtrace/NTrace-core/util"
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
	select {
	case _, ok := <-conn.MsgReceiveCh:
		if ok {
			t.Fatal("receive channel should be closed by Close")
		}
	default:
		t.Fatal("receive channel should be closed after Close returns")
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

	createWsConnFn = func(context.Context) *WsConn {
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
	createWsConnFn = func(context.Context) *WsConn {
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

func TestCreateWsConnHonorsCanceledContextDuringFastIP(t *testing.T) {
	oldFastIPFn := wsGetFastIPFn
	defer func() { wsGetFastIPFn = oldFastIPFn }()

	started := make(chan struct{})
	wsGetFastIPFn = func(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *WsConn, 1)
	go func() {
		done <- createWsConn(ctx)
	}()

	<-started
	cancel()

	select {
	case conn := <-done:
		if conn == nil {
			t.Fatal("createWsConn returned nil")
		}
		defer conn.Close()
		if conn.IsConnected() {
			t.Fatal("connection should not be connected after canceled startup")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("createWsConn did not return promptly after cancel")
	}
}

func TestCreateWsConnAsyncReturnsBeforeFastIP(t *testing.T) {
	oldFastIPFn := wsGetFastIPFn
	defer func() { wsGetFastIPFn = oldFastIPFn }()

	fastIPStarted := make(chan struct{}, 1)
	fastIPDone := make(chan struct{}, 1)
	releaseFastIP := make(chan struct{})
	defer close(releaseFastIP)
	wsGetFastIPFn = func(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
		select {
		case fastIPStarted <- struct{}{}:
		default:
		}
		defer func() {
			select {
			case fastIPDone <- struct{}{}:
			default:
			}
		}()
		select {
		case <-releaseFastIP:
			return "127.0.0.1", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	returned := make(chan *WsConn, 1)
	go func() {
		returned <- createWsConnAsync(context.Background())
	}()

	select {
	case <-fastIPStarted:
	case <-time.After(time.Second):
		t.Fatal("FastIP probe did not start")
	}

	select {
	case conn := <-returned:
		if conn == nil {
			t.Fatal("createWsConnAsync returned nil")
		}
		defer conn.Close()
	case <-fastIPDone:
		t.Fatal("createWsConnAsync waited for FastIP to complete")
	case <-time.After(time.Second):
		t.Fatal("createWsConnAsync did not return while FastIP was blocked")
	}
}

func TestWaitUntilConnectedReturnsOnConnect(t *testing.T) {
	conn := newStartedTestWsConn()
	defer conn.Close()

	done := make(chan error, 1)
	go func() {
		done <- conn.WaitUntilConnected(context.Background())
	}()

	conn.setConnectionState(true, false)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitUntilConnected returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitUntilConnected did not observe connected state")
	}
}

func TestWaitUntilConnectedHonorsContext(t *testing.T) {
	conn := newStartedTestWsConn()
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := conn.WaitUntilConnected(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitUntilConnected error = %v, want %v", err, context.DeadlineExceeded)
	}
}

func TestRecreateWsConnCloseCancelsFastIP(t *testing.T) {
	oldFastIPFn := wsGetFastIPFn
	defer func() {
		wsGetFastIPFn = oldFastIPFn
	}()

	started := make(chan struct{})
	wsGetFastIPFn = func(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}

	conn := newStartedTestWsConn()
	conn.apiHost = "example.com"
	conn.apiPort = "443"
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		conn.recreateWsConn()
		close(done)
	}()

	<-started
	conn.Close()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("recreateWsConn did not stop promptly after Close")
	}
}

func TestRecreateWsConnCanceledBaseContextSkipsReconnect(t *testing.T) {
	oldFastIPFn := wsGetFastIPFn
	defer func() {
		wsGetFastIPFn = oldFastIPFn
	}()

	var fastIPCalls int32
	wsGetFastIPFn = func(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
		atomic.AddInt32(&fastIPCalls, 1)
		return "127.0.0.1", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	conn := newWsConn(nil, make(chan os.Signal, 1))
	conn.baseCtx = ctx
	conn.apiHost = "example.com"
	conn.apiPort = "443"
	conn.setConnectionState(false, true)
	defer conn.Close()

	conn.recreateWsConn()

	if got := atomic.LoadInt32(&fastIPCalls); got != 0 {
		t.Fatalf("FastIP refresh calls = %d, want 0 after base context cancel", got)
	}
	if conn.IsConnecting() {
		t.Fatal("connection should not remain connecting after base context cancel")
	}
}

func TestKeepAliveStopsOnCanceledBaseContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	conn := newWsConn(nil, make(chan os.Signal, 1))
	conn.baseCtx = ctx
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		conn.keepAlive()
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("keepAlive did not stop after base context cancel")
	}
}

func TestCreateWsConnAsyncPreservesDirectIPOnReconnect(t *testing.T) {
	saveAndRestoreGlobalWsConn(t)

	oldFastIPFn := wsGetFastIPFn
	oldEnvHostPort := util.EnvHostPort
	oldEnvToken := envToken
	t.Cleanup(func() {
		wsGetFastIPFn = oldFastIPFn
		util.EnvHostPort = oldEnvHostPort
		envToken = oldEnvToken
	})

	var fastIPCalls int32
	wsGetFastIPFn = func(ctx context.Context, domain string, port string, enableOutput bool) (string, error) {
		atomic.AddInt32(&fastIPCalls, 1)
		return "", errors.New("unexpected FastIP refresh")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	directPort := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	_ = listener.Close()
	util.EnvHostPort = "127.0.0.1:" + directPort
	envToken = "token"

	ctx, cancel := context.WithCancel(context.Background())

	conn := createWsConnAsync(ctx)

	if !conn.directIP {
		t.Fatal("async websocket should preserve direct-IP state")
	}
	if conn.apiFastIP != "127.0.0.1" {
		t.Fatalf("apiFastIP = %q, want direct IP preserved", conn.apiFastIP)
	}
	cancel()
	conn.Close()

	// Keep the reconnect context live and use a closed local port so
	// recreateWsConn reaches the direct-IP dial path without external network.
	reconnectCtx, reconnectCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer reconnectCancel()
	if err := reconnectCtx.Err(); err != nil {
		t.Fatalf("reconnect context should be live before recreateWsConn: %v", err)
	}
	reconnectConn := newWsConn(nil, make(chan os.Signal, 1))
	reconnectConn.baseCtx = reconnectCtx
	reconnectConn.directIP = true
	reconnectConn.apiHost = "api.nxtrace.org"
	reconnectConn.apiPort = directPort
	reconnectConn.apiFastIP = "127.0.0.1"
	defer reconnectConn.Close()

	reconnectConn.recreateWsConn()

	if got := atomic.LoadInt32(&fastIPCalls); got != 0 {
		t.Fatalf("FastIP refresh calls = %d, want 0 for direct-IP reconnect", got)
	}
}
