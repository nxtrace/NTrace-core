package render

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
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
	ch   chan Event
	wg   sync.WaitGroup
	once sync.Once
}

func NewBus(r Renderer) *Bus {
	b := &Bus{ch: make(chan Event, 256)}
	if r == nil {
		close(b.ch)
		return b
	}
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
	if b == nil {
		return
	}
	ev.Time = time.Now()
	select {
	case b.ch <- ev:
	default:
		b.ch <- ev
	}
}

func (b *Bus) Close() {
	if b == nil {
		return
	}
	b.once.Do(func() { close(b.ch) })
	b.wg.Wait()
}

func (b *Bus) Flush() {
	if b == nil {
		return
	}
	done := make(chan struct{})
	b.Send(Event{Kind: KindSync, done: done})
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
	mu       sync.Mutex
	w        io.Writer
	noColor  bool
	lastProg string
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
		fmt.Fprintf(t.w, "\r%s\r", strings.Repeat(" ", len(t.lastProg)+2))
		t.lastProg = ""
	}

	switch ev.Kind {
	case KindBanner:
		fmt.Fprintf(t.w, "\n  %s%s%s%s\n", t.style(cCyan, cBold), ev.Value, t.style(cReset), "")
	case KindHeader:
		fmt.Fprintf(t.w, "\n%s%s  ▸ %s%s\n", t.style(cCyan, cBold), "", ev.Value, t.style(cReset))
	case KindInfo:
		fmt.Fprintf(t.w, "  %s%s[+]%s %s\n", t.style(cGreen, cBold), "", t.style(cReset), ev.Value)
	case KindWarn:
		fmt.Fprintf(t.w, "  %s%s[!]%s %s\n", t.style(cYellow, cBold), "", t.style(cReset), ev.Value)
	case KindResult:
		fmt.Fprintf(t.w, "  %s%s    ➜  %s%s\n", t.style(cGreen, cBold), "", ev.Value, t.style(cReset))
	case KindKV:
		fmt.Fprintf(t.w, "  %s%s%-18s%s %s\n", t.style(cDim, cBold), "", ev.Label+":", t.style(cReset), ev.Value)
	case KindLine:
		if t.noColor {
			fmt.Fprintln(t.w, "  --------------------------------------------------------")
		} else {
			fmt.Fprintf(t.w, "%s\n", t.style(cDim)+strings.Repeat("─", 58)+t.style(cReset))
		}
	case KindProgress:
		line := fmt.Sprintf("  [%s] %s", ev.Label, ev.Value)
		if !t.noColor {
			line = fmt.Sprintf("  %s[%s]%s %s", t.style(cDim), ev.Label, t.style(cReset), ev.Value)
		}
		fmt.Fprintf(t.w, "\r%s", line)
		t.lastProg = line
	case KindSync:
	}
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
		fmt.Fprintf(p.w, "\n  %s\n", ev.Value)
	case KindHeader:
		fmt.Fprintf(p.w, "\n  > %s\n", ev.Value)
	case KindInfo:
		fmt.Fprintf(p.w, "  [+] %s\n", ev.Value)
	case KindWarn:
		fmt.Fprintf(p.w, "  [!] %s\n", ev.Value)
	case KindResult:
		fmt.Fprintf(p.w, "      -> %s\n", ev.Value)
	case KindKV:
		fmt.Fprintf(p.w, "  %-18s %s\n", ev.Label+":", ev.Value)
	case KindLine:
		fmt.Fprintln(p.w, "  "+strings.Repeat("-", 56))
	case KindProgress:
		fmt.Fprintf(p.w, "  [%s] %s\n", ev.Label, ev.Value)
	case KindSync:
	}
}
