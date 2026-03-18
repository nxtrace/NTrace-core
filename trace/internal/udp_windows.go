//go:build windows && amd64

package internal

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	wd "github.com/xjasonlyu/windivert-go"

	"github.com/nxtrace/NTrace-core/util"
)

type UDPSpec struct {
	IPVersion    int
	ICMPMode     int
	SrcIP        net.IP
	DstIP        net.IP
	DstPort      int
	SourceDevice string
	icmp         net.PacketConn
	addr         wd.Address
	handle       wd.Handle
}

func (s *UDPSpec) InitUDP() {
	handle, err := wd.Open("false", wd.LayerNetwork, 0, 0)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(InitUDP) WinDivert open failed: %v", err))
		}
		log.Fatalf("(InitUDP) WinDivert open failed: %v", err)
	}
	s.handle = handle

	// 设置出站 Address
	s.addr.SetLayer(wd.LayerNetwork)
	s.addr.SetEvent(wd.EventNetworkPacket)
	s.addr.SetOutbound()
}

func (s *UDPSpec) Close() {
	_ = s.icmp.Close()
	_ = s.handle.Close()
}

func (s *UDPSpec) ListenOut(_ context.Context, _ chan struct{}, _ func(srcPort, seq, ttl int, start time.Time)) {
}

// resolveICMPMode 进行最终模式判定
func (s *UDPSpec) resolveICMPMode() int {
	icmpMode := s.ICMPMode
	if icmpMode != 1 && icmpMode != 2 {
		icmpMode = 0 // 统一成 Auto
	}

	// 指定 1=Socket：直接返回
	if icmpMode == 1 {
		return 1
	}

	// Auto(0) 或强制 Sniff(2) → 尝试 WinDivert
	ok, err := winDivertAvailable()
	if !ok {
		if icmpMode == 2 {
			log.Printf("请求使用 WinDivert 嗅探模式，但 WinDivert 不可用: %v；已回退到 Socket 模式。", err)
		}
		return 1
	}
	return 2
}

func (s *UDPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	switch s.resolveICMPMode() {
	case 1:
		s.listenICMPSock(ctx, ready, onICMP)
	case 2:
		s.listenICMPWinDivert(ctx, ready, onICMP)
	}
}

func (s *UDPSpec) listenICMPWinDivert(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, data []byte)) {
	sniffHandle, closeHandle := openWinDivertSniffHandle(ctx, winDivertICMPFilter(s.IPVersion, s.SrcIP), "ListenICMP")
	defer closeHandle()
	close(ready)

	buf := make([]byte, 65535)
	var addr wd.Address

	for {
		raw, finish, ok := receiveWinDivertPacket(ctx, sniffHandle, buf, &addr)
		if !ok {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		packet, ok := decodeWinDivertICMPPacket(s.IPVersion, raw)
		if !ok {
			continue
		}
		data, ok := packet.errorPayloadFor(s.DstIP)
		if !ok {
			continue
		}
		onICMP(packet.message(), finish, data)
	}
}

func (s *UDPSpec) SendUDP(ctx context.Context, ipHdr ipLayer, udpHdr *layers.UDP, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	if err := udpHdr.SetNetworkLayerForChecksum(ipHdr); err != nil {
		return time.Time{}, fmt.Errorf("SetNetworkLayerForChecksum: %w", err)
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 IP 与 UDP 头以及 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, ipHdr, udpHdr, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	// 复用预置的出站 Address
	if _, err := s.handle.Send(buf.Bytes(), &s.addr); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
