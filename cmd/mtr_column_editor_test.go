package cmd

import (
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

func feedMTRKeys(u *mtrUI, k *mtrKeyInput, text string) {
	for _, b := range []byte(text) {
		k.feed(u, b, time.Now())
	}
}

func TestMTRColumnEditorApplyCancelAndIsolation(t *testing.T) {
	u := newMTRUI(nil, 0)
	u.paused.Store(true)
	u.historyMode.Store(true)
	var k mtrKeyInput
	feedMTRKeys(u, &k, "O")
	_, e := u.columnSnapshot()
	if !e.Active || e.Draft != "LSNABWV" {
		t.Fatal(e)
	}
	feedMTRKeys(u, &k, "\x15qprynedgo ")
	if !u.IsPaused() || u.ConsumeRestartRequest() || u.CurrentDisplayMode() != 0 || u.CurrentNameMode() != 0 || !u.IsHistoryMode() || u.IsMPLSDisabled() {
		t.Fatal("shortcut leaked")
	}
	feedMTRKeys(u, &k, "\r")
	_, e = u.columnSnapshot()
	if !e.Active || e.Error == "" {
		t.Fatal("invalid input applied")
	}
	feedMTRKeys(u, &k, "\x15rL\x7f n\r")
	c, e := u.columnSnapshot()
	if e.Active || printer.MTRColumnCodes(c) != "R N" || u.IsHistoryMode() || !u.IsPaused() {
		t.Fatalf("%v %+v", c, e)
	}
	// Invalid/duplicate/empty drafts stay open, cancellation preserves applied columns and view.
	u.historyMode.Store(true)
	for _, draft := range []string{"", "ll", "?"} {
		feedMTRKeys(u, &k, "o\x15"+draft+"\r")
		_, e = u.columnSnapshot()
		if !e.Active || e.Error == "" {
			t.Fatal(e)
		}
		feedMTRKeys(u, &k, "\x1b")
		k.expireEscape(u, time.Now().Add(mtrEscapeDelay))
		c, e = u.columnSnapshot()
		if e.Active || printer.MTRColumnCodes(c) != "R N" || !u.IsHistoryMode() {
			t.Fatal("cancel changed state")
		}
	}
	// Returned selection is isolated from callers.
	c[0] = printer.MTRColumnLoss
	c, _ = u.columnSnapshot()
	if printer.MTRColumnCodes(c) != "R N" {
		t.Fatal("snapshot aliases state")
	}
}

func TestMTRColumnEditorEscapePasteAndLimit(t *testing.T) {
	u := newMTRUI(nil, 0)
	var k mtrKeyInput
	feedMTRKeys(u, &k, "o\x15")
	now := time.Now()
	k.feed(u, 27, now)
	k.expireEscape(u, now.Add(49*time.Millisecond))
	k.feed(u, '[', now.Add(49*time.Millisecond))
	k.feed(u, 'A', now.Add(60*time.Millisecond))
	_, e := u.columnSnapshot()
	if !e.Active || e.Draft != "" {
		t.Fatal("arrow cancelled editor")
	}
	// Arbitrarily split paste wrappers; embedded newline never submits.
	for _, part := range []string{"\x1b[2", "00~", "r\n", " n\r\nl", "\x1b[20", "1~"} {
		feedMTRKeys(u, &k, part)
	}
	_, e = u.columnSnapshot()
	if !e.Active || e.Draft != "r  n  l" {
		t.Fatal(e)
	}
	feedMTRKeys(u, &k, "\r")
	c, e := u.columnSnapshot()
	if e.Active || printer.MTRColumnCodes(c) != "R  N  L" {
		t.Fatal(c, e)
	}
	feedMTRKeys(u, &k, "o\x15"+strings.Repeat("a", 300))
	_, e = u.columnSnapshot()
	if len(e.Draft) != 256 || e.Error == "" {
		t.Fatal(e)
	}
	feedMTRKeys(u, &k, "\x08")
	_, e = u.columnSnapshot()
	if len(e.Draft) != 255 {
		t.Fatal(e)
	}
	var cancelled bool
	u.cancel = func() { cancelled = true }
	feedMTRKeys(u, &k, "\x1b[200~\x03")
	if !cancelled {
		t.Fatal("paste swallowed Ctrl-C")
	}
}

func TestMTRKeyLoopCancelWhileReadBlocked(t *testing.T) {
	r, w := io.Pipe()
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	u := newMTRUI(cancel, 0)
	done := make(chan struct{})
	go func() { u.readKeysLoop(ctx, r); close(done) }()
	waitEditor := func(active bool) {
		t.Helper()
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		for {
			_, editor := u.columnSnapshot()
			if editor.Active == active {
				return
			}
			select {
			case <-u.redraw:
			case <-timer.C:
				t.Fatalf("editor did not become active=%v without another read", active)
			}
		}
	}
	if _, err := w.Write([]byte("o")); err != nil {
		t.Fatal(err)
	}
	waitEditor(true)
	if _, err := w.Write([]byte("\x1b")); err != nil {
		t.Fatal(err)
	}
	// Wait for the parser's redraw instead of assuming a scheduler deadline.
	// Escape timing itself is covered with explicit timestamps above.
	waitEditor(false)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("key loop did not stop")
	}
}

func TestMTRRedrawInitialPausedResizeAndCleanup(t *testing.T) {
	u := newMTRUI(nil, 0)
	u.paused.Store(true)
	var width atomic.Int32
	width.Store(80)
	var renders atomic.Int32
	frames := make(chan string, 100)
	snapshot, stop := startMTRRedraw(u.redraw, func() (int, int) { return int(width.Load()), 24 }, func(_ int, stats []trace.MTRHopStat) {
		_, e := u.columnSnapshot()
		renders.Add(1)
		select {
		case frames <- e.Draft:
		default:
		}
	})
	wait := func(want string) {
		t.Helper()
		select {
		case got := <-frames:
			if got != want {
				t.Fatalf("frame %q want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("missing frame")
		}
	}
	wait("") // Visible before the first probe.
	var k mtrKeyInput
	feedMTRKeys(u, &k, "o")
	wait("LSNABWV")
	width.Store(120)
	wait("LSNABWV")
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for i := range 100 {
				snapshot(i, []trace.MTRHopStat{{TTL: 1, IP: "192.0.2.1", Snt: i}})
				u.requestRedraw()
			}
		})
	}
	wg.Wait()
	stop()
	stop()
	count := renders.Load()
	snapshot(101, nil)
	u.requestRedraw()
	time.Sleep(120 * time.Millisecond)
	if renders.Load() != count {
		t.Fatal("wrote after cleanup")
	}
}

func TestMTRColumnEditorNewCodesKeepReplayShortcutsIsolated(t *testing.T) {
	u := newMTRUI(nil, 0)
	u.replay = &mtrReplayControls{commands: make(chan mtrReplayCommand, 1), duration: time.Hour}
	u.paused.Store(true)
	var k mtrKeyInput
	feedMTRKeys(u, &k, "o\x15 DGJMXI \r")
	columns, editor := u.columnSnapshot()
	if editor.Active || printer.MTRColumnCodes(columns) != " DGJMXI " || u.replayEditor.Active || !u.IsPaused() {
		t.Fatalf("%v %+v", columns, editor)
	}
	feedMTRKeys(u, &k, "o")
	_, editor = u.columnSnapshot()
	if editor.Draft != " DGJMXI " {
		t.Fatal(editor)
	}
}
