package cmd

import (
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/printer"
)

const mtrColumnDraftLimit = 256
const mtrEscapeDelay = 50 * time.Millisecond

func (u *mtrUI) columnSnapshot() ([]printer.MTRColumn, printer.MTRColumnEditor) {
	u.columnsMu.Lock()
	defer u.columnsMu.Unlock()
	return append([]printer.MTRColumn(nil), u.columns...), u.columnEditor
}

func (u *mtrUI) requestRedraw() {
	select {
	case u.redraw <- struct{}{}:
	default:
	}
}

func (u *mtrUI) openColumnEditor() {
	u.columnsMu.Lock()
	defer u.columnsMu.Unlock()
	columns := u.columns
	if columns == nil {
		columns = printer.DefaultMTRColumns()
	}
	u.columnEditor = printer.MTRColumnEditor{Active: true, Draft: printer.MTRColumnCodes(columns)}
}

func (u *mtrUI) editColumn(b byte, paste bool) {
	u.columnsMu.Lock()
	defer u.columnsMu.Unlock()
	e := &u.columnEditor
	if !e.Active {
		return
	}
	if paste {
		if b == '\r' || b == '\n' || b == '\t' {
			b = ' '
		}
		if b < 32 || b > 126 {
			return
		}
	} else {
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
			columns, err := printer.ParseMTRColumns(e.Draft, true)
			if err != nil {
				e.Error = err.Error()
				return
			}
			u.columns = columns
			e.Active = false
			e.Error = ""
			u.historyMode.Store(false)
			return
		}
	}
	if b >= 32 && b <= 126 {
		if len(e.Draft) >= mtrColumnDraftLimit {
			e.Error = "Maximum 256 characters"
			return
		}
		e.Draft += string(b)
		e.Error = ""
	}
}

// Input state is owned by the key loop, independently of the renderer's snapshot.
type mtrKeyInput struct {
	parser   mtrInputParser
	escapeAt time.Time
	paste    bool
	pasteEnd string
}

func (k *mtrKeyInput) expireEscape(u *mtrUI, now time.Time) {
	if k.parser.state == mtrStateEsc && !k.escapeAt.IsZero() && now.Sub(k.escapeAt) >= mtrEscapeDelay {
		k.parser.state = mtrStateGround
		k.escapeAt = time.Time{}
		u.editMTRInput(27, false)
		u.requestRedraw()
	}
}

func (k *mtrKeyInput) feed(u *mtrUI, b byte, now time.Time) bool {
	if b == 3 {
		return u.applyInputAction(mtrActionQuit)
	}
	if k.paste {
		k.feedPaste(u, b)
		u.requestRedraw()
		return false
	}
	k.expireEscape(u, now)
	if k.parser.state == mtrStateGround && u.isMTRHelpActive() && (b == 'q' || b == 'Q') {
		return u.applyInputAction(mtrActionQuit)
	}
	editing := u.isMTREditing()
	k.parser.trackPaste = editing
	k.parser.trackArrows = u.isMTRDialogActive()
	if k.parser.state == mtrStateGround && b != 27 && editing {
		u.editMTRInput(b, false)
	} else {
		action := k.parser.Feed(b)
		if action == mtrActionPasteStart {
			k.paste = true
		} else if u.applyInputAction(action) {
			return true
		}
	}
	if k.parser.state == mtrStateEsc {
		if k.escapeAt.IsZero() {
			k.escapeAt = now
		}
	} else {
		k.escapeAt = time.Time{}
	}
	u.requestRedraw()
	return false
}

func (k *mtrKeyInput) feedPaste(u *mtrUI, b byte) {
	const end = "\x1b[201~"
	k.pasteEnd += string([]byte{b})
	for k.pasteEnd != "" && !strings.HasPrefix(end, k.pasteEnd) {
		u.editMTRInput(k.pasteEnd[0], true)
		k.pasteEnd = k.pasteEnd[1:]
	}
	if k.pasteEnd == end {
		k.paste = false
		k.pasteEnd = ""
	}
}

func (u *mtrUI) applyInputAction(action mtrInputAction) bool {
	if action != mtrActionQuit && u.applyMTRDialogAction(action) {
		return false
	}
	if u.replay != nil && u.applyReplayAction(action) {
		return false
	}
	switch action {
	case mtrActionQuit:
		if u.cancel != nil {
			u.cancel()
		}
		return true
	case mtrActionPause:
		u.paused.Store(true)
	case mtrActionResume:
		u.paused.Store(false)
	case mtrActionRestart:
		u.restartReq.Store(true)
	case mtrActionDisplayMode:
		u.CycleDisplayMode()
	case mtrActionNameToggle:
		u.ToggleNameMode()
	case mtrActionMPLSToggle:
		u.ToggleMPLS()
	case mtrActionHistoryToggle:
		u.ToggleHistoryMode()
	case mtrActionHistoryChart:
		u.CycleHistoryChartMode()
	case mtrActionColumns:
		u.openColumnEditor()
	case mtrActionSave:
		u.openSnapshotDialog()
	case mtrActionHelp:
		u.openHelpDialog()
	}
	return false
}
