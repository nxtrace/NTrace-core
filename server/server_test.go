package server

import (
	"net"
	"testing"
)

func TestListenHTTPReturnsActualBoundAddr(t *testing.T) {
	listener, err := listenHTTP("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listenHTTP() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener addr = %T, want *net.TCPAddr", listener.Addr())
	}
	if addr.Port == 0 {
		t.Fatal("listener port = 0, want non-zero bound port")
	}
}
