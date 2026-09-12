package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

type mtrReplaySeekResult struct {
	cursor *mtrReplayCursor
	err    error
}

type mtrReplayOutput struct {
	w   io.Writer
	err error
}

func (w *mtrReplayOutput) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}

func runMTRReplayTUI(parent context.Context, reader *mtrsession.Reader, current *mtrReplayCursor, header printer.MTRTUIHeader, duration time.Duration, complete, noSummary bool, stdout io.Writer) (runErr error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	ui := newMTRUI(cancel, header.DisplayMode)
	ui.columns = append([]printer.MTRColumn(nil), header.Columns...)
	ui.nameMode.Store(int32(header.NameMode))
	ui.disableMPLS.Store(header.DisableMPLS)
	ui.paused.Store(true)
	ui.replay = &mtrReplayControls{commands: make(chan mtrReplayCommand, 1), duration: duration}
	ui.replay.cursor.Store(int64(current.cursor))
	var snapshots mtrSnapshotStore
	publishMTRReplaySnapshot(&snapshots, current, duration, complete, false)
	ui.captureSnapshot = func() *printer.MTRSnapshot {
		snapshot := captureMTRDisplay(&snapshots, ui, header.ShowIPs)
		if snapshot != nil && snapshot.Session != nil && snapshot.Session.EffectiveParameters != nil {
			snapshot.Session.EffectiveParameters.Language = header.Lang
		}
		return snapshot
	}
	ui.Enter()
	defer func() {
		ui.Leave()
		if err := ui.closeSnapshotSaver(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		if !noSummary {
			if err := writeMTRExitSummary(stdout, ui.captureSnapshot(), runErr); err != nil {
				fmt.Fprintf(os.Stderr, "write MTR summary: %v\n", err)
			}
		}
	}()
	keysDone := make(chan struct{})
	go func() { defer close(keysDone); ui.ReadKeysLoop(ctx) }()
	defer func() { cancel(); <-keysDone }()
	var seekDone chan mtrReplaySeekResult
	var stopSeek context.CancelFunc
	var queuedSeek *time.Duration
	playing := false
	output := &mtrReplayOutput{w: stdout}
	baseCursor, baseTime := current.cursor, time.Now()
	beginSeek := func(at time.Duration) {
		seekCtx, stop := context.WithCancel(ctx)
		stopSeek = stop
		seekDone = make(chan mtrReplaySeekResult, 1)
		result := seekDone
		go func() { next, err := readMTRReplay(seekCtx, reader, at); result <- mtrReplaySeekResult{next, err} }()
	}
	defer func() {
		if stopSeek != nil {
			stopSeek()
			<-seekDone
		}
	}()
	render := func() error {
		h := header
		h.Now = header.StartTime.Add(current.cursor)
		h.HistoryNow = h.Now
		h.HistoryMode = ui.IsHistoryMode()
		h.HistoryChartMode = ui.CurrentHistoryChartMode()
		if h.HistoryMode {
			h.History = current.history.Snapshot(h.Now)
		}
		h.DisplayMode = ui.CurrentDisplayMode()
		h.NameMode = ui.CurrentNameMode()
		h.DisableMPLS = ui.IsMPLSDisabled()
		h.Status = printer.MTRTUIPaused
		if playing && !ui.IsPaused() {
			h.Status = printer.MTRTUIRunning
		}
		h.Replay = &printer.MTRReplayStatus{Cursor: current.cursor, Duration: duration, Complete: complete, Seeking: seekDone != nil, RecordedPaused: current.state.Paused()}
		ui.columnsMu.Lock()
		h.Columns = append([]printer.MTRColumn(nil), ui.columns...)
		h.ColumnEditor = ui.columnEditor
		h.ReplayEditor = ui.replayEditor
		ui.columnsMu.Unlock()
		h.SaveDialog, h.HelpDialog = ui.dialogSnapshot()
		editing := h.ColumnEditor.Active || h.ReplayEditor.Active || h.SaveDialog.Active || h.HelpDialog.Active
		// The sink retains the first error; render returns it after the frame.
		if editing {
			_, _ = fmt.Fprint(output, "\033[?2004h")
		} else {
			_, _ = fmt.Fprint(output, "\033[?2004l")
		}
		snapshot := current.state.Snapshot()
		h.Iteration = snapshot.Iteration
		publishMTRReplaySnapshot(&snapshots, current, duration, complete, playing && !ui.IsPaused())
		printer.MTRTUIRender(output, h, sanitizeMTRReplayStats(snapshot.Stats))
		return output.err
	}
	if err := render(); err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case command := <-ui.replay.commands:
			switch command.kind {
			case "pause":
				playing = false
			case "play":
				playing = true
				baseCursor, baseTime = current.cursor, time.Now()
				if seekDone == nil && current.cursor >= duration {
					beginSeek(0)
				}
			case "seek":
				playing = command.playAfter
				if seekDone != nil {
					at := command.at
					queuedSeek = &at
					stopSeek()
				} else {
					beginSeek(command.at)
				}
			}
			if err := render(); err != nil {
				return err
			}
		case result := <-seekDone:
			stopSeek()
			stopSeek = nil
			seekDone = nil
			if queuedSeek != nil {
				at := *queuedSeek
				queuedSeek = nil
				beginSeek(at)
				if err := render(); err != nil {
					return err
				}
				continue
			}
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) {
					continue
				}
				return result.err
			}
			current = result.cursor
			ui.replay.cursor.Store(int64(current.cursor))
			baseCursor, baseTime = current.cursor, time.Now()
			if err := render(); err != nil {
				return err
			}
		case <-ui.redraw:
			if err := render(); err != nil {
				return err
			}
		case <-ticker.C:
			if playing && !ui.IsPaused() && seekDone == nil {
				at := min(duration, baseCursor+time.Since(baseTime))
				if err := current.advance(ctx, reader, at); err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					return err
				}
				ui.replay.cursor.Store(int64(current.cursor))
				if current.cursor >= duration {
					playing = false
					ui.paused.Store(true)
				}
			}
			if err := render(); err != nil {
				return err
			}
		}
	}
}

func sanitizeMTRReplayText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, value)
}

// Recordings may be shared by an untrusted sender. Clean only display copies;
// the reducer and JSON export retain the recorded values.
func sanitizeMTRReplayStats(stats []trace.MTRHopStat) []trace.MTRHopStat {
	result := cloneMTRDisplayStats(stats)
	for i := range result {
		s := &result[i]
		s.Host = sanitizeMTRReplayText(s.Host)
		s.IP = sanitizeMTRReplayText(s.IP)
		for j := range s.MPLS {
			s.MPLS[j] = sanitizeMTRReplayText(s.MPLS[j])
		}
		if s.Response != nil {
			s.Response.Kind = sanitizeMTRReplayText(s.Response.Kind)
			s.Response.Marker = sanitizeMTRReplayText(s.Response.Marker)
			s.Response.Description = sanitizeMTRReplayText(s.Response.Description)
		}
		if g := s.Geo; g != nil {
			for _, field := range []*string{&g.IP, &g.Asnumber, &g.Country, &g.CountryEn, &g.Prov, &g.ProvEn, &g.City, &g.CityEn, &g.District, &g.Owner, &g.Isp, &g.Domain, &g.Whois, &g.Prefix, &g.Source} {
				*field = sanitizeMTRReplayText(*field)
			}
		}
	}
	return result
}
