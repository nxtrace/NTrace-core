package printer

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/trace"
)

type MTRReplayStatus struct {
	Cursor, Duration  time.Duration
	Complete, Seeking bool
	RecordedPaused    bool
}

type MTRReplayEditor struct {
	Active       bool
	Draft, Error string
}

func FormatMTRReplayTime(d time.Duration) string {
	ms := max(int64(0), d.Milliseconds())
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

func mtrTUIClock(header MTRTUIHeader) time.Time {
	if !header.Now.IsZero() {
		return header.Now
	}
	return time.Now()
}

func buildMTRReplayControls(header MTRTUIHeader, width int) string {
	r := header.Replay
	state := "Playing"
	if header.Status == MTRTUIPaused {
		state = "Paused"
	}
	if r.Seeking {
		state = "Seeking"
	}
	status := fmt.Sprintf("%s %s/%s", state, FormatMTRReplayTime(r.Cursor), FormatMTRReplayTime(r.Duration))
	if !r.Complete {
		status += " Incomplete"
	}
	if r.RecordedPaused {
		status += " Probe-paused"
	}
	return truncateByDisplayWidth("?:help S:save "+status+"  Q:quit J:time Space:play P:pause R:rewind D:history G:chart O:cols Y:display N:host E:mpls", width)
}

func renderMTRReplayEditor(b *strings.Builder, editor MTRReplayEditor, width int) {
	for _, line := range []string{"Go to HH:MM:SS[.mmm]", "Enter: apply Esc: cancel", "Backspace: delete Ctrl-U: clear"} {
		tuiLine(b, "%s", truncateByDisplayWidth(line, width))
	}
	prefix := "Time: "
	draft := editor.Draft
	available := max(1, width-len(prefix)-1)
	if len(draft) > available {
		draft = draft[len(draft)-available:]
	}
	tuiLine(b, "%s", truncateByDisplayWidth(prefix+draft+"_", width))
	if editor.Error != "" {
		tuiLine(b, "%s", truncateByDisplayWidth(editor.Error, width))
	}
}

// WriteMTRReplayReport shares the live report's host and metric formatting,
// while writing to an explicit sink without terminal escapes.
func WriteMTRReplayReport(w io.Writer, stats []trace.MTRHopStat, opts MTRReportOptions) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Start: %s\n", opts.StartTime.Format("2006-01-02T15:04:05-0700"))
	hosts, hostWidth := prepareMTRReportHosts(stats, opts, normalizeMTRReportLang(opts.Lang))
	columns := opts.Columns
	if columns == nil {
		columns = DefaultMTRColumns()
	}
	widths := mtrColumnWidths(columns, stats)
	hostHeader := opts.SrcHost
	if !opts.Wide {
		hostHeader = reportTruncateToWidth(hostHeader, hostWidth)
	}
	fmt.Fprintf(&b, "HOST: %s %s\n", reportPadRight(hostHeader, hostWidth), mtrSelectedMetrics(columns, widths, nil, false))
	previous := 0
	for i, stat := range stats {
		fmt.Fprintf(&b, "%s%s %s\n", mtrReportPrefix(stat.TTL, previous), reportPadRight(hosts[i], hostWidth), mtrSelectedMetrics(columns, widths, &stat, false))
		previous = stat.TTL
	}
	n, err := io.WriteString(w, b.String())
	if err == nil && n != b.Len() {
		err = io.ErrShortWrite
	}
	return err
}
