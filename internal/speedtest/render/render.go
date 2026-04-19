package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-runewidth"
)

type EventKind int

const (
	KindBanner EventKind = iota
	KindHeader
	KindInfo
	KindWarn
	KindResult
	KindKV
	KindLine
	KindProgress
	KindSync
)

type Event struct {
	Kind  EventKind
	Label string
	Value string
	Time  time.Time
	done  chan struct{}
}

type Renderer interface {
	Render(Event)
}

type Bus struct {
	ch     chan Event
	wg     sync.WaitGroup
	sendWG sync.WaitGroup
	once   sync.Once
	mu     sync.Mutex
	closed bool
}

func NewBus(r Renderer) *Bus {
	if r == nil {
		return nil
	}
	b := &Bus{ch: make(chan Event, 256)}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for ev := range b.ch {
			r.Render(ev)
			if ev.done != nil {
				close(ev.done)
			}
		}
	}()
	return b
}

func (b *Bus) Send(ev Event) {
	_ = b.send(ev)
}

func (b *Bus) send(ev Event) bool {
	if b == nil {
		return false
	}
	ev.Time = time.Now()
	if !b.beginSend() {
		return false
	}
	defer b.sendWG.Done()

	if ev.Kind == KindProgress {
		select {
		case b.ch <- ev:
			return true
		default:
			return false
		}
	}
	b.ch <- ev
	return true
}

func (b *Bus) beginSend() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	b.sendWG.Add(1)
	return true
}

func (b *Bus) Close() {
	if b == nil {
		return
	}
	b.once.Do(func() {
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()
		b.sendWG.Wait()
		close(b.ch)
	})
	b.wg.Wait()
}

func (b *Bus) Flush() {
	if b == nil {
		return
	}
	done := make(chan struct{})
	if !b.send(Event{Kind: KindSync, done: done}) {
		return
	}
	<-done
}

func (b *Bus) Banner(v string)          { b.Send(Event{Kind: KindBanner, Value: v}) }
func (b *Bus) Header(v string)          { b.Send(Event{Kind: KindHeader, Value: v}) }
func (b *Bus) Info(v string)            { b.Send(Event{Kind: KindInfo, Value: v}) }
func (b *Bus) Warn(v string)            { b.Send(Event{Kind: KindWarn, Value: v}) }
func (b *Bus) Result(v string)          { b.Send(Event{Kind: KindResult, Value: v}) }
func (b *Bus) KV(k, v string)           { b.Send(Event{Kind: KindKV, Label: k, Value: v}) }
func (b *Bus) Line()                    { b.Send(Event{Kind: KindLine}) }
func (b *Bus) Progress(label, v string) { b.Send(Event{Kind: KindProgress, Label: label, Value: v}) }

func IsTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
)

type TTYRenderer struct {
	mu            sync.Mutex
	w             io.Writer
	noColor       bool
	lastProg      string
	lastProgWidth int
}

func NewTTYRenderer(w io.Writer, noColor bool) *TTYRenderer {
	return &TTYRenderer{w: w, noColor: noColor}
}

func (t *TTYRenderer) style(parts ...string) string {
	if t.noColor {
		return ""
	}
	return strings.Join(parts, "")
}

func (t *TTYRenderer) Render(ev Event) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.lastProg != "" && ev.Kind != KindProgress {
		t.clearProgressLine()
	}

	switch ev.Kind {
	case KindBanner:
		writef(t.w, "\n  %s%s%s%s\n", t.style(cCyan, cBold), ev.Value, t.style(cReset), "")
	case KindHeader:
		writef(t.w, "\n%s%s  ▸ %s%s\n", t.style(cCyan, cBold), "", ev.Value, t.style(cReset))
	case KindInfo:
		writef(t.w, "  %s%s[+]%s %s\n", t.style(cGreen, cBold), "", t.style(cReset), ev.Value)
	case KindWarn:
		writef(t.w, "  %s%s[!]%s %s\n", t.style(cYellow, cBold), "", t.style(cReset), ev.Value)
	case KindResult:
		writef(t.w, "  %s%s    ➜  %s%s\n", t.style(cGreen, cBold), "", ev.Value, t.style(cReset))
	case KindKV:
		writef(t.w, "  %s%s%-18s%s %s\n", t.style(cDim, cBold), "", ev.Label+":", t.style(cReset), ev.Value)
	case KindLine:
		if t.noColor {
			writeln(t.w, "  --------------------------------------------------------")
		} else {
			writef(t.w, "%s\n", t.style(cDim)+strings.Repeat("─", 58)+t.style(cReset))
		}
	case KindProgress:
		plainLine := fmt.Sprintf("  [%s] %s", ev.Label, ev.Value)
		line := plainLine
		if !t.noColor {
			line = fmt.Sprintf("  %s[%s]%s %s", t.style(cDim), ev.Label, t.style(cReset), ev.Value)
		}
		if t.lastProg != "" {
			t.clearProgressLine()
		}
		writef(t.w, "\r%s", line)
		t.lastProg = line
		t.lastProgWidth = runewidth.StringWidth(plainLine)
	case KindSync:
	}
}

func (t *TTYRenderer) clearProgressLine() {
	writef(t.w, "\r%s\r", strings.Repeat(" ", t.lastProgWidth))
	t.lastProg = ""
	t.lastProgWidth = 0
}

type PlainRenderer struct {
	mu sync.Mutex
	w  io.Writer
}

func NewPlainRenderer(w io.Writer) *PlainRenderer {
	return &PlainRenderer{w: w}
}

func (p *PlainRenderer) Render(ev Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch ev.Kind {
	case KindBanner:
		writef(p.w, "\n  %s\n", ev.Value)
	case KindHeader:
		writef(p.w, "\n  > %s\n", ev.Value)
	case KindInfo:
		writef(p.w, "  [+] %s\n", ev.Value)
	case KindWarn:
		writef(p.w, "  [!] %s\n", ev.Value)
	case KindResult:
		writef(p.w, "      -> %s\n", ev.Value)
	case KindKV:
		writef(p.w, "  %-18s %s\n", ev.Label+":", ev.Value)
	case KindLine:
		writeln(p.w, "  "+strings.Repeat("-", 56))
	case KindProgress:
		writef(p.w, "  [%s] %s\n", ev.Label, ev.Value)
	case KindSync:
	}
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
