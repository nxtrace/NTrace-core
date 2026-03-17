package cmd

import (
	"strings"
	"testing"

	"github.com/akamensky/argparse"
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
	parser.Int("", "psize", &argparse.Options{Default: 52, Help: buildPayloadSizeHelp()})

	usage := parser.Usage(nil)
	for _, want := range []string{
		"load-balanced paths",
		"rate-limited links",
		"intercontinental",
		"MTU or large-packet",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing tuning guidance %q:\n%s", want, usage)
		}
	}
}

func TestProbeOptionHelpMentionsRandomPacketSizeAndTOS(t *testing.T) {
	parser := argparse.NewParser("ntr", "")
	parser.Int("", "psize", &argparse.Options{Default: 52, Help: buildPayloadSizeHelp()})
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
	parser.Int("", "psize", &argparse.Options{Default: 52})
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
	packetSize := parser.Int("", "psize", &argparse.Options{Default: 52})
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
