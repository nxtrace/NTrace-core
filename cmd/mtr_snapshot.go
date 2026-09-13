package cmd

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

type mtrTUIOptions struct {
	NoSummary  bool
	PacketSize int
	DotServer  string
	Parameters *mtrsession.Parameters
}

// Published snapshots are immutable. Only the probe callback (or replay loop)
// writes; input and rendering may independently capture the last committed view.
type mtrSnapshotStore struct {
	mu     sync.Mutex
	latest *printer.MTRSnapshot
}

func (s *mtrSnapshotStore) publish(snapshot printer.MTRSnapshot) {
	next := cloneMTRSnapshot(&snapshot)
	s.mu.Lock()
	s.latest = next
	s.mu.Unlock()
}

func (s *mtrSnapshotStore) capture() *printer.MTRSnapshot {
	s.mu.Lock()
	latest := s.latest
	s.mu.Unlock()
	return cloneMTRSnapshot(latest)
}

func cloneMTRSnapshot(snapshot *printer.MTRSnapshot) *printer.MTRSnapshot {
	if snapshot == nil {
		return nil
	}
	copy := *snapshot
	copy.Stats = cloneMTRDisplayStats(snapshot.Stats)
	for i := range copy.Stats {
		if geo := copy.Stats[i].Geo; geo != nil && geo.Router != nil {
			routers := make(map[string][]string, len(geo.Router))
			for name, addresses := range geo.Router {
				routers[name] = slices.Clone(addresses)
			}
			geo.Router = routers
		}
	}
	if copy.Stats == nil {
		copy.Stats = []trace.MTRHopStat{}
	}
	copy.PathEnd = copyMTRPathEnd(snapshot.PathEnd)
	if snapshot.Session != nil {
		session := *snapshot.Session
		copy.Session = &session
		if p := session.EffectiveParameters; p != nil {
			params := *p
			params.Port = cloneMTRInt(p.Port)
			params.SourcePort = cloneMTRInt(p.SourcePort)
			params.ICMPMode = cloneMTRInt(p.ICMPMode)
			if p.FWMark != nil {
				mark := *p.FWMark
				params.FWMark = &mark
			}
			session.EffectiveParameters = &params
		}
	}
	if snapshot.Replay != nil {
		replay := *snapshot.Replay
		copy.Replay = &replay
	}
	return &copy
}

func cloneMTRInt(value *int) *int {
	if value == nil {
		return nil
	}
	return ptrInt(*value)
}

// Event and snapshot callbacks execute serially in scheduler order. Path and
// generation changes become visible to readers only with the next statistics.
type mtrLiveSnapshots struct {
	store      mtrSnapshotStore
	session    mtrsession.Session
	start      time.Time
	pathEnd    *trace.StopReason
	generation uint64
	paused     bool
}

func (s *mtrLiveSnapshots) event(event trace.MTRSessionEvent) {
	s.generation = event.Generation
	switch event.Type {
	case trace.MTRSessionResetEvent:
		s.pathEnd = nil
	case trace.MTRSessionPauseEvent:
		s.paused = true
	case trace.MTRSessionResumeEvent:
		s.paused = false
	}
}

func (s *mtrLiveSnapshots) snapshot(stats []trace.MTRHopStat) {
	now := time.Now()
	state := "running"
	if s.paused {
		state = "paused"
	}
	s.store.publish(printer.MTRSnapshot{
		SchemaVersion: 1, Type: "mtr_snapshot", Source: "live", CapturedAt: now.UTC(),
		ElapsedNS: max(int64(0), now.Sub(s.start).Nanoseconds()), Generation: s.generation,
		State: state, Session: &s.session, PathEnd: s.pathEnd, Stats: stats,
	})
}

func captureMTRDisplay(store *mtrSnapshotStore, ui *mtrUI, showIPs bool) *printer.MTRSnapshot {
	snapshot := store.capture()
	if snapshot == nil || snapshot.Session == nil {
		return snapshot
	}
	if snapshot.Source == "live" || snapshot.Source == "replay" {
		// Controls can change before the next probe snapshot or replay render.
		// Match the current TUI control state while retaining the data timestamp.
		snapshot.State = "running"
		if ui.IsPaused() {
			snapshot.State = "paused"
		}
	}
	columns, _ := ui.columnSnapshot()
	if columns == nil {
		columns = printer.DefaultMTRColumns()
	}
	snapshot.Session.Display = mtrsession.DisplayParameters{
		HostMode: ui.CurrentDisplayMode(), HostNameMode: ui.CurrentNameMode(),
		ShowIPs: showIPs, ShowMPLS: !ui.IsMPLSDisabled(), Columns: printer.MTRColumnNames(columns),
	}
	return snapshot
}

func buildMTRSnapshotParameters(method trace.Method, conf trace.Config, interval, count int, provider string, options mtrTUIOptions) *mtrsession.Parameters {
	if options.Parameters != nil {
		return options.Parameters
	}
	params := &mtrsession.Parameters{
		MaxPerHop: count, HopIntervalMs: interval, TimeoutMs: conf.Timeout.Milliseconds(),
		BeginHop: conf.BeginHop, MaxHops: conf.MaxHops, ParallelRequests: conf.ParallelRequests,
		SourceAddress: conf.SrcAddr, SourceDevice: conf.SourceDevice,
		PacketSize: options.PacketSize, RandomPacketSize: conf.RandomPacketSize, TOS: conf.TOS,
		DataProvider: provider, Language: conf.Lang, DotServer: options.DotServer,
		RDNS: conf.RDNS, AlwaysWaitRDNS: conf.RDNS && conf.AlwaysWaitRDNS,
		DisableMPLS: conf.DisableMPLS, DN42: conf.DN42,
	}
	if conf.FWMarkSet {
		mark := conf.FWMark
		params.FWMark = &mark
	}
	if method != trace.ICMPTrace {
		params.Port, params.SourcePort = ptrInt(conf.DstPort), ptrInt(conf.SrcPort)
	}
	if conf.OSType == 2 {
		params.ICMPMode = ptrInt(conf.ICMPMode)
	}
	return params
}

func publishMTRReplaySnapshot(store *mtrSnapshotStore, current *mtrReplayCursor, duration time.Duration, complete, playing bool) {
	state := "paused"
	if playing {
		state = "running"
	}
	store.publish(printer.MTRSnapshot{
		SchemaVersion: 1, Type: "mtr_snapshot", Source: "replay",
		CapturedAt: time.Now().UTC(), ElapsedNS: int64(current.cursor),
		Generation: current.state.Generation(), State: state, Session: current.session,
		PathEnd: current.state.PathEnd(), Stats: current.state.Snapshot().Stats,
		Replay: &printer.MTRSnapshotReplay{CursorNS: int64(current.cursor), DurationNS: int64(duration),
			RecordingComplete: complete, RecordedPaused: current.state.Paused()},
	})
}

func writeMTRExitSummary(w io.Writer, snapshot *printer.MTRSnapshot, runErr error) error {
	if snapshot == nil {
		return nil
	}
	if snapshot.Source == "live" {
		snapshot.State = "completed"
		if runErr != nil {
			snapshot.State = "error"
			var signal *mtrJSONSignal
			if errors.Is(runErr, context.Canceled) || errors.As(runErr, &signal) {
				snapshot.State = "interrupted"
			}
		}
	}
	return printer.WriteMTRSnapshot(w, *snapshot, "txt")
}
