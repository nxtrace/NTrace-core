package server

import (
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeWSConn struct {
	mu            sync.Mutex
	writes        []wsEnvelope
	writeStarted  chan struct{}
	writeBlock    chan struct{}
	closeOnce     sync.Once
	closeCount    int
	controlCount  int
	deadlineCount int
}

func newFakeWSConn(blockWrites bool) *fakeWSConn {
	conn := &fakeWSConn{}
	if blockWrites {
		conn.writeStarted = make(chan struct{})
		conn.writeBlock = make(chan struct{})
	}
	return conn
}

func (f *fakeWSConn) WriteJSON(v interface{}) error {
	if f.writeStarted != nil {
		select {
		case <-f.writeStarted:
		default:
			close(f.writeStarted)
		}
	}
	if f.writeBlock != nil {
		<-f.writeBlock
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var msg wsEnvelope
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	f.mu.Lock()
	f.writes = append(f.writes, msg)
	f.mu.Unlock()
	return nil
}

func (f *fakeWSConn) SetWriteDeadline(time.Time) error {
	f.mu.Lock()
	f.deadlineCount++
	f.mu.Unlock()
	return nil
}

func (f *fakeWSConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	f.mu.Lock()
	f.controlCount++
	f.mu.Unlock()
	return nil
}

func (f *fakeWSConn) Close() error {
	f.closeOnce.Do(func() {
		f.mu.Lock()
		f.closeCount++
		f.mu.Unlock()
		if f.writeBlock != nil {
			close(f.writeBlock)
		}
	})
	return nil
}

func (f *fakeWSConn) NextReader() (messageType int, r io.Reader, err error) {
	return 0, nil, io.EOF
}

type fakeWSInitConn struct {
	deadlines []time.Time
	readLimit int64
	message   []byte
	err       error
}

func (f *fakeWSInitConn) SetReadDeadline(t time.Time) error {
	f.deadlines = append(f.deadlines, t)
	return nil
}

func (f *fakeWSInitConn) SetReadLimit(limit int64) {
	f.readLimit = limit
}

func (f *fakeWSInitConn) ReadMessage() (messageType int, p []byte, err error) {
	if f.err != nil {
		return 0, nil, f.err
	}
	return websocket.TextMessage, f.message, nil
}

func TestReadWSInitMessage_ClearsDeadlineAfterSuccessfulRead(t *testing.T) {
	conn := &fakeWSInitConn{message: []byte(`{"target":"example.com"}`)}

	msg, err := readWSInitMessage(conn)
	if err != nil {
		t.Fatalf("readWSInitMessage returned error: %v", err)
	}
	if string(msg) != `{"target":"example.com"}` {
		t.Fatalf("readWSInitMessage()=%q, want payload unchanged", string(msg))
	}
	if conn.readLimit != maxWSInitMessageBytes {
		t.Fatalf("SetReadLimit=%d, want %d", conn.readLimit, maxWSInitMessageBytes)
	}
	if len(conn.deadlines) != 2 {
		t.Fatalf("SetReadDeadline called %d times, want 2", len(conn.deadlines))
	}
	if conn.deadlines[0].IsZero() {
		t.Fatal("initial read deadline should be set")
	}
	if !conn.deadlines[1].IsZero() {
		t.Fatalf("final read deadline=%v, want zero time", conn.deadlines[1])
	}
}

func TestWSTraceSessionSend_QueueOverflowReturnsErrSlowConsumer(t *testing.T) {
	conn := newFakeWSConn(true)
	session := newWSTraceSession(conn, "cn", 1)
	defer session.finish()

	if err := session.send(wsEnvelope{Type: "first"}); err != nil {
		t.Fatalf("first send returned error: %v", err)
	}
	<-conn.writeStarted

	if err := session.send(wsEnvelope{Type: "second"}); err != nil {
		t.Fatalf("second send returned error: %v", err)
	}

	err := session.send(wsEnvelope{Type: "third"})
	if !errors.Is(err, errWSSlowConsumer) {
		t.Fatalf("expected errWSSlowConsumer, got %v", err)
	}
	if !session.closed.Load() {
		t.Fatal("session should be marked closed after queue overflow")
	}
}

func TestWSTraceSessionWriter_PreservesEnvelopeOrder(t *testing.T) {
	conn := newFakeWSConn(false)
	session := newWSTraceSession(conn, "cn", 4)

	if err := session.send(wsEnvelope{Type: "start"}); err != nil {
		t.Fatalf("first send returned error: %v", err)
	}
	if err := session.send(wsEnvelope{Type: "mtr_raw", Data: map[string]int{"ttl": 1}}); err != nil {
		t.Fatalf("second send returned error: %v", err)
	}

	session.finish()

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.writes) != 2 {
		t.Fatalf("writer sent %d envelopes, want 2", len(conn.writes))
	}
	if conn.writes[0].Type != "start" || conn.writes[1].Type != "mtr_raw" {
		t.Fatalf("unexpected write order: %+v", conn.writes)
	}
}

func TestWSTraceSessionClose_IsIdempotent(t *testing.T) {
	conn := newFakeWSConn(false)
	session := newWSTraceSession(conn, "cn", 4)

	session.closeWithCode(websocket.CloseTryAgainLater, "slow consumer")
	session.closeWithCode(websocket.CloseTryAgainLater, "slow consumer")
	session.finish()
	session.finish()

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.closeCount != 1 {
		t.Fatalf("Close called %d times, want 1", conn.closeCount)
	}
	if conn.controlCount != 1 {
		t.Fatalf("WriteControl called %d times, want 1", conn.controlCount)
	}
	if conn.deadlineCount != 0 {
		t.Fatalf("SetWriteDeadline called %d times during close path, want 0", conn.deadlineCount)
	}
}

func TestSanitizeLogParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"hello\nworld", "hello\\nworld"},
		{"hello\r\nworld", "hello\\n\\nworld"},
		{"line1\nline2\nline3", "line1\\nline2\\nline3"},
		{"tab\there", "tab\there"},
		{"null\x00byte", "null\uFFFDbyte"},
		{"esc\x1b[31m", "esc\uFFFD[31m"},
		{"", ""},
		{"safe-host.example.com", "safe-host.example.com"},
		{"evil\n[deploy] fake log entry", "evil\\n[deploy] fake log entry"},
	}
	for _, tt := range tests {
		got := sanitizeLogParam(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLogParam(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
