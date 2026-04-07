package trace

import (
	"net"
	"strings"
	"testing"
)

func TestNormalizeExplicitSourceConfigNoopWithoutOverrides(t *testing.T) {
	cfg := Config{
		OSType: 1,
		DstIP:  net.ParseIP("1.1.1.1"),
	}

	got, warning, err := NormalizeExplicitSourceConfig(TCPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
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

	cfg := Config{
		OSType:       1,
		DstIP:        net.ParseIP("1.1.1.1"),
		SrcAddr:      "192.0.2.10",
		SourceDevice: "en7",
	}

	got, warning, err := NormalizeExplicitSourceConfig(TCPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
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

	cfg := Config{
		OSType:       3,
		DstIP:        net.ParseIP("1.1.1.1"),
		SourceDevice: "eth0",
	}

	got, warning, err := NormalizeExplicitSourceConfig(UDPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
	if got.SrcAddr != "203.0.113.8" {
		t.Fatalf("SrcAddr = %q, want 203.0.113.8", got.SrcAddr)
	}
	if got.SourceDevice != "eth0" {
		t.Fatalf("SourceDevice = %q, want eth0", got.SourceDevice)
	}
}

func TestNormalizeExplicitSourceConfigRejectsDeviceWithoutMatchingFamily(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("203.0.113.8"), Mask: net.CIDRMask(24, 32)}}, nil
	})
	defer restore()

	cfg := Config{
		OSType:       3,
		DstIP:        net.ParseIP("2001:4860:4860::8888"),
		SourceDevice: "eth0",
	}

	_, _, err := NormalizeExplicitSourceConfig(UDPTrace, cfg)
	if err == nil || err.Error() != `source device "eth0" has no usable IPv6 address` {
		t.Fatalf("err = %v, want IPv6 family error", err)
	}
}

func TestNormalizeExplicitSourceConfigWindowsWarnsAndClearsDevice(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		return &net.Interface{Name: device}, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("192.0.2.20"), Mask: net.CIDRMask(24, 32)}}, nil
	})
	defer restore()

	cfg := Config{
		OSType:       2,
		DstIP:        net.ParseIP("1.1.1.1"),
		SourceDevice: "Ethernet0",
	}

	got, warning, err := NormalizeExplicitSourceConfig(ICMPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "192.0.2.20" {
		t.Fatalf("SrcAddr = %q, want 192.0.2.20", got.SrcAddr)
	}
	if got.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty", got.SourceDevice)
	}
	wantWarning := "Windows 当前不支持按 --dev 绑定真实出接口；已使用该设备地址作为 source address，实际出口仍由系统路由决定"
	if warning != wantWarning {
		t.Fatalf("warning = %q, want %q", warning, wantWarning)
	}
}

func TestNormalizeExplicitSourceConfigWindowsIgnoresDeviceWhenSourceExplicit(t *testing.T) {
	restore := stubSourceDeviceResolver(t, func(device string) (*net.Interface, error) {
		t.Fatalf("ResolveSourceDevice should not be called when Windows explicit source is set")
		return nil, nil
	}, func(_ *net.Interface) ([]net.Addr, error) {
		return nil, nil
	})
	defer restore()

	cfg := Config{
		OSType:       2,
		DstIP:        net.ParseIP("1.1.1.1"),
		SrcAddr:      "192.0.2.30",
		SourceDevice: "Ethernet0",
	}

	got, warning, err := NormalizeExplicitSourceConfig(TCPTrace, cfg)
	if err != nil {
		t.Fatalf("NormalizeExplicitSourceConfig() error = %v", err)
	}
	if got.SrcAddr != "192.0.2.30" {
		t.Fatalf("SrcAddr = %q, want 192.0.2.30", got.SrcAddr)
	}
	if got.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty", got.SourceDevice)
	}
	wantWarning := "Windows 当前不支持按 --dev 绑定真实出接口；已忽略 --dev，继续使用 --source"
	if warning != wantWarning {
		t.Fatalf("warning = %q, want %q", warning, wantWarning)
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
