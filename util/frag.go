package util

import (
	"errors"
	"net"

	"golang.org/x/net/ipv4"
)

type IPv4Fragment struct {
	Hdr  ipv4.Header
	Body []byte
}

// GetMTUByIP 根据给定 IPv4/IPv6 源地址返回所属网卡 MTU，找不到返回 0
func GetMTUByIP(srcIP net.IP) int {
	// 若已指定网卡名，直接取该网卡的 MTU
	if SrcDev != "" {
		if ifi, err := net.InterfaceByName(SrcDev); err == nil && ifi != nil {
			return ifi.MTU
		}
	}

	is6 := IsIPv6(srcIP)

	var v4, v6 net.IP
	if is6 {
		v6 = srcIP.To16()
	} else {
		v4 = srcIP.To4()
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return 0
	}
	for _, ifi := range ifaces {
		addrs, _ := ifi.Addrs()
		for _, a := range addrs {
			var got net.IP
			switch v := a.(type) {
			case *net.IPNet:
				got = v.IP
			case *net.IPAddr:
				got = v.IP
			default:
				continue
			}
			if got == nil {
				continue
			}
			if is6 {
				if g := got.To16(); g != nil && IsIPv6(got) && g.Equal(v6) {
					return ifi.MTU
				}
			} else {
				if g := got.To4(); g != nil && g.Equal(v4) {
					return ifi.MTU
				}
			}
		}
	}
	return 0
}

// IPv4Fragmentize 将 base（IPv4 头）与 body（IPv4 负载：传输层头+数据）按 mtu 进行 IP 层分片
func IPv4Fragmentize(base *ipv4.Header, body []byte, mtu int) ([]IPv4Fragment, error) {
	// 低 13 位为分片偏移（单位 8 字节）
	const ipOffsetMask = 0x1FFF

	// 提取 IPv4 头长度 ihl
	ihl := base.Len

	// MTU 至少要容纳一个完整 IPv4 头
	if mtu <= ihl {
		return nil, errors.New("IPv4Fragmentize: MTU too small (<= IHL)")
	}
	maxFragBody := mtu - ihl

	// 假如已经置位 DF=1，则直接报错并返回 nil
	if (base.Flags & ipv4.DontFragment) != 0 {
		return nil, errors.New("IPv4Fragmentize: DF set while fragmentation required")
	}

	// 非最后片的片内负载长度按 8 字节对齐
	aligned := (maxFragBody / 8) * 8

	// 预分配结果切片的容量
	capacity := (len(body) + aligned - 1) / aligned
	frags := make([]IPv4Fragment, 0, capacity)

	// 按 aligned 切出所有的分片（最后片承载所有剩余字节）
	for off := 0; off < len(body); {
		more := off+aligned < len(body)
		fragLen := len(body) - off
		if more {
			fragLen = aligned
		}

		// 为每片拷贝出独立头部
		h := *base
		h.Len = ihl
		h.TotalLen = ihl + fragLen

		// 先清除已有的 MF 标志
		h.Flags &^= ipv4.MoreFragments

		// 写入分片偏移（仅低 13 位，单位 8 字节）
		h.FragOff &^= ipOffsetMask
		h.FragOff |= (off / 8) & ipOffsetMask

		// 非最后片，则置位 MF=1 到 Flags 字段中
		if more {
			h.Flags |= ipv4.MoreFragments
		}

		// 置 0 使 Marshal 重算 IPv4 头校验和
		h.Checksum = 0

		frags = append(frags, IPv4Fragment{
			Hdr:  h,
			Body: body[off : off+fragLen],
		})
		off += fragLen
	}
	return frags, nil
}
