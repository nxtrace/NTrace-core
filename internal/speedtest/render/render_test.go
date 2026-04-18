package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlainRendererRendersReadableOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewPlainRenderer(&buf)
	r.Render(Event{Kind: KindHeader, Value: "Idle Latency"})
	r.Render(Event{Kind: KindInfo, Value: "Samples: 20"})
	r.Render(Event{Kind: KindResult, Value: "100 Mbps"})
	out := buf.String()
	for _, want := range []string{"> Idle Latency", "[+] Samples: 20", "-> 100 Mbps"} {
		if !strings.Contains(out, want) {
			t.Fatalf("plain output missing %q:\n%s", want, out)
		}
	}
}

func TestTTYRendererHonorsNoColor(t *testing.T) {
	var buf bytes.Buffer
	r := NewTTYRenderer(&buf, true)
	r.Render(Event{Kind: KindBanner, Value: "NextTrace Speed"})
	r.Render(Event{Kind: KindWarn, Value: "degraded"})
	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Fatalf("TTY no-color output should not contain ANSI sequences:\n%s", out)
	}
}
