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
