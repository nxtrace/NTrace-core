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

func ListenTCP(ctx context.Context, tcp net.PacketConn, ipv int, _, dstip net.IP, onACK func(ack uint32, peer net.Addr)) {
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
		var packet gopacket.Packet
		if ipv == 4 {
			packet = gopacket.NewPacket(pkt, layers.LayerTypeIPv4, gopacket.Default)
		} else {
			packet = gopacket.NewPacket(pkt, layers.LayerTypeIPv6, gopacket.Default)
		}
		if errLayer := packet.ErrorLayer(); errLayer != nil {
			// 回退：某些内核缓冲可能直接从 TCP 头开始
			packet = gopacket.NewPacket(pkt, layers.LayerTypeTCP, gopacket.Default)
			if packet.ErrorLayer() != nil {
				continue
			}
		}

		// 从包中获取TCP layer信息
		if tl := packet.Layer(layers.LayerTypeTCP); tl != nil {
			tcpL := tl.(*layers.TCP)
			if !(tcpL.RST || (tcpL.SYN && tcpL.ACK)) {
				continue
			}
			onACK(tcpL.Ack, peer)
		}
	}
}
