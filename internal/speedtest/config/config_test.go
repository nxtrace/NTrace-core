package config

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"1024", 1024},
		{"2G", 2_000_000_000},
		{"500M", 500_000_000},
		{"1GiB", 1 << 30},
		{"1MiB", 1 << 20},
		{"1KiB", 1 << 10},
	}
	for _, tt := range tests {
		got, err := ParseSize(tt.input)
		if err != nil {
			t.Fatalf("ParseSize(%q) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseSizeRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{"", "abc", "-1G", "1XB", "9223372036854775808", "9000000000TB"} {
		if _, err := ParseSize(input); err == nil {
			t.Fatalf("ParseSize(%q) error = nil, want non-nil", input)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != DefaultProvider {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, DefaultProvider)
	}
	if cfg.TimeoutMs != DefaultTimeoutMs {
		t.Fatalf("TimeoutMs = %d, want %d", cfg.TimeoutMs, DefaultTimeoutMs)
	}
	if cfg.Threads != DefaultThreads {
		t.Fatalf("Threads = %d, want %d", cfg.Threads, DefaultThreads)
	}
	if cfg.Max != DefaultMax {
		t.Fatalf("Max = %q, want %q", cfg.Max, DefaultMax)
	}
	if cfg.LatencyCount != DefaultLatencyCount {
		t.Fatalf("LatencyCount = %d, want %d", cfg.LatencyCount, DefaultLatencyCount)
	}
}

func TestLoadStripsSpeedFlagAndParsesArgs(t *testing.T) {
	cfg, err := Load("--speed", "--speed-provider", "cloudflare", "--timeout", "1200", "--threads", "8", "--latency-count", "7", "--endpoint", "1.1.1.1", "--language", "en", "--json", "--source", "192.0.2.10", "--dot-server", "aliyun")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "cloudflare" {
		t.Fatalf("Provider = %q, want cloudflare", cfg.Provider)
	}
	if cfg.TimeoutMs != 1200 {
		t.Fatalf("TimeoutMs = %d, want 1200", cfg.TimeoutMs)
	}
	if cfg.Threads != 8 {
		t.Fatalf("Threads = %d, want 8", cfg.Threads)
	}
	if cfg.LatencyCount != 7 {
		t.Fatalf("LatencyCount = %d, want 7", cfg.LatencyCount)
	}
	if cfg.EndpointIP != "1.1.1.1" {
		t.Fatalf("EndpointIP = %q, want 1.1.1.1", cfg.EndpointIP)
	}
	if cfg.Language != "en" {
		t.Fatalf("Language = %q, want en", cfg.Language)
	}
	if !cfg.OutputJSON {
		t.Fatal("OutputJSON = false, want true")
	}
	if cfg.SourceAddress != "192.0.2.10" {
		t.Fatalf("SourceAddress = %q, want 192.0.2.10", cfg.SourceAddress)
	}
	if cfg.DotServer != "aliyun" {
		t.Fatalf("DotServer = %q, want aliyun", cfg.DotServer)
	}
}

func TestLoadParsesSourceDevice(t *testing.T) {
	cfg, err := Load("--speed", "--dev", "eth0")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SourceDevice != "eth0" {
		t.Fatalf("SourceDevice = %q, want eth0", cfg.SourceDevice)
	}
}

func TestLoadRejectsSourceAndDeviceTogether(t *testing.T) {
	_, err := Load("--speed", "--source", "192.0.2.10", "--dev", "eth0")
	if err == nil || !strings.Contains(err.Error(), "--source and --dev") {
		t.Fatalf("Load() error = %v, want source/dev conflict", err)
	}
}

func TestLoadStripsAssignedSpeedFlag(t *testing.T) {
	cfg, err := Load("--speed=true", "--threads", "2")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Threads != 2 {
		t.Fatalf("Threads = %d, want 2", cfg.Threads)
	}
}

func TestLoadRejectsUnexpectedArgs(t *testing.T) {
	_, err := Load("--speed", "1.1.1.1")
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("Load() error = %v, want unexpected argument", err)
	}
}

func TestLoadRejectsInvalidEndpoint(t *testing.T) {
	_, err := Load("--endpoint", "bad-ip")
	if err == nil || !strings.Contains(err.Error(), "invalid endpoint IP") {
		t.Fatalf("Load() error = %v, want invalid endpoint IP", err)
	}
}

func TestLoadRejectsInvalidDotServer(t *testing.T) {
	_, err := Load("--dot-server", "bad")
	if err == nil || !strings.Contains(err.Error(), "unsupported --dot-server") {
		t.Fatalf("Load() error = %v, want dot-server validation", err)
	}
}

func TestLoadReturnsHelp(t *testing.T) {
	_, err := Load("--help")
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("Load() error = %v, want ErrHelp", err)
	}
}

func TestUsageMentionsSupportedFlags(t *testing.T) {
	usage := Usage()
	for _, want := range []string{"--speed-provider", "--max", "--timeout", "--threads", "--latency-count", "--endpoint", "--no-metadata"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("Usage() missing %q:\n%s", want, usage)
		}
	}
	if strings.Contains(usage, "--dl-url") {
		t.Fatalf("Usage() should not mention unsupported URL override flags:\n%s", usage)
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{1024, "1 KiB"},
		{1 << 20, "1.0 MiB"},
		{1 << 30, "1.00 GiB"},
	}
	for _, tt := range tests {
		if got := HumanBytes(tt.input); got != tt.want {
			t.Fatalf("HumanBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
