package cmd

import (
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nxtrace/NTrace-core/printer"
)

type mtrReplayCommand struct {
	kind      string
	at        time.Duration
	playAfter bool
}

type mtrReplayControls struct {
	commands chan mtrReplayCommand
	cursor   atomic.Int64
	duration time.Duration
}

func (r *mtrReplayControls) send(command mtrReplayCommand) {
	// Keep a pending seek when a play/pause key immediately follows Enter.
	select {
	case r.commands <- command:
		return
	default:
	}
	select {
	case previous := <-r.commands:
		if previous.kind == "seek" && command.kind != "seek" {
			previous.playAfter = command.kind == "play"
			command = previous
		}
	default:
	}
	select {
	case r.commands <- command:
	default:
	}
}

func (u *mtrUI) isMTREditing() bool {
	if u.isMTRDialogActive() {
		return true
	}
	u.columnsMu.Lock()
	defer u.columnsMu.Unlock()
	return u.columnEditor.Active || u.replayEditor.Active
}

func (u *mtrUI) editMTRInput(b byte, paste bool) {
	if u.editMTRDialog(b, paste) {
		return
	}
	u.columnsMu.Lock()
	replay := u.replayEditor.Active
	u.columnsMu.Unlock()
	if replay {
		u.editReplayTime(b, paste)
	} else {
		u.editColumn(b, paste)
	}
}

func (u *mtrUI) applyReplayAction(action mtrInputAction) bool {
	switch action {
	case mtrActionPause:
		u.paused.Store(true)
		u.replay.send(mtrReplayCommand{kind: "pause"})
	case mtrActionResume:
		u.paused.Store(false)
		u.replay.send(mtrReplayCommand{kind: "play"})
	case mtrActionRestart:
		u.paused.Store(true)
		u.replay.send(mtrReplayCommand{kind: "seek"})
	case mtrActionReplayJump:
		u.paused.Store(true)
		u.replay.send(mtrReplayCommand{kind: "pause"})
		u.columnsMu.Lock()
		u.replayEditor = printer.MTRReplayEditor{Active: true, Draft: printer.FormatMTRReplayTime(time.Duration(u.replay.cursor.Load()))}
		u.columnsMu.Unlock()
	default:
		return false
	}
	return true
}

func (u *mtrUI) editReplayTime(b byte, paste bool) {
	u.columnsMu.Lock()
	defer u.columnsMu.Unlock()
	e := &u.replayEditor
	if !e.Active {
		return
	}
	if !paste {
		switch b {
		case 27:
			e.Active = false
			return
		case 8, 127:
			if len(e.Draft) > 0 {
				e.Draft = e.Draft[:len(e.Draft)-1]
			}
			e.Error = ""
			return
		case 21:
			e.Draft = ""
			e.Error = ""
			return
		case '\r', '\n':
			at, err := parseMTRReplayTime(e.Draft)
			if err != nil {
				e.Error = err.Error()
				return
			}
			if at > u.replay.duration {
				e.Error = "time exceeds recording duration"
				return
			}
			e.Active = false
			e.Error = ""
			u.replay.send(mtrReplayCommand{kind: "seek", at: at})
			return
		}
	}
	if b >= 32 && b <= 126 {
		if len(e.Draft) >= 32 {
			e.Error = "maximum 32 characters"
			return
		}
		e.Draft += string(b)
		e.Error = ""
	}
}

func parseMTRReplayTime(value string) (time.Duration, error) {
	invalid := errors.New("use HH:MM:SS[.mmm]")
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 3 || len(parts[0]) < 2 || len(parts[1]) != 2 {
		return 0, invalid
	}
	seconds := strings.Split(parts[2], ".")
	if len(seconds) > 2 || len(seconds[0]) != 2 {
		return 0, invalid
	}
	if len(seconds) == 2 && (len(seconds[1]) == 0 || len(seconds[1]) > 3) {
		return 0, invalid
	}
	for _, p := range []string{parts[0], parts[1], strings.Join(seconds, "")} {
		for _, c := range p {
			if c < '0' || c > '9' {
				return 0, invalid
			}
		}
	}
	h, e1 := strconv.ParseUint(parts[0], 10, 64)
	m, e2 := strconv.ParseUint(parts[1], 10, 64)
	s, e3 := strconv.ParseUint(seconds[0], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || m >= 60 || s >= 60 || h > uint64((1<<63-1)/int64(time.Hour)) {
		return 0, invalid
	}
	ms := uint64(0)
	if len(seconds) == 2 {
		ms, _ = strconv.ParseUint(seconds[1]+strings.Repeat("0", 3-len(seconds[1])), 10, 64)
	}
	ns := h*uint64(time.Hour) + m*uint64(time.Minute) + s*uint64(time.Second) + ms*uint64(time.Millisecond)
	if ns > 1<<63-1 {
		return 0, invalid
	}
	return time.Duration(ns), nil
}
