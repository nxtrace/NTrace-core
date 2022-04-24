package listener_channel

import (
	"golang.org/x/net/context"
	"net"
	"time"
)

type ReceivedMessage struct {
	N    *int
	Peer net.Addr
	Msg  []byte
	Err  error
}

type ListenerChannel struct {
	ctx      context.Context
	cancel   context.CancelFunc
	Conn     net.PacketConn
	Messages chan ReceivedMessage
}

func New(conn net.PacketConn) *ListenerChannel {
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan ReceivedMessage, 50)

	return &ListenerChannel{Conn: conn, ctx: ctx, cancel: cancel, Messages: results}
}

func (l *ListenerChannel) Start() {
	for {
		select {
		case <-l.ctx.Done():
			return
		default:
		}

		reply := make([]byte, 1500)
		err := l.Conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err != nil {
			l.Messages <- ReceivedMessage{Err: err}
			continue
		}

		n, peer, err := l.Conn.ReadFrom(reply)
		if err != nil {
			l.Messages <- ReceivedMessage{Err: err}
			continue
		}
		l.Messages <- ReceivedMessage{
			N:    &n,
			Peer: peer,
			Err:  nil,
			Msg:  reply,
		}
	}
}

func (l *ListenerChannel) Stop() {
	l.cancel()
}
