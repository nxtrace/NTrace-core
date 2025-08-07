package trace

import (
	"net"
	"time"

	"golang.org/x/net/context"
)

type ReceivedMessage struct {
	N    *int
	Peer net.Addr
	Msg  []byte
	Err  error
}

// PacketListener 负责监听网络数据包并通过通道传递接收到的消息
type PacketListener struct {
	ctx      context.Context
	Conn     net.PacketConn
	Messages chan ReceivedMessage
}

// NewPacketListener 创建一个新的数据包监听器
// conn: 用于接收数据包的连接
// ctx: 用于控制监听器生命周期的上下文
// 返回初始化好的 PacketListener 实例
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
