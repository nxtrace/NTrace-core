package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nxtrace/NTrace-core/internal/mtrsession"
	"github.com/nxtrace/NTrace-core/printer"
	"github.com/nxtrace/NTrace-core/trace"
)

func snapshotForDialog() *printer.MTRSnapshot {
	return &printer.MTRSnapshot{
		CapturedAt: time.Date(2026, 9, 12, 10, 11, 12, 0, time.UTC),
		Source:     "live", State: "running",
		Session: &mtrsession.Session{Target: "example.com", Protocol: "icmp"},
		Stats:   []trace.MTRHopStat{{TTL: 1, IP: "192.0.2.1", Snt: 3}},
	}
}

func waitSnapshotResult(t *testing.T, u *mtrUI) printer.MTRSaveDialog {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		dialog, _ := u.dialogSnapshot()
		if !dialog.Busy {
			return dialog
		}
		select {
		case <-u.redraw:
		case <-timer.C:
			t.Fatal("snapshot writer did not finish")
		}
	}
}

func TestMTRSnapshotDialogCapturesOnceAndEditsUnicode(t *testing.T) {
	u := newMTRUI(nil, 0)
	var captures int
	u.captureSnapshot = func() *printer.MTRSnapshot { captures++; return snapshotForDialog() }
	u.paused.Store(true)
	u.historyMode.Store(true)
	var keys mtrKeyInput
	feedMTRKeys(u, &keys, "s")
	dialog, _ := u.dialogSnapshot()
	if !dialog.Active || dialog.Format != "txt" || !strings.HasSuffix(dialog.Path, ".txt") || dialog.CapturedAt != snapshotForDialog().CapturedAt {
		t.Fatal(dialog)
	}
	feedMTRKeys(u, &keys, "\x1b[C")
	dialog, _ = u.dialogSnapshot()
	if dialog.Format != "json" || !strings.HasSuffix(dialog.Path, ".json") {
		t.Fatal(dialog)
	}
	feedMTRKeys(u, &keys, "\x1bOD ") // SS3 left, then Space returns to JSON.
	feedMTRKeys(u, &keys, "\t\x15报告😀\x7f结果.json")
	dialog, _ = u.dialogSnapshot()
	if dialog.Path != "报告结果.json" || !utf8.ValidString(dialog.Path) || dialog.Format != "json" {
		t.Fatal(dialog)
	}
	feedMTRKeys(u, &keys, "qprynedgo?s")
	if captures != 1 || !u.IsPaused() || !u.IsHistoryMode() || u.ConsumeRestartRequest() || u.CurrentDisplayMode() != 0 || u.CurrentNameMode() != 0 || u.IsMPLSDisabled() {
		t.Fatal("path editing leaked shortcuts")
	}
	feedMTRKeys(u, &keys, "\x1b")
	keys.expireEscape(u, time.Now().Add(mtrEscapeDelay))
	dialog, _ = u.dialogSnapshot()
	if dialog.Active || !u.IsPaused() || !u.IsHistoryMode() {
		t.Fatal("cancel changed the view or pause state")
	}
}

func TestMTRSnapshotPasteCannotSubmitOrTriggerHelp(t *testing.T) {
	u := newMTRUI(nil, 0)
	u.captureSnapshot = snapshotForDialog
	var keys mtrKeyInput
	feedMTRKeys(u, &keys, "s\t\x15")
	for _, part := range []string{"\x1b[2", "00~", "中文?\r\n\t", "qpsoe\u202e.json", "\x1b[20", "1~"} {
		feedMTRKeys(u, &keys, part)
	}
	dialog, help := u.dialogSnapshot()
	if dialog.Path != "中文?   qpsoe.json" || dialog.Busy || !dialog.Active || help.Active {
		t.Fatalf("paste escaped path editor: %+v %+v", dialog, help)
	}
	feedMTRKeys(u, &keys, "\x15"+strings.Repeat("中", 1400))
	dialog, _ = u.dialogSnapshot()
	if len(dialog.Path) > mtrSnapshotPathLimit || !utf8.ValidString(dialog.Path) || dialog.Error == "" {
		t.Fatal("path limit split a rune or was not enforced")
	}
	var cancelled bool
	u.cancel = func() { cancelled = true }
	feedMTRKeys(u, &keys, "\x1b[200~\x03")
	if !cancelled {
		t.Fatal("paste swallowed Ctrl-C")
	}
}

func TestMTRHelpIsolationScrollingAndQuit(t *testing.T) {
	for _, replay := range []bool{false, true} {
		u := newMTRUI(nil, 0)
		if replay {
			u.replay = &mtrReplayControls{commands: make(chan mtrReplayCommand, 1)}
		}
		u.paused.Store(true)
		u.historyMode.Store(true)
		var keys mtrKeyInput
		feedMTRKeys(u, &keys, "?p ryneodgsj")
		_, help := u.dialogSnapshot()
		if !help.Active || !u.IsPaused() || !u.IsHistoryMode() || u.ConsumeRestartRequest() || u.CurrentDisplayMode() != 0 || u.CurrentNameMode() != 0 || u.IsMPLSDisabled() {
			t.Fatal("help changed session state")
		}
		if replay && len(u.replay.commands) > 0 {
			t.Fatal("help sent replay command")
		}
		feedMTRKeys(u, &keys, "\x1b[B")
		_, help = u.dialogSnapshot()
		if replay && help.Offset == 0 {
			t.Fatal("down arrow did not scroll")
		}
		feedMTRKeys(u, &keys, "\x1bOA")
		_, help = u.dialogSnapshot()
		if help.Offset != 0 {
			t.Fatal("SS3 up did not scroll back")
		}
		feedMTRKeys(u, &keys, "\x1b[200~?q\x1b[201~")
		if !u.isMTRHelpActive() {
			t.Fatal("pasted help shortcut was activated")
		}
		feedMTRKeys(u, &keys, "?")
		if u.isMTRHelpActive() || !u.IsPaused() {
			t.Fatal("help did not close cleanly")
		}
		feedMTRKeys(u, &keys, "?\x1b")
		keys.expireEscape(u, time.Now().Add(mtrEscapeDelay))
		if u.isMTRHelpActive() {
			t.Fatal("Esc did not close help")
		}
		var cancelled bool
		u.cancel = func() { cancelled = true }
		feedMTRKeys(u, &keys, "?")
		if !keys.feed(u, 'q', time.Now()) || !cancelled {
			t.Fatal("q did not quit from help")
		}
	}
}

func TestMTRSnapshotWriterIsSingleAndDoesNotBlockInput(t *testing.T) {
	u := newMTRUI(nil, 0)
	u.captureSnapshot = snapshotForDialog
	started := make(chan mtrSnapshotJob, 1)
	release := make(chan struct{})
	var writes atomic.Int32
	u.writeSnapshot = func(path string, s *printer.MTRSnapshot, format string) error {
		writes.Add(1)
		started <- mtrSnapshotJob{path: path, snapshot: s, format: format}
		<-release
		return errors.New("test write failure")
	}
	var keys mtrKeyInput
	feedMTRKeys(u, &keys, "s\r")
	select {
	case job := <-started:
		if job.snapshot.Stats[0].Snt != 3 || job.format != "txt" {
			t.Fatal(job)
		}
	case <-time.After(time.Second):
		t.Fatal("writer did not start")
	}
	feedMTRKeys(u, &keys, "\r\r\x1b")
	keys.expireEscape(u, time.Now().Add(mtrEscapeDelay))
	feedMTRKeys(u, &keys, "p?r")
	if !u.IsPaused() || !u.isMTRHelpActive() || u.ConsumeRestartRequest() || writes.Load() != 1 {
		t.Fatal("slow write blocked input or allowed concurrent writes")
	}
	closed := make(chan error, 1)
	go func() { closed <- u.closeSnapshotSaver() }()
	select {
	case <-closed:
		t.Fatal("shutdown did not wait for in-flight write")
	default:
	}
	close(release)
	select {
	case err := <-closed:
		if err == nil || !strings.Contains(err.Error(), "test write failure") {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish")
	}
	if err := u.closeSnapshotSaver(); err != nil {
		t.Fatal("repeated shutdown duplicated error", err)
	}
}

func TestMTRSnapshotSaveFailureRetryAndFileSafety(t *testing.T) {
	u := newMTRUI(nil, 0)
	u.captureSnapshot = snapshotForDialog
	t.Cleanup(func() { _ = u.closeSnapshotSaver() })
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existing, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	var keys mtrKeyInput
	feedMTRKeys(u, &keys, "s\t\x15"+existing+"\r")
	dialog := waitSnapshotResult(t, u)
	if !dialog.Active || dialog.Error == "" {
		t.Fatal(dialog)
	}
	if content, _ := os.ReadFile(existing); string(content) != "original" {
		t.Fatal("overwrote existing file")
	}
	path := filepath.Join(dir, "报告.txt")
	feedMTRKeys(u, &keys, "\x15"+path+"\r")
	dialog = waitSnapshotResult(t, u)
	if !strings.HasPrefix(dialog.Notice, "Saved: ") || dialog.Error != "" {
		t.Fatal(dialog)
	}
	info, err := os.Stat(path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatal(info, err)
	}
	if err := u.closeSnapshotSaver(); err != nil {
		t.Fatal("already rendered failure leaked to exit", err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link.txt")
		if err := os.Symlink(existing, link); err != nil {
			t.Fatal(err)
		}
		if err := writeMTRSnapshotFile(link, snapshotForDialog(), "txt"); !errors.Is(err, os.ErrExist) {
			t.Fatalf("symlink not refused: %v", err)
		}
	}
	partial := filepath.Join(dir, "bad.format")
	if err := writeMTRSnapshotFile(partial, snapshotForDialog(), "invalid"); err == nil {
		t.Fatal("unsupported format accepted")
	}
	if _, err := os.Stat(partial); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incomplete output was retained: %v", err)
	}
}

func TestMTRSnapshotNoDataAndReadOnlyDirectory(t *testing.T) {
	u := newMTRUI(nil, 0)
	var keys mtrKeyInput
	feedMTRKeys(u, &keys, "s\r")
	dialog, _ := u.dialogSnapshot()
	if dialog.Error != "No snapshot available" || dialog.Busy {
		t.Fatal(dialog)
	}
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	if err := writeMTRSnapshotFile(filepath.Join(dir, "report.txt"), snapshotForDialog(), "txt"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("read-only directory: %v", err)
	}
}

func TestMTRSnapshotSaveErrorKeepsCauseVisibleAndExitDetails(t *testing.T) {
	path := filepath.Join(strings.Repeat("long-directory/", 10), "report.txt")
	for _, cause := range []error{os.ErrExist, os.ErrPermission} {
		t.Run(cause.Error(), func(t *testing.T) {
			u := newMTRUI(nil, 0)
			u.captureSnapshot = snapshotForDialog
			u.writeSnapshot = func(string, *printer.MTRSnapshot, string) error {
				return &os.PathError{Op: "open", Path: path, Err: cause}
			}
			var keys mtrKeyInput
			feedMTRKeys(u, &keys, "s\r")
			exitErr := u.closeSnapshotSaver()
			if !errors.Is(exitErr, cause) || !strings.Contains(exitErr.Error(), path) {
				t.Fatalf("exit lost full error: %v", exitErr)
			}
			dialog, _ := u.dialogSnapshot()
			if dialog.Error != "save report: "+cause.Error() || len(dialog.Error) > 40 {
				t.Fatalf("40-column dialog lost cause: %q", dialog.Error)
			}
		})
	}
}
