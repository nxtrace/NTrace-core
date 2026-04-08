package trace

import (
	"testing"

	"github.com/nxtrace/NTrace-core/trace/internal"
)

func TestApplyICMPSourceDevicePropagatesOnNonWindows(t *testing.T) {
	restoreGOOS := stubCurrentGOOS(t, "darwin")
	defer restoreGOOS()

	spec := internal.NewICMPSpec(4, 0, 1, nil, nil)
	applyICMPSourceDevice(spec, 1, "en0")
	if spec.SourceDevice != "en0" {
		t.Fatalf("SourceDevice = %q, want en0 for macOS ICMP", spec.SourceDevice)
	}

	restoreGOOS = stubCurrentGOOS(t, "linux")
	defer restoreGOOS()
	spec = internal.NewICMPSpec(4, 0, 1, nil, nil)
	applyICMPSourceDevice(spec, 3, "eth0")
	if spec.SourceDevice != "eth0" {
		t.Fatalf("SourceDevice = %q, want eth0 for Unix ICMP", spec.SourceDevice)
	}
}

func TestApplyICMPSourceDeviceSkipsWindows(t *testing.T) {
	spec := internal.NewICMPSpec(6, 0, 1, nil, nil)
	applyICMPSourceDevice(spec, 2, "Ethernet0")
	if spec.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty on Windows", spec.SourceDevice)
	}
}

func TestApplyICMPSourceDeviceSkipsUnsupportedUnix(t *testing.T) {
	restoreGOOS := stubCurrentGOOS(t, "freebsd")
	defer restoreGOOS()

	spec := internal.NewICMPSpec(4, 0, 1, nil, nil)
	applyICMPSourceDevice(spec, 3, "em0")
	if spec.SourceDevice != "" {
		t.Fatalf("SourceDevice = %q, want empty on unsupported Unix", spec.SourceDevice)
	}
}
