package cmd

import (
	"net"
	"strings"
	"testing"

	"github.com/akamensky/argparse"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/tracelog"
	"github.com/nxtrace/NTrace-core/util"
)

func TestRegisterGlobalpingFlagWithAvailability_DisabledStillParses(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	from := registerGlobalpingFlagWithAvailability(parser, false)

	if err := parser.Parse([]string{"ntr", "--from", "tokyo"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got := strings.TrimSpace(*from); got != "tokyo" {
		t.Fatalf("--from = %q, want tokyo", got)
	}
}

func TestRegisterWebUIFlagsWithAvailability_DisabledStillParses(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	flags := registerWebUIFlagsWithAvailability(parser, false)

	if err := parser.Parse([]string{"ntr", "--deploy", "--listen", "127.0.0.1:1080"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !*flags.deploy {
		t.Fatal("--deploy should parse as true")
	}
	if got := strings.TrimSpace(*flags.deployListen); got != "127.0.0.1:1080" {
		t.Fatalf("--listen = %q, want 127.0.0.1:1080", got)
	}
}

func TestRegisterTTLIntervalFlagWithMTRSupport_HelpOmitsTracerouteDefault(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	registerTTLIntervalFlagWithMTRSupport(parser, true)

	usage := parser.Usage(nil)
	if strings.Contains(usage, "Default: 300") {
		t.Fatalf("usage should not advertise traceroute default in MTR mode:\n%s", usage)
	}
}

func TestApplyTTLIntervalDefault(t *testing.T) {
	ttlInterval := 0
	applyTTLIntervalDefault(&ttlInterval, false, false)
	if ttlInterval != defaultTracerouteTTLIntervalMs {
		t.Fatalf("ttlInterval = %d, want %d", ttlInterval, defaultTracerouteTTLIntervalMs)
	}

	ttlInterval = 0
	applyTTLIntervalDefault(&ttlInterval, false, true)
	if ttlInterval != 0 {
		t.Fatalf("MTR ttlInterval = %d, want 0", ttlInterval)
	}

	ttlInterval = 0
	applyTTLIntervalDefault(&ttlInterval, true, false)
	if ttlInterval != 0 {
		t.Fatalf("explicit ttlInterval = %d, want 0", ttlInterval)
	}
}

func TestAdvancedHelpTextMentionsTuningGuidance(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	registerPacketIntervalFlag(parser)
	parser.Int("", "max-attempts", &argparse.Options{Help: buildMaxAttemptsHelp()})
	parser.Int("", "parallel-requests", &argparse.Options{Default: 18, Help: buildParallelRequestsHelp()})
	parser.Int("", "timeout", &argparse.Options{Default: 1000, Help: buildTimeoutHelp()})
	parser.Int("", "psize", &argparse.Options{Help: buildPayloadSizeHelp()})

	usage := parser.Usage(nil)
	for _, want := range []string{
		"load-balanced paths",
		"rate-limited links",
		"intercontinental",
		"raise for MTU or",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing tuning guidance %q:\n%s", want, usage)
		}
	}
}

func TestProbeOptionHelpMentionsRandomPacketSizeAndTOS(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	parser.Int("", "psize", &argparse.Options{Help: buildPayloadSizeHelp()})
	parser.Int("Q", "tos", &argparse.Options{Default: 0, Help: buildTOSHelp()})

	usage := parser.Usage(nil)
	for _, want := range []string{
		"Negative values randomize each probe",
		"type-of-service / traffic class",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q:\n%s", want, usage)
		}
	}
}

func TestDetectExplicitProbeFlags(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	parser.Int("q", "queries", &argparse.Options{Default: 3})
	parser.Int("i", "ttl-time", &argparse.Options{Default: 300})
	parser.Int("", "psize", &argparse.Options{})
	parser.Int("Q", "tos", &argparse.Options{Default: 0})

	if err := parser.Parse([]string{"ntr", "--psize", "-123", "-Q", "46", "-q", "5"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	queriesExplicit, ttlTimeExplicit, packetSizeExplicit, tosExplicit := detectExplicitProbeFlags(parser)
	if !queriesExplicit {
		t.Fatal("queriesExplicit = false, want true")
	}
	if ttlTimeExplicit {
		t.Fatal("ttlTimeExplicit = true, want false")
	}
	if !packetSizeExplicit {
		t.Fatal("packetSizeExplicit = false, want true")
	}
	if !tosExplicit {
		t.Fatal("tosExplicit = false, want true")
	}
}

func TestNormalizeNegativePacketSizeArgs(t *testing.T) {
	args := []string{"ntr", "--psize", "-84", "1.1.1.1"}
	got := normalizeNegativePacketSizeArgs(args)
	want := []string{"ntr", "--psize=-84", "1.1.1.1"}

	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNegativePacketSizeParsesBeforeTarget(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	packetSize := parser.Int("", "psize", &argparse.Options{})
	ipv6Only := parser.Flag("6", "ipv6", &argparse.Options{})
	target := parser.StringPositional(&argparse.Options{})

	args := normalizeNegativePacketSizeArgs([]string{"ntr", "-6", "--psize", "-96", "2606:4700:4700::1111"})
	if err := parser.Parse(args); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !*ipv6Only {
		t.Fatal("-6 should parse as true")
	}
	if *packetSize != -96 {
		t.Fatalf("--psize = %d, want -96", *packetSize)
	}
	if *target != "2606:4700:4700::1111" {
		t.Fatalf("target = %q, want 2606:4700:4700::1111", *target)
	}
}

func TestResolvePacketSizeArg_DefaultsToProtocolMinimum(t *testing.T) {
	got := resolvePacketSizeArg(0, false, trace.TCPTrace, net.ParseIP("2a00:1450:4009:81a::200e"))
	if got != 64 {
		t.Fatalf("resolvePacketSizeArg() = %d, want 64", got)
	}
}

func TestRegisterTracerouteOutputFlagsParsesOutputPath(t *testing.T) {
	parser := argparse.NewParser("nexttrace", "")
	flags := registerTracerouteOutputFlags(parser)
	target := parser.StringPositional(&argparse.Options{})

	if err := parser.Parse([]string{"nexttrace", "-o", "trace.log", "1.1.1.1"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got := strings.TrimSpace(*flags.outputPath); got != "trace.log" {
		t.Fatalf("--output = %q, want trace.log", got)
	}
	if *flags.outputDefault {
		t.Fatal("--output-default should be false")
	}
	if *target != "1.1.1.1" {
		t.Fatalf("target = %q, want 1.1.1.1", *target)
	}
}

func TestRegisterTracerouteOutputFlagsParsesOutputDefault(t *testing.T) {
	parser := argparse.NewParser("nexttrace", "")
	flags := registerTracerouteOutputFlags(parser)
	target := parser.StringPositional(&argparse.Options{})

	if err := parser.Parse([]string{"nexttrace", "-O", "1.1.1.1"}); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !*flags.outputDefault {
		t.Fatal("--output-default should be true")
	}
	if got := strings.TrimSpace(*flags.outputPath); got != "" {
		t.Fatalf("--output = %q, want empty", got)
	}
	if *target != "1.1.1.1" {
		t.Fatalf("target = %q, want 1.1.1.1", *target)
	}
}

func TestResolveOutputPath(t *testing.T) {
	tests := []struct {
		name          string
		outputPath    string
		outputDefault bool
		want          string
		wantErr       string
	}{
		{name: "custom", outputPath: "custom.log", want: "custom.log"},
		{name: "default", outputDefault: true, want: tracelog.DefaultPath},
		{name: "disabled"},
		{name: "conflict", outputPath: "custom.log", outputDefault: true, wantErr: "--output 与 --output-default 不能同时使用"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveOutputPath(tt.outputPath, tt.outputDefault)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveOutputPath returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveOutputPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetFastIPOutputSuppressionRestoresPreviousValue(t *testing.T) {
	orig := util.SuppressFastIPOutput
	util.SuppressFastIPOutput = false
	restore := setFastIPOutputSuppression(true)
	if !util.SuppressFastIPOutput {
		t.Fatal("SuppressFastIPOutput should be true after suppression")
	}
	restore()
	if util.SuppressFastIPOutput != false {
		t.Fatalf("SuppressFastIPOutput = %v, want false", util.SuppressFastIPOutput)
	}
	util.SuppressFastIPOutput = orig
}

func TestShouldForceNoColorForMTUNonTTY(t *testing.T) {
	tests := []struct {
		name        string
		mtuMode     bool
		jsonPrint   bool
		stdoutIsTTY bool
		want        bool
	}{
		{name: "mtu non-tty text", mtuMode: true, jsonPrint: false, stdoutIsTTY: false, want: true},
		{name: "mtu tty text", mtuMode: true, jsonPrint: false, stdoutIsTTY: true, want: false},
		{name: "mtu non-tty json", mtuMode: true, jsonPrint: true, stdoutIsTTY: false, want: false},
		{name: "non-mtu non-tty text", mtuMode: false, jsonPrint: false, stdoutIsTTY: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldForceNoColorForMTUNonTTY(tt.mtuMode, tt.jsonPrint, tt.stdoutIsTTY)
			if got != tt.want {
				t.Fatalf("shouldForceNoColorForMTUNonTTY() = %v, want %v", got, tt.want)
			}
		})
	}
}
