//go:build windows && amd64

package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/windows"

	"github.com/nxtrace/NTrace-core/util"
)

type ICMPSpec struct {
	IPVersion    int
	ICMPMode     int
	EchoID       int
	SrcIP        net.IP
	DstIP        net.IP
	icmp         net.PacketConn
	icmp4        *ipv4.PacketConn
	icmp6        *ipv6.PacketConn
	hopLimitLock sync.Mutex
}

func ListenPacket(network string, laddr string) (net.PacketConn, error) {
	return net.ListenPacket(network, laddr)
}

func (s *ICMPSpec) Close() {
	_ = s.icmp.Close()
}

// isAdmin 判断当前进程是否具有管理员权限
func isAdmin() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer func() {
		_ = token.Close()
	}()

	type tokenElevation struct {
		TokenIsElevated uint32
	}
	var elev tokenElevation
	var outLen uint32

	if err := windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elev)),
		uint32(unsafe.Sizeof(elev)),
		&outLen,
	); err != nil {
		return false
	}
	return elev.TokenIsElevated != 0
}

// pcapAvailable 判断 Npcap 是否可用
func pcapAvailable() (bool, error) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
		return false, err
	}
	if len(devs) == 0 {
		return false, fmt.Errorf("no pcap devices found")
	}
	return true, nil
}

// resolveICMPMode 进行最终模式判定
func (s *ICMPSpec) resolveICMPMode() int {
	icmpMode := s.ICMPMode
	if icmpMode != 1 && icmpMode != 2 {
		icmpMode = 0 // 统一成 Auto
	}

	// 指定 1=Socket：直接返回
	if icmpMode == 1 {
		return 1
	}

	// Auto(0) 或强制 PCAP(2) → 尝试 PCAP
	if !isAdmin() {
		if icmpMode == 2 {
			log.Printf("PCAP mode requested, but administrator privilege is required; falling back to Socket mode.")
		}
		return 1
	}

	ok, err := pcapAvailable()
	if !ok {
		if icmpMode == 2 {
			log.Printf("PCAP mode requested, but Npcap is not available: %v; falling back to Socket mode.", err)
		}
		return 1
	}
	return 2
}

func (s *ICMPSpec) ListenICMP(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, seq int)) {
	switch s.resolveICMPMode() {
	case 1:
		s.listenICMPSock(ctx, ready, onICMP)
	case 2:
		s.listenICMPPcap(ctx, ready, onICMP)
	}
}

func (s *ICMPSpec) listenICMPPcap(ctx context.Context, ready chan struct{}, onICMP func(msg ReceivedMessage, finish time.Time, seq int)) {
	// 选择捕获设备与本地接口
	dev, err := util.PcapDeviceByIP(s.SrcIP)
	if err != nil {
		return
	}

	ipPrefix := "ip"
	proto := "icmp"
	if s.IPVersion == 6 {
		ipPrefix = "ip6"
		proto = "icmp6"
	}

	// 以“立即模式”打开 pcap，降低首包丢失概率
	handle, err := util.OpenLiveImmediate(dev, 65535, true, 4<<20)
	if err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenICMP) pcap open failed on %s: %v", dev, err))
		}
		log.Fatalf("(ListenICMP) pcap open failed on %s: %v", dev, err)
	}
	defer handle.Close()

	// 过滤：只抓 {ip/ip6} + {icmp/icmp6}，且目标为本机 s.SrcIP
	filter := fmt.Sprintf("%s and %s and dst host %s",
		ipPrefix, proto, s.SrcIP.String())

	if err := handle.SetBPFFilter(filter); err != nil {
		if util.EnvDevMode {
			panic(fmt.Errorf("(ListenICMP) set BPF failed: %v (filter=%q)", err, filter))
		}
		log.Fatalf("(ListenICMP) set BPF failed: %v (filter=%q)", err, filter)
	}

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	pktCh := src.Packets()
	close(ready)

	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-pktCh:
			if !ok {
				return
			}

			// 解包
			packet := pkt.NetworkLayer()
			if packet == nil {
				continue
			}
			finish := pkt.Metadata().Timestamp

			// outer = IP 头 + 负载
			outer := make([]byte, 0, len(packet.LayerContents())+len(packet.LayerPayload()))
			outer = append(outer, packet.LayerContents()...)
			outer = append(outer, packet.LayerPayload()...)

			var peerIP net.IP // 提取对端 IP（按族别）
			var data []byte   // 提取 ICMP 的负载
			if s.IPVersion == 4 {
				// 从包中获取 IPv4 层信息
				ip4, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
				if !ok || ip4 == nil {
					continue
				}
				peerIP = ip4.SrcIP

				// 从包中获取 ICMPv4 层信息
				ic4, ok := pkt.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4)
				if !ok || ic4 == nil {
					continue
				}
				data = ic4.Payload

				switch ic4.TypeCode.Type() {
				case layers.ICMPv4TypeEchoReply:
					if !peerIP.Equal(s.DstIP) {
						continue
					}

					id := int(ic4.Id)
					if id != s.EchoID {
						continue
					}
					peer := &net.IPAddr{IP: peerIP}

					// outer：外层 IP 报文字节
					msg := ReceivedMessage{
						Peer: peer,
						Msg:  outer,
					}
					seq := int(ic4.Seq)
					onICMP(msg, finish, seq)
					continue
				case layers.ICMPv4TypeTimeExceeded:
				case layers.ICMPv4TypeDestinationUnreachable:
				default:
					//log.Println("received icmp message of unknown type", ic4.TypeCode.Type())
					continue
				}

				if len(data) < 20 || data[0]>>4 != 4 {
					continue
				}

				dstIP := net.IP(data[16:20])
				if !dstIP.Equal(s.DstIP) {
					continue
				}
			} else {
				// 从包中获取 IPv6 层信息
				ip6, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
				if !ok || ip6 == nil {
					continue
				}
				peerIP = ip6.SrcIP

				// 从包中获取 ICMPv6 层信息
				ic6, ok := pkt.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
				if !ok || ic6 == nil {
					continue
				}
				data = ic6.Payload[4:]

				switch ic6.TypeCode.Type() {
				case layers.ICMPv6TypeEchoReply:
					// 从包中获取 ICMPv6 的 Echo 层信息
					echo, ok := pkt.Layer(layers.LayerTypeICMPv6Echo).(*layers.ICMPv6Echo)
					if !ok || echo == nil {
						continue
					}

					if !peerIP.Equal(s.DstIP) {
						continue
					}

					id := int(echo.Identifier)
					if id != s.EchoID {
						continue
					}
					peer := &net.IPAddr{IP: peerIP}

					// outer：外层 IP 报文字节
					msg := ReceivedMessage{
						Peer: peer,
						Msg:  outer,
					}
					seq := int(echo.SeqNumber)
					onICMP(msg, finish, seq)
					continue
				case layers.ICMPv6TypeTimeExceeded:
				case layers.ICMPv6TypePacketTooBig:
				case layers.ICMPv6TypeDestinationUnreachable:
				default:
					//log.Println("received icmp message of unknown type", ic6.TypeCode.Type())
					continue
				}

				if len(data) < 40 || data[0]>>4 != 6 {
					continue
				}

				dstIP := net.IP(data[24:40])
				if !dstIP.Equal(s.DstIP) {
					continue
				}
			}
			peer := &net.IPAddr{IP: peerIP}

			// outer：外层 IP 报文字节
			msg := ReceivedMessage{
				Peer: peer,
				Msg:  outer,
			}

			header, err := util.GetICMPResponsePayload(data)
			if err != nil {
				continue
			}

			id, err := util.GetICMPID(header)
			if err != nil || id != s.EchoID {
				continue
			}

			seq, err := util.GetICMPSeq(header)
			if err != nil {
				continue
			}
			onICMP(msg, finish, seq)
		}
	}
}

func (s *ICMPSpec) SendICMP(ctx context.Context, ipHdr gopacket.NetworkLayer, icmpHdr, icmpEcho gopacket.SerializableLayer, payload []byte) (time.Time, error) {
	select {
	case <-ctx.Done():
		return time.Time{}, context.Canceled
	default:
	}

	if s.IPVersion == 4 {
		ip4, ok := ipHdr.(*layers.IPv4)
		if !ok || ip4 == nil {
			return time.Time{}, errors.New("SendICMP: expect *layers.IPv4 when s.IPVersion==4")
		}
		ttl := int(ip4.TTL)

		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		// 序列化 ICMP 头与 payload 到缓冲区
		if err := gopacket.SerializeLayers(buf, opts, icmpHdr, gopacket.Payload(payload)); err != nil {
			return time.Time{}, err
		}

		// 串行设置 TTL + 发送，放在同一把锁里保证并发安全
		s.hopLimitLock.Lock()
		defer s.hopLimitLock.Unlock()

		if err := s.icmp4.SetTTL(ttl); err != nil {
			return time.Time{}, err
		}

		start := time.Now()

		if _, err := s.icmp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
			return time.Time{}, err
		}
		return start, nil
	}

	ip6, ok := ipHdr.(*layers.IPv6)
	if !ok || ip6 == nil {
		return time.Time{}, errors.New("SendICMP: expect *layers.IPv6 when s.IPVersion==6")
	}
	ttl := int(ip6.HopLimit)

	ic6, ok := icmpHdr.(*layers.ICMPv6)
	if !ok || ic6 == nil {
		return time.Time{}, errors.New("SendICMP: expect *layers.ICMPv6 when s.IPVersion==6")
	}

	_ = ic6.SetNetworkLayerForChecksum(ipHdr)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// 序列化 ICMP 头与 payload 到缓冲区
	if err := gopacket.SerializeLayers(buf, opts, icmpHdr, icmpEcho, gopacket.Payload(payload)); err != nil {
		return time.Time{}, err
	}

	// 串行设置 HopLimit + 发送，放在同一把锁里保证并发安全
	s.hopLimitLock.Lock()
	defer s.hopLimitLock.Unlock()

	if err := s.icmp6.SetHopLimit(ttl); err != nil {
		return time.Time{}, err
	}

	start := time.Now()

	if _, err := s.icmp.WriteTo(buf.Bytes(), &net.IPAddr{IP: s.DstIP}); err != nil {
		return time.Time{}, err
	}
	return start, nil
}
