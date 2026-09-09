package printer

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/nxtrace/NTrace-core/trace"
)

func TestMTRColumnContract(t *testing.T) {
	for _, tt := range []struct{ names, codes string }{
		{"loss,snt,last,avg,best,wrst,stdev", "LSNABWV"},
		{"received", "r"}, {" LAST ", "n"},
		{"loss,snt,received,last,avg,best,wrst,stdev", "LSRNABWV"},
		{"stdev,wrst,best,avg,last,received,snt,loss", "vwbanrsl"},
		{"avg,received,loss,stdev,snt", "ARLVS"},
	} {
		a, err := ParseMTRColumns(tt.names, false)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ParseMTRColumns(tt.codes, true)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(a, b) {
			t.Fatalf("%q != %q", a, b)
		}
		c, err := ParseMTRColumns(MTRColumnCodes(a), true)
		if err != nil || !slices.Equal(a, c) {
			t.Fatalf("roundtrip: %v", err)
		}
	}
	for _, s := range []string{"", " ", ",", "loss,", ",snt", "loss,,snt", "loss,LOSS", "rcv", "unknown"} {
		if _, err := ParseMTRColumns(s, false); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, s := range []string{"", " ", "LL", "l L", "Z", "L,S", "L\nS", "L\tS", "Ｌ"} {
		if _, err := ParseMTRColumns(s, true); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestMTRSelectedLayoutPreservesCompleteNumbers(t *testing.T) {
	columns, _ := ParseMTRColumns("received,stdev,loss,snt,last,avg,best,wrst", false)
	maxInt := int(^uint(0) >> 1)
	stats := []trace.MTRHopStat{
		{TTL: 1, IP: "192.0.2.1", Host: "中文主机.example.test", Snt: maxInt, Received: 1000, Last: 0, Avg: 1.23, Best: 0, Wrst: 99999.99, StDev: 12.34, MPLS: []string{"[MPLS: Lbl 123 Exp 0 S 1 TTL 1]"}},
		{TTL: 1, IP: "192.0.2.2", Snt: 1000, Received: 999, Loss: 0.1},
		{TTL: 2, Loss: 100, Snt: 1000},
	}
	before := fmt.Sprint(stats)
	for _, width := range []int{24, 40, 60, 80, 120, 200} {
		var b strings.Builder
		renderMTRSelectedColumns(&b, MTRTUIHeader{Columns: columns, Lang: "cn"}, stats, width)
		out := b.String()
		for _, line := range strings.Split(out, "\r\n") {
			if displayWidth(line) > width {
				t.Fatalf("width %d overflow: %q", width, line)
			}
		}
		if strings.Contains(out, "Select fewer") {
			continue
		}
		for _, value := range []string{fmt.Sprint(maxInt), "1000", "99999.99", "0.00", "12.34", "Rcv"} {
			if !strings.Contains(out, value) {
				t.Fatalf("missing %s at %d: %s", value, width, out)
			}
		}
		if strings.Contains(out, "Packets") || strings.Contains(out, "Pings") {
			t.Fatal("misleading groups")
		}
	}
	if fmt.Sprint(stats) != before {
		t.Fatal("renderer changed statistics")
	}
	widths := mtrColumnWidths(columns, stats)
	if widths[0] != 4 || widths[3] != len(fmt.Sprint(maxInt)) {
		t.Fatalf("count widths %v", widths)
	}
	if strings.TrimSpace(mtrSelectedMetrics(columns, widths, &stats[2], false)) != "" {
		t.Fatal("waiting metrics not blank")
	}
	if sntWidthForMax(maxInt) != len(fmt.Sprint(maxInt)) {
		t.Fatal("max integer width")
	}
}

func TestMTRSelectedReportAndTableOrder(t *testing.T) {
	columns, _ := ParseMTRColumns("received,avg,loss", false)
	stats := []trace.MTRHopStat{{TTL: 1, IP: "192.0.2.1", Snt: 1000, Received: 999, Avg: 1.23, Loss: 0.1}}
	for _, wide := range []bool{false, true} {
		out := captureStdout(t, func() { MTRReportPrint(stats, MTRReportOptions{Columns: columns, SrcHost: "source", Wide: wide}) })
		if !strings.Contains(out, "Rcv  Avg Loss%") || !strings.Contains(out, "999 1.23  0.1%") || strings.Contains(out, "StDev") {
			t.Fatal(out)
		}
	}
	out := captureStdout(t, func() { MTRTablePrinterWithColumns(stats, 1, 0, 0, "cn", false, columns) })
	for _, v := range []string{"Hop", "Rcv", "Avg", "Loss%", "Host"} {
		i := strings.Index(out, v)
		if i < 0 {
			t.Fatal(out)
		}
		out = out[i+len(v):]
	}
}

func TestMTRCustomFrameAndEditorStayWithinTerminal(t *testing.T) {
	columns, _ := ParseMTRColumns("received,loss,snt,last,avg,best,wrst,stdev", false)
	for _, width := range []int{24, 40, 60, 80, 120, 200} {
		for _, editing := range []bool{false, true} {
			header := MTRTUIHeader{Columns: columns, ColumnEditor: MTRColumnEditor{Active: editing, Draft: strings.Repeat("L", 256)}, Target: "192.0.2.1", SrcHost: "中文主机.example", Status: MTRTUIPaused}
			out := mtrTUIRenderStringWithWidth(header, nil, width)
			out = strings.TrimPrefix(out, "\x1b[H\x1b[2J")
			for _, line := range strings.Split(out, "\r\n") {
				if displayWidth(line) > width {
					t.Fatalf("width %d: %q", width, line)
				}
			}
			if editing {
				for _, code := range []string{"L: Loss", "S: Sent", "R: Received", "N: Newest", "A: Average", "B: Min/Best", "W: Max/Worst", "V: Standard", "G: Geometric", "J: Current", "M: Jitter", "X: Worst", "I: Interarrival"} {
					if !strings.Contains(out, code) {
						t.Fatalf("missing %s in %d: %s", code, width, out)
					}
				}
			} else if !strings.Contains(out, "O") {
				t.Fatal("missing O hint")
			}
		}
	}
}

func TestMTRColumnsDistinguishesEmptyAndUnknownEntries(t *testing.T) {
	for _, input := range []string{"", " ", ",loss", "loss,", "loss,,snt", "loss, ,snt"} {
		_, err := ParseMTRColumns(input, false)
		if err == nil || err.Error() != "empty MTR column entry" {
			t.Fatalf("input %q: %v", input, err)
		}
	}
	_, err := ParseMTRColumns("unknown", false)
	if err == nil || err.Error() != `unknown MTR column "unknown"` {
		t.Fatalf("unknown column: %v", err)
	}
}

func TestMTRDefaultControlsFitTerminal(t *testing.T) {
	old := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = old })
	for _, width := range []int{24, 40, 60, 80, 120, 160, 200} {
		for _, history := range []bool{false, true} {
			line := buildMTRTUIControlsLine(MTRTUIHeader{HistoryMode: history}, width)
			if displayWidth(line) > width {
				t.Fatalf("width %d: %q", width, line)
			}
			if !strings.Contains(line, "O") {
				t.Fatal("missing column editor hint")
			}
		}
	}
	line := buildMTRTUIControlsLine(MTRTUIHeader{}, 120)
	for _, hint := range []string{"O-columns", "Y-display", "N-host", "D-history(off)"} {
		if !strings.Contains(line, hint) {
			t.Fatalf("missing %q: %s", hint, line)
		}
	}
}

func TestMTRDefaultControlsMeasureVisibleColorWidth(t *testing.T) {
	oldKey, oldStatus := mtrTUIKeyHiColor, mtrTUIStatusColor
	t.Cleanup(func() { mtrTUIKeyHiColor, mtrTUIStatusColor = oldKey, oldStatus })
	key, status := color.New(color.FgHiWhite), color.New(color.FgHiYellow, color.Bold)
	key.EnableColor()
	status.EnableColor()
	mtrTUIKeyHiColor, mtrTUIStatusColor = key.SprintFunc(), status.SprintFunc()
	line := buildMTRTUIControlsLine(MTRTUIHeader{}, 120)
	if !strings.Contains(line, "\x1b[") {
		t.Fatalf("expected colored controls: %q", line)
	}
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(line, "")
	if displayWidth(plain) > 120 || !strings.Contains(plain, "D-history(off)") {
		t.Fatalf("colored controls: %q", line)
	}
}

func TestMTRExpandedColumnsAndSpaces(t *testing.T) {
	for _, tt := range []struct{ names, codes string }{
		{"dropped,gmean,jitter,javg,jmax,jint", "DGJMXI"},
		{"loss,space,avg", "L A"},
		{"space,loss,space,space,avg,space", " L  A "},
	} {
		columns, err := ParseMTRColumns(tt.names, false)
		if err != nil || MTRColumnCodes(columns) != tt.codes {
			t.Fatalf("%v %v", columns, err)
		}
		roundtrip, err := ParseMTRColumns(tt.codes, true)
		if err != nil || !slices.Equal(columns, roundtrip) {
			t.Fatalf("roundtrip %v %v", roundtrip, err)
		}
	}
	for _, input := range []string{"space", "space,space", "jitter,JITTER", "loss, ,avg"} {
		if _, err := ParseMTRColumns(input, false); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	stat := trace.MTRHopStat{TTL: 1, IP: "192.0.2.1", Dropped: 2, GMean: 20, Jitter: 20, JitterAvg: 10, JitterMax: 20, JitterInterarrival: 29.375}
	columns, _ := ParseMTRColumns("dropped,gmean,jitter,javg,jmax,jint", false)
	widths := mtrColumnWidths(columns, []trace.MTRHopStat{stat})
	if got := strings.Fields(mtrSelectedMetrics(columns, widths, &stat, false)); !slices.Equal(got, []string{"2", "20.00", "20.00", "10.00", "20.00", "29.38"}) {
		t.Fatal(got)
	}
	stat = trace.MTRHopStat{TTL: 1, Snt: 2, Dropped: 2, Loss: 100}
	if strings.TrimSpace(mtrSelectedMetrics(columns, widths, &stat, false)) != "" {
		t.Fatal("waiting row has metrics")
	}
}

func TestMTRSpaceWidthAcrossTextOutputs(t *testing.T) {
	old := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = old })
	stat := trace.MTRHopStat{TTL: 1, IP: "192.0.2.1", Loss: 1, Avg: 2}
	for _, tt := range []struct {
		codes string
		extra int
	}{{"LA", 0}, {"L A", 1}, {"L  A", 2}, {" LA ", 2}} {
		columns, _ := ParseMTRColumns(tt.codes, true)
		base, _ := ParseMTRColumns("LA", true)
		renderers := []func([]MTRColumn) string{
			func(c []MTRColumn) string {
				return mtrSelectedMetrics(c, mtrColumnWidths(c, []trace.MTRHopStat{stat}), &stat, false)
			},
			func(c []MTRColumn) string {
				return captureStdout(t, func() { MTRTablePrinterWithColumns([]trace.MTRHopStat{stat}, 1, 0, 0, "en", false, c) })
			},
			func(c []MTRColumn) string {
				return captureStdout(t, func() {
					MTRReportPrint([]trace.MTRHopStat{stat}, MTRReportOptions{Columns: c, SrcHost: "source", Wide: true})
				})
			},
			func(c []MTRColumn) string {
				var b strings.Builder
				err := WriteMTRReplayReport(&b, []trace.MTRHopStat{stat}, MTRReportOptions{Columns: c, SrcHost: "source", Wide: true})
				if err != nil {
					t.Fatal(err)
				}
				return b.String()
			},
		}
		for i, render := range renderers {
			got, want := strings.Split(strings.TrimSuffix(render(columns), "\n"), "\n"), strings.Split(strings.TrimSuffix(render(base), "\n"), "\n")
			if len(got) != len(want) {
				t.Fatal(got, want)
			}
			for row := range got {
				if strings.Contains(got[row], "Loss%") || strings.Contains(got[row], "1.0%") {
					if len(got[row])-len(want[row]) != tt.extra {
						t.Fatalf("renderer %d codes %q: %q versus %q", i, tt.codes, got[row], want[row])
					}
				}
			}
		}
	}
}

func TestMTRFieldsPageDimensions(t *testing.T) {
	old := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = old })
	for _, size := range [][2]int{{80, 24}, {24, 24}, {80, 10}, {24, 8}, {24, 4}} {
		for _, replay := range []bool{false, true} {
			h := MTRTUIHeader{Target: "192.0.2.1", SrcHost: "source", ColumnEditor: MTRColumnEditor{Active: true, Draft: "LS DGJMXI", Error: "duplicate MTR column loss"}}
			if replay {
				h.Replay = &MTRReplayStatus{Complete: true}
			}
			var b strings.Builder
			mtrTUIRenderWithSize(&b, h, nil, size[0], size[1])
			out := strings.TrimPrefix(b.String(), "\x1b[H\x1b[2J")
			if strings.Count(out, "\r\n") >= size[1] || !strings.Contains(out, "Fields: LS DGJMXI_") {
				t.Fatal(size, out)
			}
			for _, line := range strings.Split(out, "\r\n") {
				if displayWidth(line) > size[0] {
					t.Fatalf("overflow %v: %q", size, line)
				}
			}
			if size[1] >= 24 {
				for _, label := range []string{"D: Dropped Packets", "G: Geometric Mean", "J: Current Jitter", "M: Jitter Mean/Avg.", "X: Worst Jitter", "I: Interarrival Jitter"} {
					if !strings.Contains(out, label) {
						t.Fatalf("missing %s: %s", label, out)
					}
				}
			}
		}
	}
}

func TestMTRFieldsPagePreservesColoredHeader(t *testing.T) {
	oldTitle := mtrTUITitleColor
	t.Cleanup(func() { mtrTUITitleColor = oldTitle })
	style := color.New(color.FgHiWhite)
	style.EnableColor()
	mtrTUITitleColor = style.SprintFunc()
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, width := range []int{40, 80, 200} {
		header := MTRTUIHeader{Target: "192.0.2.1", SrcHost: "中文主机.example", Version: strings.Repeat("v", 30), Now: time.Unix(1, 0), ColumnEditor: MTRColumnEditor{Active: true, Draft: "LS DGJMXI"}}
		var b strings.Builder
		renderMTRColumnEditor(&b, header, width, 24)
		lines := strings.Split(b.String(), "\r\n")
		want := []string{buildMTRTUITitleLine(header, width), buildMTRTUIRouteLine(header, width, header.Now)}
		for i := range want {
			if i == 0 && !strings.Contains(want[i], "\x1b[") {
				t.Fatal("color output not exercised")
			}
			if lines[i] != want[i] {
				t.Fatalf("colored header changed at width %d: got %q want %q", width, lines[i], want[i])
			}
		}
		for _, line := range lines {
			if displayWidth(ansi.ReplaceAllString(line, "")) > width {
				t.Fatalf("colored overflow: %q", line)
			}
		}
	}
}
