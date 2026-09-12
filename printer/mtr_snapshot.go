package printer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/trace"
)

// MTRSnapshot is a point-in-time report, not an event recording or a completed
// session. Callers own the snapshot and must not change it while writing.
type MTRSnapshot struct {
	SchemaVersion int                 `json:"schema_version"`
	Type          string              `json:"type"`
	Source        string              `json:"source"`
	CapturedAt    time.Time           `json:"captured_at"`
	ElapsedNS     int64               `json:"elapsed_ns"`
	Generation    uint64              `json:"generation"`
	State         string              `json:"state"`
	Session       *mtrsession.Session `json:"session"`
	PathEnd       *trace.StopReason   `json:"path_end"`
	Stats         []trace.MTRHopStat  `json:"stats"`
	Replay        *MTRSnapshotReplay  `json:"replay,omitempty"`
}

// MTRSnapshotReplay describes the committed replay position in a snapshot.
type MTRSnapshotReplay struct {
	CursorNS          int64 `json:"cursor_ns"`
	DurationNS        int64 `json:"duration_ns"`
	RecordingComplete bool  `json:"recording_complete"`
	RecordedPaused    bool  `json:"recorded_paused"`
}

// MTRColumnNames serializes the current display selection using CLI column names.
func MTRColumnNames(columns []MTRColumn) string {
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		if int(column) < len(mtrColumnDefinitions) {
			names = append(names, mtrColumnDefinitions[column].name)
		}
	}
	return strings.Join(names, ",")
}

// WriteMTRSnapshot writes an immutable report as txt, json or self-contained html.
// JSON preserves all recorded values; human-readable formats clean display text.
func WriteMTRSnapshot(w io.Writer, s MTRSnapshot, format string) error {
	var content []byte
	var err error
	switch format {
	case "json":
		s.SchemaVersion, s.Type = 1, "mtr_snapshot"
		if s.Stats == nil {
			s.Stats = []trace.MTRHopStat{}
		}
		content, err = json.MarshalIndent(s, "", "  ")
		if err == nil {
			content = append(content, '\n')
		}
	case "txt", "html":
		var report mtrSnapshotReport
		report, err = buildMTRSnapshotReport(s)
		if err != nil {
			break
		}
		var b bytes.Buffer
		if format == "html" {
			err = mtrSnapshotHTML.Execute(&b, report)
		} else {
			b.WriteString("NextTrace MTR snapshot\n")
			for _, field := range report.Metadata {
				fmt.Fprintf(&b, "%s: %s\n", field.Label, field.Value)
			}
			b.WriteByte('\n')
			b.WriteString(report.Table)
		}
		content = b.Bytes()
	default:
		return fmt.Errorf("unknown MTR snapshot format %q", format)
	}
	if err != nil {
		return err
	}
	n, err := w.Write(content)
	if err == nil && n != len(content) {
		err = io.ErrShortWrite
	}
	return err
}

// DefaultMTRSnapshotFilename returns a portable filename in the current directory.
func DefaultMTRSnapshotFilename(s MTRSnapshot, format string) string {
	target := "target"
	if s.Session != nil && s.Session.Target != "" {
		target = s.Session.Target
	}
	var name strings.Builder
	for _, r := range target {
		if name.Len() >= 80 {
			break
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '.' || r == '-' || r == '_' {
			name.WriteRune(r)
		} else {
			name.WriteByte('-')
		}
	}
	target = strings.Trim(name.String(), ".-_")
	if target == "" {
		target = "target"
	}
	if format != "json" && format != "html" {
		format = "txt"
	}
	return fmt.Sprintf("nexttrace-%s-%s.%s", target, s.CapturedAt.Format("20060102-150405.000000000"), format)
}

type mtrSnapshotField struct{ Label, Value string }

type mtrSnapshotReport struct {
	Metadata []mtrSnapshotField
	Table    string
}

func buildMTRSnapshotReport(s MTRSnapshot) (mtrSnapshotReport, error) {
	display := mtrsession.DisplayParameters{ShowMPLS: true}
	lang := "cn"
	if s.Session != nil {
		display = s.Session.Display
		if s.Session.EffectiveParameters != nil {
			lang = s.Session.EffectiveParameters.Language
		}
	}
	columns := DefaultMTRColumns()
	if display.Columns != "" {
		var err error
		columns, err = ParseMTRColumns(display.Columns, false)
		if err != nil {
			return mtrSnapshotReport{}, err
		}
	}
	hosts := make([]mtrHostParts, len(s.Stats))
	asnWidth := 0
	for i, stat := range s.Stats {
		parts := buildTUIHostParts(stat, display.HostMode, display.HostNameMode, lang, display.ShowIPs)
		parts.asn = mtrSnapshotText(parts.asn)
		parts.base = mtrSnapshotText(parts.base)
		for j := range parts.extras {
			parts.extras[j] = mtrSnapshotText(parts.extras[j])
		}
		hosts[i] = parts
		asnWidth = max(asnWidth, displayWidth(parts.asn))
	}
	rows := make([]string, len(s.Stats))
	hostWidth, hopWidth := len("Host"), len("Hop")
	for i, stat := range s.Stats {
		rows[i] = mtrSnapshotText(appendMTRResponseMarker(formatTUIHost(hosts[i], asnWidth), stat))
		hostWidth = max(hostWidth, displayWidth(rows[i]))
		hopWidth = max(hopWidth, len(fmt.Sprint(stat.TTL)))
	}
	widths := mtrColumnWidths(columns, s.Stats)
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s  %s\n", padLeft("Hop", hopWidth), padRight("Host", hostWidth), mtrSelectedMetrics(columns, widths, nil, false))
	previousTTL := -1
	for i, stat := range s.Stats {
		hop := ""
		if stat.TTL != previousTTL {
			hop = fmt.Sprint(stat.TTL)
		}
		fmt.Fprintf(&b, "%s  %s  %s\n", padLeft(hop, hopWidth), padRight(rows[i], hostWidth), mtrSelectedMetrics(columns, widths, &stat, false))
		previousTTL = stat.TTL
		if display.ShowMPLS {
			for _, mpls := range stat.MPLS {
				fmt.Fprintf(&b, "%s  %s\n", strings.Repeat(" ", hopWidth), mtrSnapshotText(mpls))
			}
		}
	}
	if len(s.Stats) == 0 {
		b.WriteString("No probe statistics.\n")
	}
	return mtrSnapshotReport{Metadata: mtrSnapshotMetadata(s), Table: b.String()}, nil
}

func mtrSnapshotMetadata(s MTRSnapshot) []mtrSnapshotField {
	var fields []mtrSnapshotField
	add := func(label, value string) {
		value = mtrSnapshotText(value)
		if value == "" {
			value = "-"
		}
		fields = append(fields, mtrSnapshotField{label, value})
	}
	if session := s.Session; session != nil {
		add("Target", session.Target)
		if session.ResolvedIP != session.Target {
			add("Resolved IP", session.ResolvedIP)
		}
		add("Protocol", session.Protocol)
		source := session.SourceIP
		if source == "" && session.EffectiveParameters != nil {
			source = session.EffectiveParameters.SourceAddress
		}
		if session.SourceHost != "" && session.SourceHost != source {
			if source != "" {
				source = session.SourceHost + " (" + source + ")"
			} else {
				source = session.SourceHost
			}
		}
		add("Source", source)
		if !session.StartedAt.IsZero() {
			add("Started", session.StartedAt.Format(time.RFC3339Nano))
		}
	}
	add("Captured", s.CapturedAt.Format(time.RFC3339Nano))
	if s.Replay == nil {
		add("Elapsed", FormatMTRReplayTime(time.Duration(s.ElapsedNS)))
	}
	add("Mode", s.Source)
	add("State", s.State)
	add("Generation", fmt.Sprint(s.Generation))
	if s.PathEnd != nil {
		add("Path", strings.TrimPrefix(FormatTraceStopReason(s.PathEnd), "Trace Stopped: "))
	}
	if r := s.Replay; r != nil {
		add("Replay", FormatMTRReplayTime(time.Duration(r.CursorNS))+" / "+FormatMTRReplayTime(time.Duration(r.DurationNS)))
		complete := "Incomplete"
		if r.RecordingComplete {
			complete = "Complete"
		}
		add("Recording", complete)
		add("Recorded paused", fmt.Sprint(r.RecordedPaused))
	}
	return fields
}

// Control and format characters are never emitted into human-readable fields.
// Keep the original strings untouched for JSON and event recordings.
func mtrSnapshotText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, value)
}

var mtrSnapshotHTML = template.Must(template.New("mtr-snapshot").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>NextTrace MTR snapshot</title>
<style>body{margin:2rem;font:16px system-ui,sans-serif;color:#17212b;background:#fff}h1{font-size:1.5rem}dl{display:grid;grid-template-columns:max-content 1fr;gap:.35rem 1rem}dt{font-weight:600}dd{margin:0;overflow-wrap:anywhere}pre{overflow-x:auto;padding:1rem;background:#f4f6f8;font:14px/1.6 ui-monospace,monospace;white-space:pre}@media print{body{margin:0}pre{overflow:visible;font-size:9px}}</style>
</head><body><h1>NextTrace MTR snapshot</h1><dl>{{range .Metadata}}<dt>{{.Label}}</dt><dd>{{.Value}}</dd>{{end}}</dl><pre>{{.Table}}</pre></body></html>
`))
