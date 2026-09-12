package printer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

func snapshotReportFixture() MTRSnapshot {
	return MTRSnapshot{
		Source: "live", State: "running", CapturedAt: time.Date(2026, 9, 12, 8, 9, 10, 11, time.FixedZone("test", 8*3600)),
		ElapsedNS: int64(3 * time.Second), Generation: 2,
		Session: &mtrsession.Session{
			Target: "目标.example", ResolvedIP: "203.0.113.1", Protocol: "ICMP", SourceHost: "test-host", SourceIP: "192.0.2.2",
			EffectiveParameters: &mtrsession.Parameters{Language: "cn"},
			Display:             mtrsession.DisplayParameters{HostMode: HostModeFull, ShowIPs: true, ShowMPLS: true, Columns: "received,space,space,jint"},
		},
		Stats: []trace.MTRHopStat{
			{TTL: 1, Host: "网关.example", IP: "192.0.2.1", Snt: 8, Received: 7, Dropped: 1, Loss: 12.5, JitterInterarrival: 13.5,
				Geo: &ipgeo.IPGeoData{Asnumber: "1", Country: "中国", City: "上海", Owner: "运营商"}, MPLS: []string{"[MPLS: Label 123]"}},
			{TTL: 2, Host: strings.Repeat("very-long-host-", 12) + ".example", IP: "203.0.113.1", Snt: 8, Received: 8,
				Geo: &ipgeo.IPGeoData{Asnumber: "123456"}},
			{TTL: 3, Snt: 8, Loss: 100},
		},
		PathEnd: &trace.StopReason{Hop: 2, Reason: trace.StopReasonDestination},
	}
}

func TestWriteMTRSnapshotTextUsesCurrentDisplayAndFullHosts(t *testing.T) {
	s := snapshotReportFixture()
	var out bytes.Buffer
	if err := WriteMTRSnapshot(&out, s, "txt"); err != nil {
		t.Fatal(err)
	}
	report := out.String()
	for _, want := range []string{
		"Target: 目标.example", "Source: test-host (192.0.2.2)", "Protocol: ICMP", "State: running", "Generation: 2",
		"Elapsed: 00:00:03.000", "Captured: 2026-09-12T08:09:10.000000011+08:00",
		"Path: Destination Reached at Hop 2", "网关.example (192.0.2.1) 中国 上海 运营商", s.Stats[1].Host,
		"[MPLS: Label 123]", "Rcv    Jint", "13.50", "(waiting for reply)",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("missing %q in report:\n%s", want, report)
		}
	}
	if strings.Contains(report, "StDev") || strings.Contains(report, "Trace Stopped") {
		t.Fatalf("snapshot used default columns or claimed the live session stopped:\n%s", report)
	}
	lines := strings.Split(report, "\n")
	var hostStarts []int
	for _, line := range lines {
		if at := strings.Index(line, "网关.example"); at >= 0 {
			hostStarts = append(hostStarts, displayWidth(line[:at]))
		}
		if at := strings.Index(line, s.Stats[1].Host); at >= 0 {
			hostStarts = append(hostStarts, displayWidth(line[:at]))
		}
	}
	if len(hostStarts) != 2 || hostStarts[0] != hostStarts[1] {
		t.Fatalf("ASN prefixes are not aligned: %v", hostStarts)
	}
}

func TestWriteMTRSnapshotHostModesAndMPLSVisibility(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mode     int
		nameMode int
		want     string
		absent   []string
	}{
		{"base", HostModeBase, HostNamePTRorIP, "网关.example", []string{"AS1", "上海", "运营商"}},
		{"asn", HostModeASN, HostNamePTRorIP, "AS1", []string{"上海", "运营商"}},
		{"city", HostModeCity, HostNamePTRorIP, "上海", []string{"运营商"}},
		{"owner", HostModeOwner, HostNamePTRorIP, "运营商", []string{"上海"}},
		{"ip", HostModeBase, HostNameIPOnly, "192.0.2.1", []string{"网关.example", "AS1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := snapshotReportFixture()
			s.Session.Display.HostMode, s.Session.Display.HostNameMode = tc.mode, tc.nameMode
			s.Session.Display.ShowMPLS = false
			var out bytes.Buffer
			if err := WriteMTRSnapshot(&out, s, "txt"); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("missing %q", tc.want)
			}
			for _, absent := range append(tc.absent, "[MPLS: Label 123]") {
				if strings.Contains(out.String(), absent) {
					t.Errorf("unexpected %q", absent)
				}
			}
		})
	}
}

func TestWriteMTRSnapshotJSONContractAndRawValues(t *testing.T) {
	s := snapshotReportFixture()
	s.Type, s.SchemaVersion = "end", 999
	s.Source, s.State = "replay", "paused"
	s.Replay = &MTRSnapshotReplay{CursorNS: 5, DurationNS: 10, RecordingComplete: false, RecordedPaused: true}
	s.Stats[0].Host = "原值\x1b\n\u202e<script>"
	var out bytes.Buffer
	if err := WriteMTRSnapshot(&out, s, "json"); err != nil {
		t.Fatal(err)
	}
	var decoded MTRSnapshot
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != "mtr_snapshot" || decoded.SchemaVersion != 1 {
		t.Fatalf("unexpected schema: %+v", decoded)
	}
	if !reflect.DeepEqual(decoded.Stats, s.Stats) || !reflect.DeepEqual(decoded.Replay, s.Replay) {
		t.Fatalf("JSON lost statistics or raw values: %s", out.String())
	}
	if s.Type != "end" || s.SchemaVersion != 999 {
		t.Fatal("writing mutated the input")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"end", "ended_at", "end_reason"} {
		if _, exists := fields[absent]; exists {
			t.Errorf("snapshot invented %q", absent)
		}
	}
	if !strings.Contains(out.String(), `"jitter_interarrival_ms": 13.5`) || !strings.Contains(out.String(), `"dropped": 1`) {
		t.Fatal("selected text columns leaked into JSON field selection")
	}
}

func TestWriteMTRSnapshotEmptyStatsAndReplayStatus(t *testing.T) {
	s := snapshotReportFixture()
	s.Stats = nil
	s.Replay = &MTRSnapshotReplay{CursorNS: int64(time.Second), DurationNS: int64(10 * time.Second)}
	var out bytes.Buffer
	if err := WriteMTRSnapshot(&out, s, "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"stats": []`) {
		t.Fatalf("empty statistics must be an array: %s", out.String())
	}
	out.Reset()
	if err := WriteMTRSnapshot(&out, s, "txt"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"No probe statistics.", "Replay: 00:00:01.000 / 00:00:10.000", "Recording: Incomplete"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "Elapsed:") {
		t.Fatal("replay repeats the cursor as elapsed time")
	}
	out.Reset()
	if err := WriteMTRSnapshot(&out, s, "html"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<dt>Elapsed</dt>") || strings.Count(out.String(), "00:00:01.000") != 1 {
		t.Fatal("HTML repeats the replay cursor")
	}
}

func TestWriteMTRSnapshotEscapesHTMLAndRemovesDisplayControls(t *testing.T) {
	s := snapshotReportFixture()
	s.Session.Target = `<script>alert("target")</script>`
	s.Stats[0].Host = "<img src=x onerror=alert(1)>\r\n\x1b\u202e中文"
	s.Stats[0].Geo.Owner = "控制\x07运营商"
	s.Stats[0].MPLS = []string{"标签\x9d\u200e"}
	for _, format := range []string{"txt", "html"} {
		var out bytes.Buffer
		if err := WriteMTRSnapshot(&out, s, format); err != nil {
			t.Fatal(err)
		}
		for _, r := range out.String() {
			if (unicode.IsControl(r) && r != '\n') || unicode.Is(unicode.Cf, r) {
				t.Errorf("%s contains control rune %U", format, r)
			}
		}
		if format == "html" {
			if strings.Contains(out.String(), "<script>") || strings.Contains(out.String(), "<img ") || strings.Contains(out.String(), "<link ") {
				t.Fatalf("HTML contains unescaped content or external resource: %s", out.String())
			}
			if !strings.Contains(out.String(), "&lt;script&gt;") || !strings.Contains(out.String(), "&lt;img ") {
				t.Fatalf("missing escaped content: %s", out.String())
			}
		}
	}
	if !strings.Contains(s.Stats[0].Host, "\x1b") || !strings.Contains(s.Stats[0].Geo.Owner, "\x07") {
		t.Fatal("human report changed source strings")
	}
}

type snapshotShortWriter struct{ err error }

func (w snapshotShortWriter) Write(p []byte) (int, error) { return len(p) / 2, w.err }

func TestWriteMTRSnapshotPropagatesWriteAndEncodingErrors(t *testing.T) {
	s := snapshotReportFixture()
	failure := errors.New("disk full")
	for _, format := range []string{"txt", "json", "html"} {
		if err := WriteMTRSnapshot(snapshotShortWriter{}, s, format); !errors.Is(err, io.ErrShortWrite) {
			t.Errorf("%s short write: %v", format, err)
		}
		if err := WriteMTRSnapshot(snapshotShortWriter{failure}, s, format); !errors.Is(err, failure) {
			t.Errorf("%s writer error: %v", format, err)
		}
	}
	var out bytes.Buffer
	if err := WriteMTRSnapshot(&out, s, "csv"); err == nil || out.Len() != 0 {
		t.Fatal("unknown format should fail before writing")
	}
	s.Session.Display.Columns = "unknown"
	if err := WriteMTRSnapshot(&out, s, "txt"); err == nil || out.Len() != 0 {
		t.Fatal("invalid columns should fail before writing")
	}
	s.Stats[0].Avg = math.NaN()
	if err := WriteMTRSnapshot(&out, s, "json"); err == nil || out.Len() != 0 {
		t.Fatal("invalid JSON data should fail before writing")
	}
}

func TestDefaultMTRSnapshotFilenameIsPortableAndUnicodeSafe(t *testing.T) {
	s := snapshotReportFixture()
	for _, target := range []string{"目标.example", "CON", "../../etc/passwd", "2001:db8::1%en0", "?<>|*\"\\/\x00\u202e", strings.Repeat("界", 200)} {
		s.Session.Target = target
		name := DefaultMTRSnapshotFilename(s, "html")
		if !utf8.ValidString(name) || filepath.Base(name) != name || len(name) >= 255 || strings.ContainsAny(name, "<>:\"/\\|?*\x00\u202e") {
			t.Errorf("unsafe filename %q from target %q", name, target)
		}
		if !strings.HasSuffix(name, "-20260912-080910.000000011.html") {
			t.Errorf("wrong time or extension: %q", name)
		}
	}
	if got := DefaultMTRSnapshotFilename(MTRSnapshot{}, "unknown"); !strings.HasSuffix(got, ".txt") {
		t.Errorf("default format: %q", got)
	}
}

func TestMTRSnapshotColumnNamesRoundTrip(t *testing.T) {
	columns := []MTRColumn{MTRColumnJitterInterarrival, MTRColumnSpace, MTRColumnSpace, MTRColumnReceived}
	names := MTRColumnNames(columns)
	if names != "jint,space,space,received" {
		t.Fatalf("unexpected names: %q", names)
	}
	parsed, err := ParseMTRColumns(names, false)
	if err != nil || !reflect.DeepEqual(parsed, columns) {
		t.Fatalf("columns did not round-trip: %v, %v", parsed, err)
	}
}
