package trace

import (
	"context"
	"errors"
	"net"
	"time"
)

type ReceivedMessage struct {
	N    *int
	Peer net.Addr
	Msg  []byte
	Err  error
}

// PacketListener 负责监听网络数据包并通过通道传递接收到的消息
// 对外暴露只读的 Messages，避免外部代码误写
type PacketListener struct {
	Conn     net.PacketConn
	Messages <-chan ReceivedMessage
	ch       chan ReceivedMessage
}

// NewPacketListener 创建一个新的数据包监听器
// conn: 用于接收数据包的连接
// 返回初始化好的 PacketListener 实例
func NewPacketListener(conn net.PacketConn) *PacketListener {
	ch := make(chan ReceivedMessage, 64)

	return &PacketListener{Conn: conn, Messages: ch, ch: ch}
}

func (l *PacketListener) Start(ctx context.Context) {
	defer close(l.ch)

	go func() {
		<-ctx.Done()
		_ = l.Conn.Close()
	}()

	buf := make([]byte, 4096)

	for {
		n, peer, err := l.Conn.ReadFrom(buf)
		if err != nil {
			// 连接关闭或 ctx 取消：直接退出
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}

			// 限时等待投递错误；超时或取消就丢弃/退出
			select {
			case l.ch <- ReceivedMessage{Err: err}:
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		if n == 0 {
			continue
		}

		// 拷贝出精确长度，避免 buf 复用带来的数据竞争
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		nn := new(int)
		*nn = n

		// 限时等待投递数据；超时或取消就丢弃/退出
		select {
		case l.ch <- ReceivedMessage{N: nn, Peer: peer, Msg: pkt}:
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
