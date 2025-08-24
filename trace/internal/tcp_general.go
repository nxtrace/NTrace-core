//go:build !darwin

package internal

import (
	"context"
	"errors"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/nxtrace/NTrace-core/util"
)

func ListenTCP(ctx context.Context, tcp net.PacketConn, ipv int, _, dstip net.IP, onACK func(ack uint32, peer net.Addr, ackType int)) {
	if tcp == nil {
		return
	}

	go func() {
		<-ctx.Done()
		_ = tcp.Close()
	}()

	buf := make([]byte, 4096)

	for {
		n, peer, err := tcp.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			continue
		}
		if n == 0 {
			continue
		}

		// 仅处理来自目标 IP 的 TCP 包
		if ip := util.AddrIP(peer); ip == nil || !ip.Equal(dstip) {
			continue
		}

		// 拷贝出精确长度，避免 buf 复用带来的数据竞争/覆盖
		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		// 解包
		packet := gopacket.NewPacket(pkt, layers.LayerTypeTCP, gopacket.Default)
		if packet.ErrorLayer() != nil {
			continue
		}

		// 从包中获取 TCP 层信息，并区分 RST+ACK / SYN+ACK
		if tl, ok := packet.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
			var ackType int
			if tl.ACK && tl.RST {
				ackType = 1 // 1=RST+ACK
			} else if tl.ACK && tl.SYN {
				ackType = 2 // 2=SYN+ACK
			} else {
				continue
			}
			onACK(tl.Ack, peer, ackType)
		}
	}
}
