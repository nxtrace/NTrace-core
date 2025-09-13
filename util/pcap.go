//go:build darwin || (windows && amd64)

package util

import (
	"fmt"
	"net"
	"sync"

	"github.com/google/gopacket/pcap"
)

var (
	DevCache sync.Map // key: string(srcip) -> string(pcap device name)
)

func ipKey(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}

// PcapDeviceByIP 返回可用于 pcap.OpenLive 的设备名
func PcapDeviceByIP(srcip net.IP) (string, error) {
	key := ipKey(srcip)
	if v, ok := DevCache.Load(key); ok {
		return v.(string), nil
	}

	devs, err := pcap.FindAllDevs()
	if err != nil {
		return "", fmt.Errorf("pcap list devices: %w", err)
	}

	is6 := IsIPv6(srcip)
	var v4, v6 net.IP
	if is6 {
		v6 = srcip.To16()
	} else {
		v4 = srcip.To4()
	}

	// 按 IP 精确匹配
	for _, d := range devs {
		for _, a := range d.Addresses {
			var got net.IP
			got = a.IP
			if got == nil {
				continue
			}

			if is6 {
				if g := got.To16(); g != nil && IsIPv6(got) && g.Equal(v6) {
					DevCache.Store(key, d.Name)
					return d.Name, nil
				}
			} else {
				if g := got.To4(); g != nil && g.Equal(v4) {
					DevCache.Store(key, d.Name)
					return d.Name, nil
				}
			}
		}
	}
	return "", fmt.Errorf("pcap device for IP %s not found", srcip)
}

// OpenLiveImmediate 打开一个启用“立即模式”的 pcap 句柄
func OpenLiveImmediate(dev string, snaplen int, promisc bool, bufferSize int) (*pcap.Handle, error) {
	// 创建一个未激活的句柄
	ih, err := pcap.NewInactiveHandle(dev)
	if err != nil {
		return nil, err
	}

	defer func() {
		ih.CleanUp()
	}()

	if snaplen <= 0 {
		snaplen = 65535
	}

	// 设置每个包的最大抓取长度
	if err := ih.SetSnapLen(snaplen); err != nil {
		return nil, err
	}

	// 设置超时模式为 BlockForever，阻塞等待每包数据
	if err := ih.SetTimeout(pcap.BlockForever); err != nil {
		return nil, err
	}

	// 开启“立即模式”
	if err := ih.SetImmediateMode(true); err != nil {
		return nil, err
	}

	// 开启“混杂模式”，以抓更多帧
	if err := ih.SetPromisc(promisc); err != nil {
		return nil, err
	}

	// 设置内核缓冲区大小
	if bufferSize > 0 {
		_ = ih.SetBufferSize(bufferSize)
	}

	// 激活：获得可读写的数据包句柄
	h, err := ih.Activate()
	if err != nil {
		return nil, err
	}
	return h, nil
}
