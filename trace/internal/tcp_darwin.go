//go:build darwin

package internal

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/nxtrace/NTrace-core/util"
)

func ListenTCP(ctx context.Context, _ net.PacketConn, ipv int, srcip, dstip net.IP, onACK func(ack uint32, peer net.Addr)) {
	dev := util.FindDeviceByIP(srcip)
	if dev == "" {
		dev = "en0"
	}

	ipPrefix := "ip"
	if ipv == 6 {
		ipPrefix = "ip6"
	}

	handle, err := pcap.OpenLive(dev, 65535, true, pcap.BlockForever)
	if err != nil {
		log.Printf("pcap open failed on %s: %v", dev, err)
		return
	}
	defer handle.Close()

	// 过滤：只抓{ip/ip6}+tcp，来自 (dstip) → 本机 (srcip)，且 RST 或 SYN+ACK
	filter := fmt.Sprintf(
		"%s and tcp and src host %s and dst host %s and (tcp[13] & 0x04 != 0 or tcp[13] & 0x12 == 0x12)",
		ipPrefix, dstip.String(), srcip.String(),
	)
	if err := handle.SetBPFFilter(filter); err != nil {
		log.Printf("pcap set filter failed: %v", err)
		return
	}

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	pktCh := src.Packets()

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

			// 从包中获取TCP layer信息
			if tl := pkt.Layer(layers.LayerTypeTCP); tl != nil {
				tcpL := tl.(*layers.TCP)
				switch ipv {
				case 4:
					if ip4, _ := packet.(*layers.IPv4); ip4 != nil {
						onACK(tcpL.Ack, &net.IPAddr{IP: ip4.SrcIP})
					} else {
						continue
					}
				case 6:
					if ip6, _ := packet.(*layers.IPv6); ip6 != nil {
						onACK(tcpL.Ack, &net.IPAddr{IP: ip6.SrcIP})
					} else {
						continue
					}
				default:
					continue
				}
			}
		}
	}
}
