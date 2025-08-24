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

func ListenTCP(ctx context.Context, _ net.PacketConn, ipv int, srcip, dstip net.IP, onACK func(ack uint32, peer net.Addr, ackType int)) {
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

	// 过滤：只抓{ip/ip6}+tcp，来自 (dstip) → 本机 (srcip)
	filter := fmt.Sprintf(
		"%s and tcp and src host %s and dst host %s",
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

			// 从包中获取 TCP 层信息，并区分 RST+ACK / SYN+ACK
			if tl, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
				var ackType int
				if tl.ACK && tl.RST {
					ackType = 1 // 1=RST+ACK
				} else if tl.ACK && tl.SYN {
					ackType = 2 // 2=SYN+ACK
				} else {
					continue
				}

				// 提取对端 IP（按族别）
				var ip net.IP
				if ipv == 4 {
					if ip4, ok := packet.(*layers.IPv4); ok && ip4 != nil {
						ip = ip4.SrcIP
					}
				} else {
					if ip6, ok := packet.(*layers.IPv6); ok && ip6 != nil {
						ip = ip6.SrcIP
					}
				}
				peer := &net.IPAddr{IP: ip}
				if util.AddrIP(peer) == nil {
					continue
				}
				onACK(tl.Ack, peer, ackType)
			}
		}
	}
}
