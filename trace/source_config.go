package trace

import (
	"fmt"
	"net"
	"runtime"
	"strings"

	"github.com/nxtrace/NTrace-core/util"
)

var (
	lookupSourceDeviceByName = net.InterfaceByName
	loadSourceDeviceAddrs    = func(dev *net.Interface) ([]net.Addr, error) { return dev.Addrs() }
	currentGOOS              = runtime.GOOS
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

func ResolveSourceDeviceAddr(dev *net.Interface, dstIP net.IP) (string, error) {
	if dev == nil || dstIP == nil {
		return "", nil
	}
	addrs, err := loadSourceDeviceAddrs(dev)
	if err != nil {
		return "", fmt.Errorf("load source device %q addresses: %w", dev.Name, err)
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
			return candidate, nil
		}
	}
	return candidate, nil
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
	resolved, err = ResolveSourceDeviceAddr(dev, dstIP)
	if err != nil {
		return "", false, err
	}
	if resolved != "" {
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
		resolved, err := ResolveSourceDeviceAddr(dev, config.DstIP)
		if err != nil {
			return config, "", err
		}
		if resolved == "" {
			return config, "", fmt.Errorf("source device %q has no usable %s address", config.SourceDevice, sourceFamilyLabel(config.DstIP))
		}
		config.SrcAddr = resolved
	}

	if config.OSType == 2 {
		config.SourceDevice = ""
		return config, "", nil
	}
	if !supportsSourceDeviceBinding(currentGOOS) {
		config.SourceDevice = ""
	}
	return config, "", nil
}

func sourceFamilyLabel(dstIP net.IP) string {
	if util.IsIPv6(dstIP) {
		return "IPv6"
	}
	return "IPv4"
}

func supportsSourceDeviceBinding(goos string) bool {
	switch goos {
	case "darwin", "linux":
		return true
	default:
		return false
	}
}
