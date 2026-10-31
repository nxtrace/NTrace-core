package printer

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/trace"
)

func TestMTRTUIPrinterPreservesTerminalHistory(t *testing.T) {
	printFrame := MTRTUIPrinter("192.0.2.1", "", "192.0.2.1", "test", time.Now(),
		"host", "192.0.2.2", "en", nil, false,
		nil, nil, nil, nil, nil, nil, nil)
	output := captureStdout(t, func() {
		for sent := 1; sent <= 3; sent++ {
			printFrame(sent, []trace.MTRHopStat{{TTL: 1, IP: "192.0.2.3", Snt: sent, Received: sent}})
		}
	})
	if !strings.Contains(output, "192.0.2.3") {
		t.Fatal("interactive refresh must render the visible hop")
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*J`).MatchString(output) {
		t.Fatal("interactive refresh must not erase the display and move old frames into terminal history")
	}
	if strings.ContainsAny(output, "\r\n") {
		t.Fatal("interactive refresh must use absolute positions without scrolling line feeds")
	}
}

func TestMTRTUIProbeErrorRepaintsPausedHeader(t *testing.T) {
	var renderer MTRTUIRenderer
	var output strings.Builder
	header := MTRTUIHeader{Status: MTRTUIPaused, APIInfo: "preferred API IP: cached"}
	if err := renderer.renderWithSize(&output, header, nil, 150, 24); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	header.ProbeError = "mtr: probe error (1/10): unavailable"
	if err := renderer.renderWithSize(&output, header, nil, 150, 24); err != nil {
		t.Fatal(err)
	}
	if update := output.String(); !strings.Contains(update, header.ProbeError) || strings.Contains(update, header.APIInfo) || strings.ContainsAny(update, "\r\n") {
		t.Fatalf("diagnostic was not rendered as a non-scrolling header update: %q", update)
	}
	output.Reset()
	header.ProbeError = ""
	if err := renderer.renderWithSize(&output, header, nil, 150, 24); err != nil {
		t.Fatal(err)
	}
	if update := output.String(); !strings.Contains(update, header.APIInfo) || strings.Contains(update, "unavailable") {
		t.Fatalf("clearing the diagnostic must restore API information: %q", update)
	}
}

// Model the visible rows rather than matching text anywhere in an accumulated
// transcript: old hop and dialog text must actually disappear from the screen.
type mtrTestScreen struct {
	width int
	rows  []string
}

var mtrScreenPosition = regexp.MustCompile(`\x1b\[(\d+);1H`)
var mtrScreenStyle = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func (s *mtrTestScreen) apply(t *testing.T, update string) []int {
	t.Helper()
	if strings.ContainsAny(update, "\r\n") || regexp.MustCompile(`\x1b\[[0-9;]*J`).MatchString(update) {
		t.Fatalf("update can scroll or erase the display: %q", update)
	}
	matches := mtrScreenPosition.FindAllStringSubmatchIndex(update, -1)
	if update != "" && (len(matches) == 0 || matches[0][0] != 0) {
		t.Fatalf("update does not start at an absolute row: %q", update)
	}
	var touched []int
	for i, match := range matches {
		row, err := strconv.Atoi(update[match[2]:match[3]])
		if err != nil || row < 1 || row > len(s.rows) {
			t.Fatalf("out-of-bounds row %d: %q", row, update)
		}
		end := len(update)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		payload := update[match[1]:end]
		if !strings.HasPrefix(payload, "\x1b[0m\x1b[2K") || !strings.HasSuffix(payload, "\x1b[0m") {
			t.Fatalf("row is not cleared before writing with isolated styles: %q", payload)
		}
		text := mtrScreenStyle.ReplaceAllString(strings.TrimPrefix(payload, "\x1b[0m\x1b[2K"), "")
		if !utf8.ValidString(text) || strings.ContainsRune(text, '\x1b') || ansi.StringWidth(text) > s.width {
			t.Fatalf("invalid or wrapping row at width %d: %q", s.width, text)
		}
		s.rows[row-1] = text
		touched = append(touched, row)
	}
	return touched
}

func mtrScreenHeader() MTRTUIHeader {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	return MTRTUIHeader{Target: "192.0.2.9", StartTime: now, Now: now}
}

func mtrScreenRender(t *testing.T, r *MTRTUIRenderer, h MTRTUIHeader, stats []trace.MTRHopStat, width, height int) string {
	t.Helper()
	var out strings.Builder
	if err := r.renderWithSize(&out, h, stats, width, height); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestMTRTUIRendererSkipsUnchangedRows(t *testing.T) {
	var renderer MTRTUIRenderer
	header := mtrScreenHeader()
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "192.0.2.1", Snt: 3, Received: 3, Last: 1},
		{TTL: 2, IP: "192.0.2.2", Snt: 3, Received: 3, Last: 2},
	}
	screen := mtrTestScreen{width: 120, rows: make([]string, 24)}
	first := mtrScreenRender(t, &renderer, header, stats, 120, 24)
	if got := screen.apply(t, first); len(got) != 24 {
		t.Fatalf("first frame cleared %d rows, want the full viewport", len(got))
	}
	if got := mtrScreenRender(t, &renderer, header, stats, 120, 24); got != "" {
		t.Fatalf("unchanged frame wrote to terminal: %q", got)
	}
	stats[1].Last = 7.25
	update := mtrScreenRender(t, &renderer, header, stats, 120, 24)
	if got := screen.apply(t, update); !slices.Equal(got, []int{7}) {
		t.Fatalf("single changed hop touched rows %v, want [7]", got)
	}
	if !strings.Contains(screen.rows[5], "192.0.2.1") || !strings.Contains(screen.rows[6], "7.25") {
		t.Fatalf("incorrect visible hops: %v", screen.rows)
	}
	header.Status = MTRTUIPaused
	if got := screen.apply(t, mtrScreenRender(t, &renderer, header, stats, 120, 24)); !slices.Equal(got, []int{3}) {
		t.Fatalf("pause touched rows %v, want [3]", got)
	}
	header.Status = MTRTUIRunning
	if got := screen.apply(t, mtrScreenRender(t, &renderer, header, stats, 120, 24)); !slices.Equal(got, []int{3}) {
		t.Fatalf("resume touched rows %v, want [3]", got)
	}
}

func TestMTRTUIRendererClearsShorterAndRemovedRows(t *testing.T) {
	var renderer MTRTUIRenderer
	header := mtrScreenHeader()
	stats := []trace.MTRHopStat{
		{TTL: 1, Host: "long-router-name.example", IP: "192.0.2.1", Snt: 1, Received: 1, MPLS: []string{"label 12345"}},
		{TTL: 2, IP: "192.0.2.2", Snt: 1, Received: 1},
	}
	screen := mtrTestScreen{width: 120, rows: make([]string, 24)}
	screen.apply(t, mtrScreenRender(t, &renderer, header, stats, 120, 24))
	if !strings.Contains(strings.Join(screen.rows, "\n"), "label 12345") {
		t.Fatal("test did not display MPLS continuation")
	}
	stats[0].Host = "r"
	stats[0].MPLS = nil
	stats = stats[:1]
	screen.apply(t, mtrScreenRender(t, &renderer, header, stats, 120, 24))
	text := strings.Join(screen.rows, "\n")
	for _, stale := range []string{"long-router-name", "label 12345", "192.0.2.2"} {
		if strings.Contains(text, stale) {
			t.Fatalf("stale text %q remains on screen: %q", stale, text)
		}
	}
	if screen.rows[6] != "" || screen.rows[7] != "" {
		t.Fatalf("removed rows were not cleared: %v", screen.rows)
	}
}

func TestMTRTUIRendererSharesViewsAndDialogs(t *testing.T) {
	var renderer MTRTUIRenderer
	header := mtrScreenHeader()
	stats := []trace.MTRHopStat{{TTL: 1, IP: "192.0.2.1", Snt: 1, Received: 1}}
	screen := mtrTestScreen{width: 120, rows: make([]string, 32)}
	views := []struct {
		name string
		set  func(*MTRTUIHeader)
		want string
	}{
		{"table", func(*MTRTUIHeader) {}, "Packets"},
		{"history", func(h *MTRTUIHeader) { h.HistoryMode = true }, "History"},
		{"columns", func(h *MTRTUIHeader) { h.Columns = []MTRColumn{MTRColumnReceived, MTRColumnAvg} }, "Rcv"},
		{"column editor", func(h *MTRTUIHeader) { h.ColumnEditor = MTRColumnEditor{Active: true, Draft: "LS"} }, "Fields: LS_"},
		{"save", func(h *MTRTUIHeader) { h.SaveDialog = MTRSaveDialog{Active: true, Format: "json", Path: "report.json"} }, "Save report"},
		{"help", func(h *MTRTUIHeader) { h.HelpDialog.Active = true }, "Keyboard shortcuts"},
		{"replay editor", func(h *MTRTUIHeader) { h.ReplayEditor.Active = true }, "Go to HH:MM:SS"},
		{"table again", func(*MTRTUIHeader) {}, "Packets"},
	}
	for _, replay := range []bool{false, true} {
		for _, view := range views {
			t.Run(fmt.Sprintf("replay=%v/%s", replay, view.name), func(t *testing.T) {
				h := header
				if replay {
					h.Replay = &MTRReplayStatus{}
				}
				view.set(&h)
				screen.apply(t, mtrScreenRender(t, &renderer, h, stats, 120, 32))
				if text := strings.Join(screen.rows, "\n"); !strings.Contains(text, view.want) {
					t.Fatalf("missing %q in visible screen: %q", view.want, text)
				}
			})
		}
		for _, stale := range []string{"Save report", "Keyboard shortcuts", "Go to HH:MM:SS", "Fields:"} {
			if strings.Contains(strings.Join(screen.rows, "\n"), stale) {
				t.Fatalf("closed dialog %q remains visible", stale)
			}
		}
	}
}

func TestMTRTUIRendererBoundsAndResize(t *testing.T) {
	orig := color.NoColor
	t.Cleanup(func() { color.NoColor = orig })
	for _, noColor := range []bool{true, false} {
		color.NoColor = noColor
		var renderer MTRTUIRenderer
		header := mtrScreenHeader()
		header.Target = strings.Repeat("中文e\u0301😀", 40)
		var stats []trace.MTRHopStat
		for ttl := 1; ttl <= 35; ttl++ {
			stats = append(stats, trace.MTRHopStat{TTL: ttl, Host: "中文e\u0301😀", IP: fmt.Sprintf("192.0.2.%d", ttl), Snt: 1, Received: 1})
		}
		for _, size := range [][2]int{{120, 24}, {24, 5}, {1, 1}, {8, 2}, {80, 10}, {160, 40}} {
			width, height := size[0], size[1]
			screen := mtrTestScreen{width: width, rows: make([]string, height)}
			for i := range screen.rows {
				screen.rows[i] = "old reflowed content"
			}
			update := mtrScreenRender(t, &renderer, header, stats, width, height)
			if got := screen.apply(t, update); len(got) != height {
				t.Fatalf("resize %v touched %d rows, want %d", size, len(got), height)
			}
			if screen.rows[height-1] != "" {
				t.Fatalf("reserved bottom row is not blank at %v", size)
			}
			if height == 1 && ansi.Strip(update) != "" {
				t.Fatalf("one-row terminal received body text: %q", update)
			}
		}
	}
}

func TestMTRTUIRendererFullWidthRowAndStyleReset(t *testing.T) {
	var renderer MTRTUIRenderer
	header := mtrScreenHeader()
	header.SaveDialog = MTRSaveDialog{Active: true, Format: "json"}
	width := len("Save report")
	screen := mtrTestScreen{width: width, rows: make([]string, 8)}
	screen.apply(t, mtrScreenRender(t, &renderer, header, nil, width, 8))
	if screen.rows[0] != "Save report" {
		t.Fatalf("last column was lost: %q", screen.rows[0])
	}
}

type mtrScreenFailWriter struct{ err error }

func (w mtrScreenFailWriter) Write(data []byte) (int, error) {
	return len(data) / 2, w.err
}

func TestMTRTUIRendererRebuildsAfterWriteFailure(t *testing.T) {
	writeErr := errors.New("terminal disconnected")
	for _, failure := range []error{nil, writeErr} {
		var renderer MTRTUIRenderer
		header := mtrScreenHeader()
		mtrScreenRender(t, &renderer, header, nil, 80, 10)
		header.Status = MTRTUIPaused
		err := renderer.renderWithSize(mtrScreenFailWriter{err: failure}, header, nil, 80, 10)
		want := failure
		if want == nil {
			want = io.ErrShortWrite
		}
		if !errors.Is(err, want) {
			t.Fatalf("write error = %v, want %v", err, want)
		}
		screen := mtrTestScreen{width: 80, rows: make([]string, 10)}
		if got := screen.apply(t, mtrScreenRender(t, &renderer, header, nil, 80, 10)); len(got) != 10 {
			t.Fatalf("recovery rebuilt only %d rows", len(got))
		}
		if !strings.Contains(screen.rows[2], "Paused") {
			t.Fatalf("recovery lost current state: %v", screen.rows)
		}
	}
}
