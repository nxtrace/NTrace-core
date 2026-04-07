package trace

import (
	"fmt"
	"net"
	"strings"

	"github.com/nxtrace/NTrace-core/util"
)

var (
	lookupSourceDeviceByName = net.InterfaceByName
	loadSourceDeviceAddrs    = func(dev *net.Interface) ([]net.Addr, error) { return dev.Addrs() }
)

func ResolveSourceDevice(device string) (*net.Interface, error) {
	trimmed := strings.TrimSpace(device)
	if trimmed == "" {
		return nil, nil
	}
	dev, err := lookupSourceDeviceByName(trimmed)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve source device %q: %w", trimmed, err)
	}
	if dev == nil {
		return nil, fmt.Errorf("source device %q not found", trimmed)
	}
	return dev, nil
}

func ResolveSourceDeviceAddr(dev *net.Interface, dstIP net.IP) string {
	if dev == nil || dstIP == nil {
		return ""
	}
	addrs, err := loadSourceDeviceAddrs(dev)
	if err != nil {
		return ""
	}
	var candidate string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if (ipNet.IP.To4() == nil) != (dstIP.To4() == nil) {
			continue
		}
		candidate = ipNet.IP.String()
		parsed := net.ParseIP(candidate)
		if parsed != nil && !(parsed.IsPrivate() ||
			parsed.IsLoopback() ||
			parsed.IsLinkLocalUnicast() ||
			parsed.IsLinkLocalMulticast()) {
			return candidate
		}
	}
	return candidate
}

func ResolveFallbackSrcAddr(dstIP net.IP) string {
	if dstIP == nil {
		return ""
	}
	if util.IsIPv6(dstIP) {
		resolved, _ := util.LocalIPPortv6(dstIP, nil, "udp6")
		if resolved != nil {
			return resolved.String()
		}
		return ""
	}
	resolved, _ := util.LocalIPPort(dstIP, nil, "udp")
	if resolved != nil {
		return resolved.String()
	}
	return ""
}

func ResolveConfiguredSrcAddr(dstIP net.IP, srcAddr, srcDev string) (resolved string, explicit bool, err error) {
	if trimmed := strings.TrimSpace(srcAddr); trimmed != "" {
		return trimmed, true, nil
	}
	dev, err := ResolveSourceDevice(srcDev)
	if err != nil {
		return "", false, err
	}
	if resolved := ResolveSourceDeviceAddr(dev, dstIP); resolved != "" {
		return resolved, false, nil
	}
	return ResolveFallbackSrcAddr(dstIP), false, nil
}

func NormalizeExplicitSourceConfig(method Method, config Config) (Config, string, error) {
	_ = method

	config.SrcAddr = strings.TrimSpace(config.SrcAddr)
	config.SourceDevice = strings.TrimSpace(config.SourceDevice)
	explicitSource := config.SrcAddr != ""

	if config.SrcAddr == "" && config.SourceDevice == "" {
		return config, "", nil
	}
	if config.SourceDevice == "" {
		return config, "", nil
	}
	if config.OSType == 2 && explicitSource {
		config.SourceDevice = ""
		return config, "", nil
	}

	dev, err := ResolveSourceDevice(config.SourceDevice)
	if err != nil {
		return config, "", err
	}
	if !explicitSource {
		resolved := ResolveSourceDeviceAddr(dev, config.DstIP)
		if resolved == "" {
			return config, "", fmt.Errorf("source device %q has no usable %s address", config.SourceDevice, sourceFamilyLabel(config.DstIP))
		}
		config.SrcAddr = resolved
	}

	if config.OSType == 2 {
		config.SourceDevice = ""
		return config, "", nil
	}
	return config, "", nil
}

func sourceFamilyLabel(dstIP net.IP) string {
	if util.IsIPv6(dstIP) {
		return "IPv6"
	}
	return "IPv4"
}
