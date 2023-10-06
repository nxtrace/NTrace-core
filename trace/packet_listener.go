package trace

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

type PacketListener struct {
	ctx      context.Context
	Conn     net.PacketConn
	Messages chan ReceivedMessage
}

func NewPacketListener(conn net.PacketConn, ctx context.Context) *PacketListener {
	results := make(chan ReceivedMessage, 50)

	return &PacketListener{Conn: conn, ctx: ctx, Messages: results}
}

func (l *PacketListener) Start() {
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
