package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/akamensky/argparse"
)

func TestRegisterNaliFlagWithAvailabilityEnabledAddsHelpEntry(t *testing.T) {
	parser := argparse.NewParser("nexttrace", "")
	registerNaliFlagWithAvailability(parser, true)

	usage := parser.Usage(nil)
	if !strings.Contains(usage, "--nali") {
		t.Fatalf("usage missing --nali:\n%s", usage)
	}
}

func TestRegisterNaliFlagWithAvailabilityDisabledDoesNotAcceptNali(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	registerNaliFlagWithAvailability(parser, false)

	if err := parser.Parse([]string{"ntr", "--nali"}); err == nil {
		t.Fatal("Parse() error = nil, want unknown flag when nali mode unavailable")
	}
}

func TestNaliFlavorAvailabilityConstant(t *testing.T) {
	switch appBinName {
	case "nexttrace":
		if !enableNali {
			t.Fatal("full nexttrace should enable --nali")
		}
	case "nexttrace-tiny", "ntr":
		if enableNali {
			t.Fatalf("%s should not enable --nali", appBinName)
		}
	default:
		t.Fatalf("unexpected appBinName %q", appBinName)
	}
}

func TestNaliParserDoesNotRequireTarget(t *testing.T) {
	parser := argparse.NewParser("nexttrace", "")
	naliMode := registerNaliFlagWithAvailability(parser, true)
	parser.StringPositional(&argparse.Options{Help: "target"})

	if err := parser.Parse([]string{"nexttrace", "--nali"}); err != nil {
		t.Fatalf("Parse(--nali) error = %v", err)
	}
	if !*naliMode {
		t.Fatal("naliMode = false, want true")
	}
}

func TestValidateNaliModeOptions(t *testing.T) {
	tests := []struct {
		name string
		opts naliModeOptions
		want string
	}{
		{name: "ok", opts: naliModeOptions{}, want: ""},
		{name: "json", opts: naliModeOptions{json: true}, want: "--nali 不支持 --json"},
		{name: "dual family", opts: naliModeOptions{ipv4Only: true, ipv6Only: true}, want: "-4/--ipv4 不能与 -6/--ipv6 同时使用"},
		{name: "mtu", opts: naliModeOptions{mtu: true}, want: "--nali 不能与 --mtu 同时使用"},
		{name: "mtr", opts: naliModeOptions{mtr: true}, want: "--nali 不能与 --mtr/-r/--report/-w/--wide 同时使用"},
		{name: "output", opts: naliModeOptions{output: true}, want: "--nali 不能与 --output 同时使用"},
		{name: "probe", opts: naliModeOptions{queries: true}, want: "--nali 不能与 --queries 同时使用"},
		{name: "source", opts: naliModeOptions{sourceDevice: true}, want: "--nali 不能与 --dev 同时使用"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNaliModeOptions(tt.opts)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("validateNaliModeOptions() error = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("validateNaliModeOptions() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildNaliModeOptionsDetectsExplicitDefaults(t *testing.T) {
	parser := argparse.NewParser("nexttrace", "")
	parser.Int("q", "queries", &argparse.Options{Default: 3})
	parser.Int("m", "max-hops", &argparse.Options{Default: 30})
	parser.Int("", "timeout", &argparse.Options{Default: 1000})
	if err := parser.Parse([]string{"nexttrace", "-q", "3", "--max-hops", "30"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	opts := buildNaliModeOptions(
		parser,
		false, false, false, false, false,
		effectiveMTRModes{},
		false, false, false, false,
		"", false, false, false,
		"", false, "", false, "", false, false, false, false,
		"", 0, "",
	)
	if !opts.queries || !opts.maxHops {
		t.Fatalf("explicit probe flags not detected: %+v", opts)
	}
	if opts.port || opts.packetSize {
		t.Fatalf("unexpected explicit flags: %+v", opts)
	}
}

func TestRunNaliModeTargetWithDisableGeoIPKeepsOriginal(t *testing.T) {
	var out bytes.Buffer
	err := runNaliMode(t.Context(), naliRunOptions{
		stdout:    &out,
		data:      "disable-geoip",
		pow:       "api.nxtrace.org",
		lang:      "en",
		timeoutMs: 100,
		target:    "A 8.8.8.8",
	})
	if err != nil {
		t.Fatalf("runNaliMode() error = %v", err)
	}
	if got, want := out.String(), "A 8.8.8.8\n"; got != want {
		t.Fatalf("runNaliMode() output = %q, want %q", got, want)
	}
}

func TestRunNaliModeReadsStdinWithoutTarget(t *testing.T) {
	var out bytes.Buffer
	err := runNaliMode(t.Context(), naliRunOptions{
		stdin:     strings.NewReader("A 192.0.2.1\n"),
		stdout:    &out,
		data:      "disable-geoip",
		pow:       "api.nxtrace.org",
		lang:      "en",
		timeoutMs: 100,
	})
	if err != nil {
		t.Fatalf("runNaliMode(stdin) error = %v", err)
	}
	if got, want := out.String(), "A 192.0.2.1 [RFC5737]\n"; got != want {
		t.Fatalf("runNaliMode(stdin) output = %q, want %q", got, want)
	}
}
