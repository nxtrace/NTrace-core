package trace

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestResolveSourceDeviceAddrSupportsIPAddr(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPAddr{IP: net.ParseIP("2001:db8::10")}}, nil
	})
	defer restore()

	dev, err := ResolveSourceDevice("en7")
	if err != nil {
		t.Fatalf("ResolveSourceDevice() error = %v", err)
	}
	got, err := ResolveSourceDeviceAddr(dev, net.ParseIP("2606:4700:4700::1111"))
	if err != nil {
		t.Fatalf("ResolveSourceDeviceAddr() error = %v", err)
	}
	if got != "2001:db8::10" {
		t.Fatalf("ResolveSourceDeviceAddr() = %q, want 2001:db8::10", got)
	}
}

func TestResolveSourceDeviceAddrPrefersPrivateOverLinkLocalFallback(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.ParseIP("fd00::10"), Mask: net.CIDRMask(64, 128)},
			&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		}, nil
	})
	defer restore()

	dev, err := ResolveSourceDevice("en7")
	if err != nil {
		t.Fatalf("ResolveSourceDevice() error = %v", err)
	}
	got, err := ResolveSourceDeviceAddr(dev, net.ParseIP("2606:4700:4700::1111"))
	if err != nil {
		t.Fatalf("ResolveSourceDeviceAddr() error = %v", err)
	}
	if got != "fd00::10" {
		t.Fatalf("ResolveSourceDeviceAddr() = %q, want fd00::10", got)
	}
}

func TestResolveSourceDeviceAddrFallsBackToLoopbackBeforeLinkLocal(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
			&net.IPNet{IP: net.ParseIP("169.254.0.1"), Mask: net.CIDRMask(16, 32)},
		}, nil
	})
	defer restore()

	dev, err := ResolveSourceDevice("lo0")
	if err != nil {
		t.Fatalf("ResolveSourceDevice() error = %v", err)
	}
	got, err := ResolveSourceDeviceAddr(dev, net.ParseIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("ResolveSourceDeviceAddr() error = %v", err)
	}
	if got != "127.0.0.1" {
		t.Fatalf("ResolveSourceDeviceAddr() = %q, want 127.0.0.1", got)
	}
}

func TestNormalizeExplicitSourceConfigNoopWithoutOverrides(t *testing.T) {
	cfg := Config{
		OSType: 1,
		DstIP:  net.ParseIP("1.1.1.1"),
	}

	got, err := NormalizeExplicitSourceConfig(TCPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != cfg.SrcAddr || got.SourceDevice != cfg.SourceDevice || got.OSType != cfg.OSType {
		t.Fatalf("config changed without overrides: got %#v want %#v", got, cfg)
	}
	if !got.DstIP.Equal(cfg.DstIP) {
		t.Fatalf("DstIP changed without overrides: got %v want %v", got.DstIP, cfg.DstIP)
	}
}

func TestNormalizeExplicitSourceConfigPrefersExplicitSource(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("198.51.100.10"), Mask: net.CIDRMask(24, 32)}}, nil
	})
	defer restore()
	restoreGOOS := stubCurrentGOOS(t, "darwin")
	defer restoreGOOS()

	cfg := Config{
		OSType:       1,
		DstIP:        net.ParseIP("1.1.1.1"),
		SrcAddr:      "192.0.2.10",
		SourceDevice: "en7",
	}

	got, err := NormalizeExplicitSourceConfig(TCPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "192.0.2.10" {
		t.Fatalf("SrcAddr = %q, want explicit source", got.SrcAddr)
	}
	if got.SourceDevice != "en7" {
		t.Fatalf("SourceDevice = %q, want en7", got.SourceDevice)
	}
}

func TestNormalizeExplicitSourceConfigResolvesSourceFromDevice(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
			&net.IPNet{IP: net.ParseIP("203.0.113.8"), Mask: net.CIDRMask(24, 32)},
		}, nil
	})
	defer restore()
	restoreGOOS := stubCurrentGOOS(t, "linux")
	defer restoreGOOS()

	cfg := Config{
		OSType:       3,
		DstIP:        net.ParseIP("1.1.1.1"),
		SourceDevice: "eth0",
	}

	got, err := NormalizeExplicitSourceConfig(UDPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "203.0.113.8" {
		t.Fatalf("SrcAddr = %q, want 203.0.113.8", got.SrcAddr)
	}
	if got.SourceDevice != "eth0" {
		t.Fatalf("SourceDevice = %q, want eth0", got.SourceDevice)
	}
}

func TestNormalizeExplicitSourceConfigClearsSourceDeviceOnUnsupportedUnix(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("203.0.113.8"), Mask: net.CIDRMask(24, 32)}}, nil
	})
	defer restore()
	restoreGOOS := stubCurrentGOOS(t, "freebsd")
	defer restoreGOOS()

	cfg := Config{
		OSType:       3,
		DstIP:        net.ParseIP("1.1.1.1"),
		SourceDevice: "em0",
	}

	got, err := NormalizeExplicitSourceConfig(UDPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "203.0.113.8" {
		t.Fatalf("SrcAddr = %q, want 203.0.113.8", got.SrcAddr)
	}
	if got.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty on unsupported unix", got.SourceDevice)
	}
}

func TestNormalizeExplicitSourceConfigRejectsDeviceWithoutMatchingFamily(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("203.0.113.8"), Mask: net.CIDRMask(24, 32)}}, nil
	})
	defer restore()
	restoreGOOS := stubCurrentGOOS(t, "linux")
	defer restoreGOOS()

	cfg := Config{
		OSType:       3,
		DstIP:        net.ParseIP("2001:4860:4860::8888"),
		SourceDevice: "eth0",
	}

	_, err := NormalizeExplicitSourceConfig(UDPTrace, cfg)
	if err == nil || err.Error() != `source device "eth0" has no usable IPv6 address` {
		t.Fatalf("err = %v, want IPv6 family error", err)
	}
}

func TestResolveConfiguredSrcAddrPropagatesSourceDeviceAddrLoadError(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return nil, fmt.Errorf("boom")
	})
	defer restore()

	_, _, err := ResolveConfiguredSrcAddr(net.ParseIP("1.1.1.1"), "", "eth0")
	if err == nil {
		t.Fatal("ResolveConfiguredSrcAddr() error = nil, want propagated addrs error")
	}
	if err.Error() != `load source device "eth0" addresses: boom` {
		t.Fatalf("err = %q, want propagated addrs error", err.Error())
	}
}

func TestNormalizeExplicitSourceConfigPropagatesSourceDeviceAddrLoadError(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return nil, fmt.Errorf("boom")
	})
	defer restore()
	restoreGOOS := stubCurrentGOOS(t, "linux")
	defer restoreGOOS()

	cfg := Config{
		OSType:       3,
		DstIP:        net.ParseIP("1.1.1.1"),
		SourceDevice: "eth0",
	}

	_, err := NormalizeExplicitSourceConfig(UDPTrace, cfg)
	if err == nil {
		t.Fatal("NormalizeExplicitSourceConfig() error = nil, want propagated addrs error")
	}
	if err.Error() != `load source device "eth0" addresses: boom` {
		t.Fatalf("err = %q, want propagated addrs error", err.Error())
	}
}

func TestNormalizeExplicitSourceConfigWindowsClearsDeviceForICMP(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("192.0.2.20"), Mask: net.CIDRMask(24, 32)}}, nil
	})
	defer restore()

	cfg := Config{
		OSType:       osTypeWindows,
		DstIP:        net.ParseIP("1.1.1.1"),
		SourceDevice: "Ethernet0",
	}

	got, err := NormalizeExplicitSourceConfig(ICMPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "192.0.2.20" {
		t.Fatalf("SrcAddr = %q, want 192.0.2.20", got.SrcAddr)
	}
	if got.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty", got.SourceDevice)
	}
}

func TestNormalizeExplicitSourceConfigWindowsIgnoresDeviceWhenSourceExplicitForICMP(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		t.Fatalf("ResolveSourceDevice should not be called when Windows explicit source is set")
		return nil, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return nil, nil
	})
	defer restore()

	cfg := Config{
		OSType:       osTypeWindows,
		DstIP:        net.ParseIP("1.1.1.1"),
		SrcAddr:      "192.0.2.30",
		SourceDevice: "Ethernet0",
	}

	got, err := NormalizeExplicitSourceConfig(ICMPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "192.0.2.30" {
		t.Fatalf("SrcAddr = %q, want 192.0.2.30", got.SrcAddr)
	}
	if got.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty", got.SourceDevice)
	}
}

func TestNormalizeExplicitSourceConfigWindowsTCPResolvesDeviceToSourceAddress(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		if device != "Ethernet0" {
			t.Fatalf("ResolveSourceDevice device = %q, want Ethernet0", device)
		}
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("192.0.2.44"), Mask: net.CIDRMask(24, 32)}}, nil
	})
	defer restore()

	cfg := Config{
		OSType:       osTypeWindows,
		DstIP:        net.ParseIP("1.1.1.1"),
		SourceDevice: "Ethernet0",
	}

	got, err := NormalizeExplicitSourceConfig(TCPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "192.0.2.44" {
		t.Fatalf("SrcAddr = %q, want 192.0.2.44", got.SrcAddr)
	}
	if got.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty", got.SourceDevice)
	}
}

func TestResolveSourceDeviceNotFoundWithoutNilError(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return nil, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return nil, nil
	})
	defer restore()

	_, err := ResolveSourceDevice("missing0")
	if err == nil {
		t.Fatal("ResolveSourceDevice() error = nil, want not-found error")
	}
	if err.Error() != `source device "missing0" not found` {
		t.Fatalf("err = %q, want clean not-found error", err.Error())
	}
	if strings.Contains(err.Error(), "<nil>") {
		t.Fatalf("err = %q, should not contain <nil>", err.Error())
	}
}

func stubSourceDeviceResolver(t *testing.T, lookup func(string) (*net.Interface, error), addrs func(*net.Interface) ([]net.Addr, error)) func() {
	t.Helper()
	prevLookup := lookupSourceDeviceByName
	prevAddrs := loadSourceDeviceAddrs
	lookupSourceDeviceByName = lookup
	loadSourceDeviceAddrs = addrs
	return func() {
		lookupSourceDeviceByName = prevLookup
		loadSourceDeviceAddrs = prevAddrs
	}
}

func stubCurrentGOOS(t *testing.T, goos string) func() {
	t.Helper()
	prev := currentGOOS
	currentGOOS = goos
	return func() {
		currentGOOS = prev
	}
}
