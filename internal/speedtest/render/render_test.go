package render

import (
	"bytes"
	"fmt"
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

func TestTTYRendererClearsShorterProgressLine(t *testing.T) {
	var buf bytes.Buffer
	r := NewTTYRenderer(&buf, true)
	r.Render(Event{Kind: KindProgress, Label: "download", Value: "1234567890"})
	r.Render(Event{Kind: KindProgress, Label: "download", Value: "1"})
	out := buf.String()
	clearSeq := "\r" + strings.Repeat(" ", len("  [download] 1234567890")) + "\r"
	if !strings.Contains(out, clearSeq) {
		t.Fatalf("progress output should clear the previous line before shorter update:\n%q", out)
	}
}

func TestTTYRendererClearsColoredProgressByVisibleWidth(t *testing.T) {
	var buf bytes.Buffer
	r := NewTTYRenderer(&buf, false)
	r.Render(Event{Kind: KindProgress, Label: "download", Value: "1234567890"})
	r.Render(Event{Kind: KindProgress, Label: "download", Value: "1"})
	out := buf.String()

	plainLine := "  [download] 1234567890"
	coloredLine := fmt.Sprintf("  %s[%s]%s %s", cDim, "download", cReset, "1234567890")
	visibleClearSeq := "\r" + strings.Repeat(" ", len(plainLine)) + "\r"
	rawClearSeq := "\r" + strings.Repeat(" ", len(coloredLine)) + "\r"
	if !strings.Contains(out, visibleClearSeq) {
		t.Fatalf("colored progress output should clear by visible width:\n%q", out)
	}
	if strings.Contains(out, rawClearSeq) {
		t.Fatalf("colored progress output cleared by raw ANSI byte length:\n%q", out)
	}
}

func TestNewBusWithNilRendererReturnsNil(t *testing.T) {
	if bus := NewBus(nil); bus != nil {
		t.Fatalf("NewBus(nil) = %#v, want nil", bus)
	}
}

func TestBusSendAfterCloseIsNoop(t *testing.T) {
	var buf bytes.Buffer
	bus := NewBus(NewPlainRenderer(&buf))
	bus.Close()
	bus.Send(Event{Kind: KindInfo, Value: "late"})
	bus.Flush()
	if strings.Contains(buf.String(), "late") {
		t.Fatalf("late event rendered after Close():\n%s", buf.String())
	}
}
