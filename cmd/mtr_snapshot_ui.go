package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nxtrace/NTrace-core/printer"
	"golang.org/x/term"
)

const mtrSnapshotPathLimit = 4096

type mtrSnapshotJob struct {
	path, format string
	snapshot     *printer.MTRSnapshot
}

// dialogSnapshot is consumed by the renderer. A returned failure no longer
// needs to be repeated after the alternate screen has been restored.
func (u *mtrUI) dialogSnapshot() (printer.MTRSaveDialog, printer.MTRHelpDialog) {
	u.columnsMu.Lock()
	otherEditor := u.columnEditor.Active || u.replayEditor.Active
	u.columnsMu.Unlock()
	u.dialogsMu.Lock()
	defer u.dialogsMu.Unlock()
	if u.saveDialog.Error != "" && (u.saveDialog.Active || (!u.helpDialog.Active && !otherEditor)) {
		u.snapshotError = nil
	}
	return u.saveDialog, u.helpDialog
}

func (u *mtrUI) isMTRDialogActive() bool {
	u.dialogsMu.Lock()
	defer u.dialogsMu.Unlock()
	return u.saveDialog.Active || u.helpDialog.Active
}

func (u *mtrUI) isMTRHelpActive() bool {
	u.dialogsMu.Lock()
	defer u.dialogsMu.Unlock()
	return u.helpDialog.Active
}

func (u *mtrUI) openSnapshotDialog() {
	var snapshot *printer.MTRSnapshot
	if u.captureSnapshot != nil {
		snapshot = u.captureSnapshot()
	}
	u.dialogsMu.Lock()
	defer u.dialogsMu.Unlock()
	if u.snapshotClosed {
		return
	}
	// An in-flight write retains its original snapshot even after Esc.
	if u.saveDialog.Busy {
		u.saveDialog.Active = true
		return
	}
	u.saveSnapshot = snapshot
	u.saveUTF8 = nil
	u.saveDialog = printer.MTRSaveDialog{Active: true, Format: "txt"}
	if snapshot == nil {
		u.saveDialog.Error = "No snapshot available"
		return
	}
	u.saveDialog.CapturedAt = snapshot.CapturedAt
	u.saveDialog.Path = printer.DefaultMTRSnapshotFilename(*snapshot, "txt")
}

func (u *mtrUI) openHelpDialog() {
	u.dialogsMu.Lock()
	u.helpDialog = printer.MTRHelpDialog{Active: true}
	u.dialogsMu.Unlock()
}

func (u *mtrUI) applyMTRDialogAction(action mtrInputAction) bool {
	u.dialogsMu.Lock()
	defer u.dialogsMu.Unlock()
	if u.helpDialog.Active {
		width, height := 80, 24
		if u.terminalSize != nil {
			width, height = u.terminalSize()
		} else if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			width, height = w, h
		}
		limit := printer.MTRHelpMaxOffset(u.replay != nil, width, height)
		u.helpDialog.Offset = min(u.helpDialog.Offset, limit)
		switch action {
		case mtrActionUp:
			u.helpDialog.Offset = max(0, u.helpDialog.Offset-1)
		case mtrActionDown:
			u.helpDialog.Offset = min(limit, u.helpDialog.Offset+1)
		}
		return true
	}
	if u.saveDialog.Active {
		if !u.saveDialog.Busy && u.saveDialog.Field == 0 {
			switch action {
			case mtrActionLeft:
				u.cycleSnapshotFormat(-1)
			case mtrActionRight:
				u.cycleSnapshotFormat(1)
			}
		}
		return true
	}
	return false
}

func (u *mtrUI) editMTRDialog(b byte, paste bool) bool {
	u.dialogsMu.Lock()
	defer u.dialogsMu.Unlock()
	if u.helpDialog.Active {
		if !paste && (b == '?' || b == 27) {
			u.helpDialog.Active = false
		}
		return true
	}
	if !u.saveDialog.Active {
		return false
	}
	if !paste && b == 27 {
		u.saveDialog.Active = false
		u.saveUTF8 = nil
		return true
	}
	if u.saveDialog.Busy {
		return true
	}
	if !paste {
		switch b {
		case '\t':
			u.saveDialog.Field = 1 - u.saveDialog.Field
			u.saveUTF8 = nil
			return true
		case '\r', '\n':
			u.submitSnapshot()
			return true
		case ' ':
			if u.saveDialog.Field == 0 {
				u.cycleSnapshotFormat(1)
				return true
			}
		}
	}
	if u.saveDialog.Field == 1 {
		u.editSnapshotPath(b, paste)
	}
	return true
}

// The caller holds dialogsMu for all save-dialog editing helpers.
func (u *mtrUI) cycleSnapshotFormat(delta int) {
	formats := []string{"txt", "json", "html"}
	index := 0
	for i, format := range formats {
		if format == u.saveDialog.Format {
			index = i
		}
	}
	old := u.saveDialog.Format
	u.saveDialog.Format = formats[(index+delta+len(formats))%len(formats)]
	if filepath.Ext(u.saveDialog.Path) == "."+old {
		u.saveDialog.Path = strings.TrimSuffix(u.saveDialog.Path, "."+old) + "." + u.saveDialog.Format
	}
	u.saveDialog.Error, u.saveDialog.Notice = "", ""
}

func (u *mtrUI) editSnapshotPath(b byte, paste bool) {
	e := &u.saveDialog
	if !paste {
		switch b {
		case 8, 127:
			if len(u.saveUTF8) != 0 {
				u.saveUTF8 = nil
			} else if e.Path != "" {
				_, size := utf8.DecodeLastRuneInString(e.Path)
				e.Path = e.Path[:len(e.Path)-size]
			}
			e.Error, e.Notice = "", ""
			return
		case 21:
			e.Path, e.Error, e.Notice = "", "", ""
			u.saveUTF8 = nil
			return
		}
	} else if b == '\r' || b == '\n' || b == '\t' {
		b = ' '
	}
	if b < 32 || b == 127 {
		u.saveUTF8 = nil
		return
	}
	u.saveUTF8 = append(u.saveUTF8, b)
	for utf8.FullRune(u.saveUTF8) {
		r, size := utf8.DecodeRune(u.saveUTF8)
		u.saveUTF8 = u.saveUTF8[size:]
		if (r == utf8.RuneError && size == 1) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if len(e.Path)+size > mtrSnapshotPathLimit {
			e.Error = "Path exceeds 4096 bytes"
			continue
		}
		e.Path += string(r)
		e.Error, e.Notice = "", ""
	}
}

func (u *mtrUI) submitSnapshot() {
	e := &u.saveDialog
	if u.saveSnapshot == nil {
		e.Error = "No snapshot available"
		return
	}
	if strings.TrimSpace(e.Path) == "" {
		e.Error = "Enter a file path"
		return
	}
	if len(u.saveUTF8) != 0 {
		e.Error = "Incomplete path character"
		return
	}
	if u.snapshotClosed {
		return
	}
	if u.snapshotJobs == nil {
		u.snapshotJobs = make(chan mtrSnapshotJob, 1)
		u.snapshotDone = make(chan struct{})
		go u.runSnapshotSaver()
	}
	e.Busy = true
	e.Error, e.Notice = "", ""
	u.snapshotError = nil
	u.snapshotJobs <- mtrSnapshotJob{path: e.Path, format: e.Format, snapshot: u.saveSnapshot}
}

func (u *mtrUI) runSnapshotSaver() {
	defer close(u.snapshotDone)
	write := u.writeSnapshot
	if write == nil {
		write = writeMTRSnapshotFile
	}
	for job := range u.snapshotJobs {
		err := write(job.path, job.snapshot, job.format)
		u.dialogsMu.Lock()
		u.saveDialog.Busy = false
		if err != nil {
			u.snapshotError = fmt.Errorf("save report: %w", err)
			u.saveDialog.Error = mtrSnapshotSaveError(err)
		} else {
			u.saveDialog.Notice = "Saved: " + job.path
		}
		u.dialogsMu.Unlock()
		u.requestRedraw()
	}
}

func mtrSnapshotSaveError(err error) string {
	cause := err
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		cause = pathError.Err
	}
	switch {
	case errors.Is(err, os.ErrExist):
		cause = os.ErrExist
	case errors.Is(err, os.ErrPermission):
		cause = os.ErrPermission
	}
	return "save report: " + cause.Error()
}

func (u *mtrUI) closeSnapshotSaver() error {
	u.dialogsMu.Lock()
	if !u.snapshotClosed {
		u.snapshotClosed = true
		if u.snapshotJobs != nil {
			close(u.snapshotJobs)
		}
	}
	done := u.snapshotDone
	u.dialogsMu.Unlock()
	if done != nil {
		<-done
	}
	u.dialogsMu.Lock()
	defer u.dialogsMu.Unlock()
	err := u.snapshotError
	u.snapshotError = nil
	return err
}

func writeMTRSnapshotFile(path string, snapshot *printer.MTRSnapshot, format string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	original, statErr := f.Stat()
	writeErr := printer.WriteMTRSnapshot(f, *snapshot, format)
	closeErr := f.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		// Remove only the incomplete file this task created, never a replacement.
		current, lstatErr := os.Lstat(path)
		if statErr == nil && lstatErr == nil && os.SameFile(original, current) {
			_ = os.Remove(path)
		}
	}
	return err
}
