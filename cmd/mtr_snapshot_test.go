package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akamensky/argparse"
	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

func TestMTRSnapshotPublishesPathAndResetWithStatistics(t *testing.T) {
	live := mtrLiveSnapshots{start: time.Now(), session: mtrsession.Session{Target: "example.com"}}
	stats := []trace.MTRHopStat{{TTL: 3, IP: "192.0.2.3", Snt: 4,
		Geo: &ipgeo.IPGeoData{Owner: "original", Router: map[string][]string{"router": {"original"}, "empty": {}, "null": nil}}, MPLS: []string{"label"}}}
	live.snapshot(stats)
	frozen := live.store.capture()
	live.pathEnd = &trace.StopReason{Reason: trace.StopReasonDestination, Hop: 2}
	if next := live.store.capture(); next.PathEnd != nil || next.Stats[0].TTL != 3 {
		t.Fatal("path was published before its matching statistics")
	}
	live.snapshot([]trace.MTRHopStat{{TTL: 2, IP: "192.0.2.2", Snt: 1}})
	if next := live.store.capture(); next.PathEnd.Hop != 2 || next.Stats[0].TTL != 2 {
		t.Fatalf("inconsistent path snapshot: %+v", next)
	}
	live.event(trace.MTRSessionEvent{Type: trace.MTRSessionResetEvent, Generation: 1})
	if next := live.store.capture(); next.Generation != 0 || next.PathEnd == nil {
		t.Fatal("reset was published before its matching statistics")
	}
	live.snapshot(nil)
	if next := live.store.capture(); next.Generation != 1 || next.PathEnd != nil || next.Stats == nil || len(next.Stats) != 0 {
		t.Fatalf("invalid reset snapshot: %+v", next)
	}
	stats[0].Geo.Owner, stats[0].MPLS[0] = "changed", "changed"
	stats[0].Geo.Router["router"][0] = "changed"
	if frozen.Stats[0].Geo.Owner != "original" || frozen.Stats[0].MPLS[0] != "label" || frozen.Stats[0].Snt != 4 {
		t.Fatal("captured snapshot changed with later metadata")
	}
	if frozen.Stats[0].Geo.Router["router"][0] != "original" {
		t.Fatal("captured snapshot shares nested router metadata")
	}
	if frozen.Stats[0].Geo.Router["empty"] == nil || frozen.Stats[0].Geo.Router["null"] != nil {
		t.Fatal("captured snapshot changed empty and null router values")
	}
}

func TestMTRSnapshotConcurrentCaptureOwnsItsData(t *testing.T) {
	var store mtrSnapshotStore
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Go(func() {
			for i := 0; i < 100; i++ {
				store.publish(printer.MTRSnapshot{Session: &mtrsession.Session{
					EffectiveParameters: &mtrsession.Parameters{Port: ptrInt(443)}},
					Stats: []trace.MTRHopStat{{Snt: 1, Geo: &ipgeo.IPGeoData{Owner: "owner"}}}})
				snapshot := store.capture()
				*snapshot.Session.EffectiveParameters.Port = 80
				snapshot.Stats[0].Geo.Owner = "mutated"
			}
		})
	}
	wg.Wait()
	got := store.capture()
	if *got.Session.EffectiveParameters.Port != 443 || got.Stats[0].Geo.Owner != "owner" {
		t.Fatal("reader modified the published snapshot")
	}
}

func TestMTRSnapshotUsesCurrentDisplayWithoutChangingSession(t *testing.T) {
	var store mtrSnapshotStore
	store.publish(printer.MTRSnapshot{Session: &mtrsession.Session{Display: mtrsession.DisplayParameters{HostMode: 0}}})
	ui := newMTRUI(nil, 4)
	ui.columns = []printer.MTRColumn{printer.MTRColumnLoss, printer.MTRColumnSpace, printer.MTRColumnJitter}
	ui.ToggleNameMode()
	ui.ToggleMPLS()
	got := captureMTRDisplay(&store, ui, true).Session.Display
	if got.HostMode != 4 || got.HostNameMode != 1 || !got.ShowIPs || got.ShowMPLS || got.Columns != "loss,space,jitter" {
		t.Fatalf("display=%+v", got)
	}
	if store.capture().Session.Display.HostMode != 0 {
		t.Fatal("display capture changed the stored session")
	}
}

func TestMTRSnapshotHonorsInitialMPLSSettingAndRuntimeToggle(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		conf := trace.Config{DisableMPLS: disabled}
		ui := newMTRTraceUI(nil, 4, conf.DisableMPLS, []printer.MTRColumn{printer.MTRColumnLoss})
		var store mtrSnapshotStore
		store.publish(printer.MTRSnapshot{Session: &mtrsession.Session{
			EffectiveParameters: &mtrsession.Parameters{DisableMPLS: conf.DisableMPLS},
		}})
		initial := captureMTRDisplay(&store, ui, false)
		if ui.IsMPLSDisabled() != disabled || initial.Session.Display.ShowMPLS == disabled {
			t.Fatalf("disable_mpls=%v: initial display=%+v", disabled, initial.Session.Display)
		}
		var keys mtrKeyInput
		for _, key := range []string{"e", "E"} {
			feedMTRKeys(ui, &keys, key)
			snapshot := captureMTRDisplay(&store, ui, false)
			wantShow := disabled
			if key == "E" {
				wantShow = !disabled
			}
			if snapshot.Session.Display.ShowMPLS != wantShow {
				t.Fatalf("disable_mpls=%v after %s: display=%+v", disabled, key, snapshot.Session.Display)
			}
			if snapshot.Session.EffectiveParameters.DisableMPLS != disabled || initial.Session.Display.ShowMPLS == disabled {
				t.Fatal("display toggle changed probe settings or an earlier snapshot")
			}
		}
	}
}

func TestMTRSnapshotPauseWithoutAnotherProbe(t *testing.T) {
	var store mtrSnapshotStore
	store.publish(printer.MTRSnapshot{Source: "live", State: "running", Session: &mtrsession.Session{}})
	ui := newMTRUI(nil, 0)
	ui.paused.Store(true)
	if got := captureMTRDisplay(&store, ui, false).State; got != "paused" {
		t.Fatalf("state=%s after pause without a probe", got)
	}
	ui.paused.Store(false)
	if got := captureMTRDisplay(&store, ui, false).State; got != "running" {
		t.Fatalf("state=%s after resume without a probe", got)
	}
}

func TestMTRReplaySnapshotRetainsPositionAndIncompleteState(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	current := &mtrReplayCursor{session: &mtrsession.Session{StartedAt: start},
		state: trace.NewMTRReplayState(false, 30), cursor: 2 * time.Second}
	var store mtrSnapshotStore
	publishMTRReplaySnapshot(&store, current, 10*time.Second, false, true)
	snapshot := store.capture()
	if snapshot.Source != "replay" || snapshot.State != "running" || snapshot.Replay.RecordingComplete || snapshot.Replay.CursorNS != int64(2*time.Second) || snapshot.Replay.DurationNS != int64(10*time.Second) {
		t.Fatalf("snapshot=%+v replay=%+v", snapshot, snapshot.Replay)
	}
	if time.Since(snapshot.CapturedAt) > time.Second {
		t.Fatal("capture time is the old recording timestamp")
	}
	var output bytes.Buffer
	if err := printer.WriteMTRSnapshot(&output, *snapshot, "json"); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"ended_at", "end_reason", "end"} {
		if _, ok := document[forbidden]; ok {
			t.Fatalf("snapshot invents %s", forbidden)
		}
	}
}

func TestMTRExitSummaryRetainsInterruptionAndEmptyInitialization(t *testing.T) {
	var output bytes.Buffer
	if err := writeMTRExitSummary(&output, nil, context.Canceled); err != nil || output.Len() != 0 {
		t.Fatal("initialization without a snapshot produced a summary")
	}
	snapshot := &printer.MTRSnapshot{Source: "live", Session: &mtrsession.Session{Target: "example.com"}}
	if err := writeMTRExitSummary(&output, snapshot, context.Canceled); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "interrupted") {
		t.Fatalf("summary lost interruption: %s", output.String())
	}
}

func TestNoMTRSummaryDoesNotSelectMode(t *testing.T) {
	parser := argparse.NewParser(appBinName, "")
	flags := registerMTRFlags(parser)
	if err := parser.Parse([]string{appBinName, "--no-mtr-summary"}); err != nil {
		t.Fatal(err)
	}
	if !*flags.noSummary || *flags.mtrMode || *flags.reportMode || *flags.wideMode {
		t.Fatal("summary presentation flag selected a probe mode")
	}
	var out, diagnostic bytes.Buffer
	handled, code := maybeRunMTRReplayMode([]string{"--mtr-replay", "--help", "--no-mtr-summary"}, &out, &diagnostic)
	if !handled || code != 0 || !strings.Contains(out.String(), "no-mtr-summary") {
		t.Fatalf("replay help: handled=%v code=%d stderr=%s", handled, code, &diagnostic)
	}
}
