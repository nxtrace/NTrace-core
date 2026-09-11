package trace

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

func TestMTRSessionReplayMatchesScheduler(t *testing.T) {
	state := NewMTRReplayState(false, 5)
	var events []MTRSessionEvent
	resetRequested := false
	rt, err := newMTRSchedulerRuntime(t.Context(), &mockTTLProber{}, NewMTRAggregator(), mtrSchedulerConfig{
		BeginHop: 1, MaxHops: 5, ParallelRequests: 1,
		IsResetRequested: func() bool { v := resetRequested; resetRequested = false; return v },
		OnEvent: func(event MTRSessionEvent) error {
			if event.At.IsZero() {
				t.Error("session event has no application timestamp")
			}
			// Exercise the actual serialized representation, including private
			// response peer attribution and integer RTT round trips.
			encoded, err := json.Marshal(event)
			if err != nil {
				return err
			}
			var decoded MTRSessionEvent
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				return err
			}
			events = append(events, decoded)
			return state.Apply(decoded)
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.workers.shutdown(nil)
	compare := func() {
		t.Helper()
		got := state.Snapshot()
		if want := rt.snapshotStats(); !reflect.DeepEqual(got.Stats, want) {
			t.Fatalf("replay stats differ:\n got %#v\nwant %#v", got.Stats, want)
		}
		if got.Iteration != rt.computeIteration() || !reflect.DeepEqual(state.PathEnd(), rt.pathTracker.pathEnd()) {
			t.Fatalf("replay boundary/iteration differ: got %+v, %v", got, state.PathEnd())
		}
	}
	probe := func(ttl int, ip string, rtt time.Duration, kind string) {
		t.Helper()
		result := mtrProbeResult{TTL: ttl, RTT: rtt}
		if ip != "" {
			result.Success = true
			result.Addr = &net.IPAddr{IP: net.ParseIP(ip)}
			result.MPLS = []string{"label 20", "label 10"}
			result.Response = &MTRProbeResponse{Kind: kind, Description: kind}
		}
		// Completion timestamps may move backwards; application order is authoritative.
		rt.processProbeSuccess(ttl, result, time.Now().Add(-time.Duration(len(events)+1)*time.Second))
		if err := context.Cause(rt.ctx); err != nil {
			t.Fatal(err)
		}
		compare()
	}
	probe(5, "2001:db8::5", time.Nanosecond, MTRResponseTransit)
	probe(2, "", 0, "")
	probe(2, "192.0.2.2", 0, MTRResponseTransit)
	probe(2, "192.0.2.2", 13234567*time.Nanosecond, MTRResponseTransit)
	probe(2, "192.0.2.20", 2*time.Millisecond, MTRResponseTransit)
	probe(2, "", 0, "")
	rt.processMetadataResult(mtrMetadataResult{gen: 0, kind: mtrMetadataKindGeo, patch: mtrMetadataPatch{
		ip: "192.0.2.2", geo: &ipgeo.IPGeoData{Country: "中国", CountryEn: "China", City: "北京", CityEn: "Beijing", Router: map[string][]string{"64500": {"route"}}},
	}})
	rt.processMetadataResult(mtrMetadataResult{gen: 0, kind: mtrMetadataKindHost, patch: mtrMetadataPatch{ip: "192.0.2.2", host: "router.example"}})
	compare()
	probe(2, "192.0.2.20", 3*time.Millisecond, MTRResponseUnreachable)
	if len(state.Snapshot().Stats) != 3 {
		t.Fatalf("unreachable should hide high TTL: %+v", state.Snapshot())
	}
	probe(2, "192.0.2.20", 4*time.Millisecond, MTRResponseTransit)
	if len(state.Snapshot().Stats) != 4 {
		t.Fatalf("reopen should retain high TTL: %+v", state.Snapshot())
	}
	probe(3, "192.0.2.3", 5*time.Millisecond, MTRResponseDestination)
	if len(state.agg.Snapshot()) != 4 {
		t.Fatal("destination did not clear TTL 5")
	}
	probe(1, "192.0.2.1", 6*time.Millisecond, MTRResponseDestination)
	if len(state.agg.Snapshot()) != 1 {
		t.Fatal("lower destination retained old rows")
	}
	resetRequested = true
	rt.handleReset()
	compare()
	before := len(events)
	rt.inFlight = 1
	rt.processResult(mtrCompletedProbe{ttl: 1, gen: 0, result: mtrProbeResult{TTL: 1}})
	rt.processMetadataResult(mtrMetadataResult{gen: 0, kind: mtrMetadataKindHost, patch: mtrMetadataPatch{ip: "192.0.2.1", host: "stale.example"}})
	if len(events) != before {
		t.Fatal("old generation produced session events")
	}
	probe(1, "2001:db8::1", 7*time.Millisecond, MTRResponseTransit)
	if state.Generation() != 1 || state.Snapshot().Stats[0].Snt != 1 {
		t.Fatal("reset did not begin fresh statistics")
	}
}

func TestMTRSessionSyncMetadataUsesAccountedHop(t *testing.T) {
	ClearCaches()
	defer ClearCaches()
	var recorded MTRSessionEvent
	var legacy mtrProbeResult
	rt, err := newMTRSchedulerRuntime(t.Context(), &mockTTLProber{}, NewMTRAggregator(), mtrSchedulerConfig{
		BeginHop: 1, MaxHops: 1, FillGeo: true,
		BaseConfig: Config{Lang: "cn", IPGeoSource: func(string, time.Duration, string, bool) (*ipgeo.IPGeoData, error) {
			return &ipgeo.IPGeoData{Asnumber: "64500", Country: "中国", CountryEn: "China"}, nil
		}},
		OnEvent: func(event MTRSessionEvent) error { recorded = event; return nil },
	}, nil, func(result mtrProbeResult, _ int, _ time.Time) { legacy = result })
	if err != nil {
		t.Fatal(err)
	}
	defer rt.workers.shutdown(nil)
	completed := time.Now().Add(-time.Second)
	rt.processProbeSuccess(1, mtrProbeResult{TTL: 1, Success: true, Addr: &net.IPAddr{IP: net.ParseIP("9.9.9.245")}, RTT: time.Nanosecond}, completed)
	if recorded.Probe == nil || recorded.Probe.Geo == nil || recorded.Probe.Geo.CountryEn != "China" {
		t.Fatalf("record lost synchronous metadata: %+v", recorded)
	}
	if !recorded.Probe.CompletedAt.Equal(completed) || !recorded.At.After(completed) {
		t.Fatal("completion/application timestamps conflated")
	}
	if legacy.Geo != nil {
		t.Fatal("existing OnProbe payload changed")
	}
	state := NewMTRReplayState(false, 1)
	if err := state.Apply(recorded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state.Snapshot().Stats, rt.snapshotStats()) {
		t.Fatal("recorded synchronous metadata differs from live statistics")
	}
}

func TestMTRSessionSyntheticTimeoutAndCausalPathOrder(t *testing.T) {
	var events []MTRSessionEvent
	var legacyOrder []string
	rt, err := newMTRSchedulerRuntime(t.Context(), &mockTTLProber{}, NewMTRAggregator(), mtrSchedulerConfig{
		BeginHop: 1, MaxHops: 2, MaxConsecErrors: 2,
		OnEvent:   func(event MTRSessionEvent) error { events = append(events, event); return nil },
		OnPathEnd: func(*StopReason) { legacyOrder = append(legacyOrder, "path_end") },
	}, nil, func(mtrProbeResult, int, time.Time) { legacyOrder = append(legacyOrder, "probe") })
	if err != nil {
		t.Fatal(err)
	}
	defer rt.workers.shutdown(nil)
	completed := time.Now()
	rt.processProbeError(1, errors.New("temporary failure"), completed)
	if len(events) != 0 {
		t.Fatal("uncounted retry recorded as probe")
	}
	rt.processProbeError(1, errors.New("temporary failure"), completed)
	if len(events) != 1 || events[0].Type != MTRSessionProbeEvent || events[0].Probe.Success || !events[0].Probe.CompletedAt.Equal(completed) {
		t.Fatalf("missing counted synthetic timeout: %+v", events)
	}
	legacyOrder = nil
	rt.processProbeSuccess(2, mtrProbeResult{TTL: 2, Success: true, Addr: &net.IPAddr{IP: net.ParseIP("192.0.2.2")}, Response: &MTRProbeResponse{Kind: MTRResponseDestination}}, completed)
	if len(events) != 3 || events[1].Type != MTRSessionProbeEvent || events[2].Type != MTRSessionPathEndEvent {
		t.Fatalf("wrong causal order: %+v", events)
	}
	if !reflect.DeepEqual(legacyOrder, []string{"path_end", "probe"}) {
		t.Fatalf("legacy callbacks reordered: %v", legacyOrder)
	}
	// A tail cut after the cause probe must still rebuild the live boundary.
	state := NewMTRReplayState(false, 2)
	for _, event := range events[:2] {
		if err := state.Apply(event); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(state.Snapshot().Stats, rt.snapshotStats()) || !reflect.DeepEqual(state.PathEnd(), rt.pathTracker.pathEnd()) {
		t.Fatal("prefix ending at cause probe lost path semantics")
	}
}

func TestMTRSessionObserverFailureStopsWorkers(t *testing.T) {
	for _, eventType := range []string{MTRSessionProbeEvent, MTRSessionPathEndEvent, MTRSessionMetadataEvent, MTRSessionPauseEvent, MTRSessionResetEvent} {
		t.Run(eventType, func(t *testing.T) {
			ClearCaches()
			defer ClearCaches()
			sentinel := errors.New("recording write failed")
			var calls atomic.Int32
			prober := &mockTTLProber{probeFn: func(ctx context.Context, ttl int) (mtrProbeResult, error) {
				calls.Add(1)
				if eventType == MTRSessionResetEvent {
					<-ctx.Done()
					return mtrProbeResult{}, ctx.Err()
				}
				return mtrProbeResult{TTL: ttl, Success: true, Addr: &net.IPAddr{IP: net.ParseIP("9.9.9.244")}}, nil
			}}
			cfg := mtrSchedulerConfig{
				BeginHop: 1, MaxHops: 1, MaxPerHop: 1, ParallelRequests: 1, HopInterval: time.Millisecond,
				OnEvent: func(event MTRSessionEvent) error {
					if event.Type == eventType {
						return sentinel
					}
					return nil
				},
				IsPaused:         func() bool { return eventType == MTRSessionPauseEvent },
				IsResetRequested: func() bool { return eventType == MTRSessionResetEvent },
			}
			if eventType == MTRSessionMetadataEvent {
				cfg.FillGeo, cfg.AsyncMetadata = true, true
				cfg.BaseConfig.IPGeoSource = func(string, time.Duration, string, bool) (*ipgeo.IPGeoData, error) {
					return &ipgeo.IPGeoData{Asnumber: "64500"}, nil
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			err := runMTRScheduler(ctx, prober, NewMTRAggregator(), cfg, nil, nil)
			if !errors.Is(err, sentinel) {
				t.Fatalf("error = %v, want write failure", err)
			}
			if atomic.LoadInt32(&prober.closeCnt) != 1 {
				t.Fatal("prober was not closed exactly once")
			}
			want := int32(1)
			if eventType == MTRSessionPauseEvent {
				want = 0
			}
			if calls.Load() != want {
				t.Fatalf("launched %d probes, want %d", calls.Load(), want)
			}
		})
	}
}

func TestMTRReplayRejectsInvalidEvents(t *testing.T) {
	tests := []MTRSessionEvent{
		{Type: "future_event"},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 256}},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 1, Success: true}},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 1, Response: &MTRProbeResponse{Kind: MTRResponseDestination}}},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 1, IP: "192.0.2.1", Response: &MTRProbeResponse{Kind: MTRResponseUnreachable}}},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 1, IP: "192.0.2.1", Success: true, Response: &MTRProbeResponse{Kind: "unknown"}}},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 1, IP: "192.0.2.1", Success: true, Response: &MTRProbeResponse{}}},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 1, IP: "not-an-ip"}},
		{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 1, RTT: -1}},
		{Type: MTRSessionProbeEvent, Generation: 1, Probe: &MTRSessionProbe{TTL: 1}},
		{Type: MTRSessionResetEvent, Generation: 2},
		{Type: MTRSessionMetadataEvent},
		{Type: MTRSessionPathEndEvent, PathEnd: &StopReason{Hop: 256, Reason: StopReasonDestination}},
		{Type: MTRSessionPathEndEvent, PathEnd: &StopReason{Hop: 1, Reason: "unknown"}},
	}
	for _, event := range tests {
		state := NewMTRReplayState(false, 30)
		if err := state.Apply(event); err == nil {
			t.Errorf("accepted invalid event: %+v", event)
		}
		if len(state.Snapshot().Stats) != 0 || state.Generation() != 0 || state.PathEnd() != nil {
			t.Errorf("invalid event changed state: %+v", event)
		}
	}
}

func TestMTRReplayPathEndRequiresProbeEvidence(t *testing.T) {
	empty := NewMTRReplayState(true, 5)
	if err := empty.Apply(MTRSessionEvent{Type: MTRSessionPathEndEvent, PathEnd: &StopReason{Hop: 5, Reason: StopReasonMaxHops}}); err == nil {
		t.Fatal("accepted max-hops without probing the maximum TTL")
	}
	if empty.PathEnd() != nil {
		t.Fatal("invalid max-hops event changed the path")
	}
	for _, bounded := range []bool{false, true} {
		for _, kind := range []string{MTRResponseTransit, MTRResponseDestination, MTRResponseUnreachable} {
			state := NewMTRReplayState(bounded, 5)
			probe := MTRSessionEvent{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: 3, IP: "192.0.2.3", Success: true, Response: &MTRProbeResponse{Kind: kind, Description: "test response", Marker: "!H"}}}
			if err := state.Apply(probe); err != nil {
				t.Fatal(err)
			}
			before, edge := state.Snapshot(), state.PathEnd()
			for _, fabricated := range []*StopReason{
				{Hop: 1, Reason: StopReasonDestination}, {Hop: 1, Reason: StopReasonUnreachable},
				{Hop: 3, Reason: StopReasonDestination, Responses: []string{"fabricated"}},
			} {
				if err := state.Apply(MTRSessionEvent{Type: MTRSessionPathEndEvent, PathEnd: fabricated}); err == nil {
					t.Fatalf("accepted fabricated path bound: kind=%s bounded=%v edge=%+v", kind, bounded, fabricated)
				}
				if !reflect.DeepEqual(before, state.Snapshot()) || !reflect.DeepEqual(edge, state.PathEnd()) {
					t.Fatal("rejected path event changed statistics or evidence")
				}
			}
			if edge != nil {
				if err := state.Apply(MTRSessionEvent{Type: MTRSessionPathEndEvent}); err == nil {
					t.Fatal("accepted reopening without transit evidence")
				}
			}
			if err := state.Apply(MTRSessionEvent{Type: MTRSessionPathEndEvent, PathEnd: edge}); err != nil {
				t.Fatalf("rejected matching evidence: %v", err)
			}
		}
	}
}

func TestMTRSessionBoundedReplayNaturalCompletion(t *testing.T) {
	state := NewMTRReplayState(true, 3)
	var lastEvent MTRSessionEvent
	var final MTRSnapshot
	prober := &mockTTLProber{probeFn: func(_ context.Context, ttl int) (mtrProbeResult, error) {
		return mtrProbeResult{
			TTL: ttl, Success: true, Addr: &net.IPAddr{IP: net.IPv4(192, 0, 2, byte(ttl))},
			RTT:      time.Duration(ttl) * 1234567,
			Response: &MTRProbeResponse{Kind: MTRResponseTransit, Description: "transit"},
		}, nil
	}}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	err := runMTRScheduler(ctx, prober, NewMTRAggregator(), mtrSchedulerConfig{
		BeginHop: 1, MaxHops: 3, MaxPerHop: 3, ParallelRequests: 1, HopInterval: time.Millisecond,
		OnEvent: func(event MTRSessionEvent) error { lastEvent = event; return state.Apply(event) },
	}, func(iteration int, stats []MTRHopStat) { final = MTRSnapshot{Iteration: iteration, Stats: stats} }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Snapshot(); !reflect.DeepEqual(got, final) {
		t.Fatalf("bounded replay = %+v, live = %+v", got, final)
	}
	if lastEvent.Type != MTRSessionPathEndEvent || state.PathEnd().Reason != StopReasonMaxHops {
		t.Fatalf("natural completion did not record max_hops: %+v", lastEvent)
	}
}

func TestMTRSessionPauseCountsInFlightAndDiscardsDisabledResults(t *testing.T) {
	paused := true
	state := NewMTRReplayState(false, 3)
	var types []string
	rt, err := newMTRSchedulerRuntime(t.Context(), &mockTTLProber{}, NewMTRAggregator(), mtrSchedulerConfig{
		BeginHop: 1, MaxHops: 3, ParallelRequests: 1,
		IsPaused: func() bool { return paused },
		OnEvent:  func(event MTRSessionEvent) error { types = append(types, event.Type); return state.Apply(event) },
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.workers.shutdown(nil)
	rt.inFlight = 1
	rt.scheduleReady()
	rt.processProbeSuccess(2, mtrProbeResult{
		TTL: 2, Success: true, Addr: &net.IPAddr{IP: net.ParseIP("192.0.2.2")},
		Response: &MTRProbeResponse{Kind: MTRResponseDestination},
	}, time.Now())
	if !state.Paused() || state.Snapshot().Stats[0].Snt != 1 {
		t.Fatal("pause discarded an in-flight result")
	}
	before := len(types)
	rt.states[3].inFlightCount = 1
	rt.processResult(mtrCompletedProbe{ttl: 3, result: mtrProbeResult{TTL: 3}})
	if len(types) != before {
		t.Fatal("disabled TTL produced a session event")
	}
	rt.inFlight = 1
	paused = false
	rt.scheduleReady()
	if state.Paused() || !reflect.DeepEqual(types, []string{MTRSessionPauseEvent, MTRSessionProbeEvent, MTRSessionPathEndEvent, MTRSessionResumeEvent}) {
		t.Fatalf("pause/resume sequence = %v", types)
	}
}

func TestMTRSessionRecordsAppliedProbeDuringCancellation(t *testing.T) {
	ClearCaches()
	defer ClearCaches()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	state := NewMTRReplayState(true, 1)
	var final MTRSnapshot
	var events []MTRSessionEvent
	prober := &mockTTLProber{probeFn: func(context.Context, int) (mtrProbeResult, error) {
		return mtrProbeResult{
			TTL: 1, Success: true, Addr: &net.IPAddr{IP: net.ParseIP("9.9.9.243")}, RTT: time.Millisecond,
			Response: &MTRProbeResponse{Kind: MTRResponseDestination},
		}, nil
	}}
	err := runMTRScheduler(ctx, prober, NewMTRAggregator(), mtrSchedulerConfig{
		BeginHop: 1, MaxHops: 1, MaxPerHop: 1, ParallelRequests: 1, FillGeo: true,
		BaseConfig: Config{IPGeoSource: func(string, time.Duration, string, bool) (*ipgeo.IPGeoData, error) {
			// Ctrl-C can arrive after a probe completed but before synchronous
			// enrichment returns to the scheduler's accounting code.
			cancel()
			return &ipgeo.IPGeoData{Asnumber: "64500"}, nil
		}},
		OnEvent: func(event MTRSessionEvent) error { events = append(events, event); return state.Apply(event) },
	}, func(iteration int, stats []MTRHopStat) { final = MTRSnapshot{Iteration: iteration, Stats: stats} }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	if len(final.Stats) != 1 || final.Stats[0].Snt != 1 {
		t.Fatalf("expected counted in-flight probe: %+v", final)
	}
	if !reflect.DeepEqual(state.Snapshot(), final) {
		t.Fatalf("cancel lost applied probe: replay=%+v live=%+v events=%+v", state.Snapshot(), final, events)
	}
	if len(events) != 2 || events[0].Type != MTRSessionProbeEvent || events[1].Type != MTRSessionPathEndEvent {
		t.Fatalf("cancel lost causal event order: %+v", events)
	}
}

func TestMTRSessionRecordsAppliedMetadataDuringCancellation(t *testing.T) {
	state := NewMTRReplayState(false, 1)
	rt, err := newMTRSchedulerRuntime(t.Context(), &mockTTLProber{}, NewMTRAggregator(), mtrSchedulerConfig{
		BeginHop: 1, MaxHops: 1,
		OnEvent: state.Apply,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.workers.shutdown(nil)
	rt.processProbeSuccess(1, mtrProbeResult{TTL: 1, Success: true, Addr: &net.IPAddr{IP: net.ParseIP("192.0.2.1")}}, time.Now())
	// Reproduce cancellation racing with an already selected metadata result.
	rt.workers.cancel(context.Canceled)
	rt.processMetadataResult(mtrMetadataResult{gen: 0, kind: mtrMetadataKindHost, patch: mtrMetadataPatch{ip: "192.0.2.1", host: "router.example"}})
	if !reflect.DeepEqual(state.Snapshot().Stats, rt.snapshotStats()) || state.Snapshot().Stats[0].Host != "router.example" {
		t.Fatalf("cancel lost applied metadata: replay=%+v live=%+v", state.Snapshot(), rt.snapshotStats())
	}
}

func TestMTRReplayRejectsUnappliedMetadata(t *testing.T) {
	for _, scenario := range []string{"before_probe", "unknown_peer", "after_reset", "already_applied", "empty_patch"} {
		t.Run(scenario, func(t *testing.T) {
			state := NewMTRReplayState(false, 3)
			generation := uint64(0)
			patch := &MTRSessionMetadata{IP: "192.0.2.1", Host: "router.invalid"}
			if scenario != "before_probe" {
				for _, ttl := range []int{1, 2} {
					if err := state.Apply(MTRSessionEvent{Type: MTRSessionProbeEvent, Probe: &MTRSessionProbe{TTL: ttl, IP: patch.IP, Success: true}}); err != nil {
						t.Fatal(err)
					}
				}
			}
			switch scenario {
			case "unknown_peer":
				patch.IP = "192.0.2.2"
			case "after_reset":
				generation = 1
				if err := state.Apply(MTRSessionEvent{Type: MTRSessionResetEvent, Generation: generation}); err != nil {
					t.Fatal(err)
				}
			case "already_applied":
				if err := state.Apply(MTRSessionEvent{Type: MTRSessionMetadataEvent, Metadata: patch}); err != nil {
					t.Fatal(err)
				}
				for _, stat := range state.Snapshot().Stats {
					if stat.Host != patch.Host || stat.Snt != 1 {
						t.Fatalf("cross-TTL patch: %+v", stat)
					}
				}
			case "empty_patch":
				patch.Host = ""
			}
			before := state.Snapshot()
			if err := state.Apply(MTRSessionEvent{Type: MTRSessionMetadataEvent, Generation: generation, Metadata: patch}); err == nil {
				t.Fatal("accepted unapplied metadata")
			}
			if !reflect.DeepEqual(before, state.Snapshot()) {
				t.Fatal("rejected metadata changed statistics")
			}
		})
	}
}
