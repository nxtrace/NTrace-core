package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/nxtrace/NTrace-core/internal/speedtest"
)

const (
	DefaultProvider     = "apple"
	DefaultMax          = "2G"
	DefaultTimeoutMs    = 10000
	DefaultThreads      = 4
	DefaultLatencyCount = 20
)

var ErrHelp = errors.New("help requested")

var allowedDotServers = map[string]bool{
	"":           true,
	"dnssb":      true,
	"aliyun":     true,
	"dnspod":     true,
	"google":     true,
	"cloudflare": true,
}

type Config struct {
	Provider       string
	Max            string
	MaxBytes       int64
	TimeoutMs      int
	Threads        int
	LatencyCount   int
	OutputJSON     bool
	NonInteractive bool
	EndpointIP     string
	NoMetadata     bool
	Language       string
	NoColor        bool
	DotServer      string
	SourceAddress  string
	SourceDevice   string
}

func Usage() string {
	return `Usage:
  nexttrace --speed [options]

Options:
  -h, --help                    Show this help message
  --speed-provider PROVIDER     Speed test backend: apple or cloudflare
  --max SIZE                    Per-thread transfer cap, e.g. 2G/500M/1GiB
  --timeout MS                  Per-thread timeout in milliseconds
  --threads N                   Concurrent workers for multi-thread rounds
  --latency-count N             Idle latency sample count
  --non-interactive             Disable endpoint prompt and auto-select
  --endpoint IP                 Force a specific endpoint IP and skip discovery
  --no-metadata                 Skip client/server metadata lookup
  --json                        Output a single JSON document to stdout
  -g, --language LANG           Output language: cn or en
  -C, --no-color                Disable colorful output
  --dot-server NAME             DoT server for endpoint discovery [dnssb, aliyun, dnspod, google, cloudflare]
  -s, --source IP               Use source address for outgoing HTTP connections
  -D, --dev NAME                Resolve the source address from the specified device

Examples:
  nexttrace --speed
  nexttrace --speed --speed-provider cloudflare --json
  nexttrace --speed --endpoint 1.2.3.4 --threads 8
`
}

func Load(args ...string) (*Config, error) {
	cleaned := stripSpeedFlag(args)
	fs := flag.NewFlagSet("speed", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := &Config{
		Provider:     DefaultProvider,
		Max:          DefaultMax,
		TimeoutMs:    DefaultTimeoutMs,
		Threads:      DefaultThreads,
		LatencyCount: DefaultLatencyCount,
		Language:     "cn",
	}

	help := false
	fs.BoolVar(&help, "h", false, "show help")
	fs.BoolVar(&help, "help", false, "show help")
	fs.StringVar(&cfg.Provider, "speed-provider", cfg.Provider, "speed provider")
	fs.StringVar(&cfg.Max, "max", cfg.Max, "per-thread transfer cap")
	fs.IntVar(&cfg.TimeoutMs, "timeout", cfg.TimeoutMs, "per-thread timeout in milliseconds")
	fs.IntVar(&cfg.Threads, "threads", cfg.Threads, "worker count")
	fs.IntVar(&cfg.LatencyCount, "latency-count", cfg.LatencyCount, "idle latency samples")
	fs.BoolVar(&cfg.NonInteractive, "non-interactive", false, "disable interactive endpoint selection")
	fs.StringVar(&cfg.EndpointIP, "endpoint", "", "force endpoint IP")
	fs.BoolVar(&cfg.NoMetadata, "no-metadata", false, "skip metadata lookup")
	fs.BoolVar(&cfg.OutputJSON, "json", false, "output JSON")
	fs.StringVar(&cfg.Language, "language", cfg.Language, "output language")
	fs.StringVar(&cfg.Language, "g", cfg.Language, "output language")
	fs.BoolVar(&cfg.NoColor, "no-color", false, "disable color")
	fs.BoolVar(&cfg.NoColor, "C", false, "disable color")
	fs.StringVar(&cfg.DotServer, "dot-server", "", "dot server")
	fs.StringVar(&cfg.SourceAddress, "source", "", "source address")
	fs.StringVar(&cfg.SourceAddress, "s", "", "source address")
	fs.StringVar(&cfg.SourceDevice, "dev", "", "source device")
	fs.StringVar(&cfg.SourceDevice, "D", "", "source device")

	if err := fs.Parse(cleaned); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, ErrHelp
		}
		return nil, err
	}
	if help {
		return nil, ErrHelp
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument(s): %s", strings.Join(fs.Args(), " "))
	}

	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	cfg.Language = speedtest.NormalizeLanguage(cfg.Language)
	cfg.DotServer = strings.ToLower(strings.TrimSpace(cfg.DotServer))
	cfg.SourceAddress = strings.TrimSpace(cfg.SourceAddress)
	cfg.SourceDevice = strings.TrimSpace(cfg.SourceDevice)

	switch cfg.Provider {
	case "apple", "cloudflare":
	default:
		return nil, fmt.Errorf("unsupported speed provider %q", cfg.Provider)
	}
	if cfg.TimeoutMs <= 0 {
		return nil, errors.New("--timeout must be > 0")
	}
	if cfg.TimeoutMs > 120000 {
		return nil, errors.New("--timeout must be <= 120000")
	}
	if cfg.Threads <= 0 {
		return nil, errors.New("--threads must be > 0")
	}
	if cfg.Threads > 64 {
		return nil, errors.New("--threads must be <= 64")
	}
	if cfg.LatencyCount <= 0 {
		return nil, errors.New("--latency-count must be > 0")
	}
	if cfg.LatencyCount > 100 {
		return nil, errors.New("--latency-count must be <= 100")
	}
	if !allowedDotServers[cfg.DotServer] {
		return nil, fmt.Errorf("unsupported --dot-server %q", cfg.DotServer)
	}
	if cfg.EndpointIP != "" && net.ParseIP(cfg.EndpointIP) == nil {
		return nil, fmt.Errorf("invalid endpoint IP %q", cfg.EndpointIP)
	}
	if cfg.SourceAddress != "" && cfg.SourceDevice != "" {
		return nil, errors.New("--source and --dev cannot be used together")
	}
	if cfg.SourceAddress != "" && net.ParseIP(cfg.SourceAddress) == nil {
		return nil, fmt.Errorf("invalid source IP %q", cfg.SourceAddress)
	}
	maxBytes, err := ParseSize(cfg.Max)
	if err != nil {
		return nil, fmt.Errorf("invalid --max %q: %w", cfg.Max, err)
	}
	if maxBytes <= 0 {
		return nil, errors.New("--max must be > 0")
	}
	cfg.MaxBytes = maxBytes

	return cfg, nil
}

func (c *Config) Summary() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s=%s  %s=%s  %s=%dms  %s=%d  %s=%d  json=%t  metadata=%t",
		speedtest.Text(c.Language, "provider", "后端"),
		c.Provider,
		speedtest.Text(c.Language, "max", "上限"),
		c.Max,
		speedtest.Text(c.Language, "timeout", "超时"),
		c.TimeoutMs,
		speedtest.Text(c.Language, "threads", "线程"),
		c.Threads,
		speedtest.Text(c.Language, "latency_count", "延迟采样"),
		c.LatencyCount,
		c.OutputJSON,
		!c.NoMetadata,
	)
}

var sizeRe = regexp.MustCompile(`(?i)^\s*([\d.]+)\s*([a-z]*)\s*$`)

func ParseSize(s string) (int64, error) {
	m := sizeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("cannot parse size %q", s)
	}
	num, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}
	if math.IsInf(num, 0) || math.IsNaN(num) {
		return 0, fmt.Errorf("size %q is out of range", s)
	}
	if num < 0 {
		return 0, fmt.Errorf("size must be non-negative")
	}
	unit := strings.ToLower(m[2])
	if unit == "" {
		return sizeBytes(num, 1, s)
	}
	var mul int64
	switch unit {
	case "k", "kb":
		mul = 1000
	case "m", "mb":
		mul = 1000 * 1000
	case "g", "gb":
		mul = 1000 * 1000 * 1000
	case "t", "tb":
		mul = 1000 * 1000 * 1000 * 1000
	case "kib":
		mul = 1 << 10
	case "mib":
		mul = 1 << 20
	case "gib":
		mul = 1 << 30
	case "tib":
		mul = 1 << 40
	default:
		return 0, fmt.Errorf("unknown unit %q", unit)
	}
	return sizeBytes(num, mul, s)
}

func sizeBytes(num float64, mul int64, input string) (int64, error) {
	bytes := num * float64(mul)
	if math.IsInf(bytes, 0) || math.IsNaN(bytes) || bytes >= float64(math.MaxInt64) {
		return 0, fmt.Errorf("size %q exceeds maximum supported value", input)
	}
	return int64(bytes), nil
}

func HumanBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GiB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KiB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func stripSpeedFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i, arg := range args {
		if arg == "--speed" || strings.HasPrefix(arg, "--speed=") {
			continue
		}
		out = append(out, arg)
		if arg == "--" {
			out = append(out, args[i+1:]...)
			break
		}
	}
	return out
}
