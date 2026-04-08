//go:build windows && amd64

package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	wd "github.com/xjasonlyu/windivert-go"
	"golang.org/x/sys/windows"
)

func TestOpenWinDivertSniffHandlePanicsInDevMode(t *testing.T) {
	oldOpen := openWinDivertSniffCall
	oldFatal := winDivertSniffFatal
	oldDevMode := winDivertSniffDevMode
	openWinDivertSniffCall = func(string, uint64) (wd.Handle, error) {
		return 0, wd.Error(windows.ERROR_FILE_NOT_FOUND)
	}
	var gotFatal string
	winDivertSniffFatal = func(msg string) {
		gotFatal = msg
	}
	winDivertSniffDevMode = func() bool { return true }
	defer func() {
		openWinDivertSniffCall = oldOpen
		winDivertSniffFatal = oldFatal
		winDivertSniffDevMode = oldDevMode
	}()
	defer func() {
		if gotFatal != "" {
			t.Fatalf("fatal should not be called in dev mode: %s", gotFatal)
		}
	}()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("openWinDivertSniffHandle() did not panic in dev mode")
		} else if msg := fmt.Sprintf("%v", r); !strings.Contains(msg, "WinDivert") || !strings.Contains(msg, `filter="false"`) {
			t.Fatalf("panic = %q, want WinDivert context and filter", msg)
		}
	}()

	openWinDivertSniffHandle(context.Background(), "false", "test")
}

func TestOpenWinDivertSniffHandleCallsFatalOutsideDevModeThenPanics(t *testing.T) {
	oldOpen := openWinDivertSniffCall
	oldFatal := winDivertSniffFatal
	oldDevMode := winDivertSniffDevMode
	openWinDivertSniffCall = func(string, uint64) (wd.Handle, error) {
		return 0, errors.New("boom")
	}
	var gotFatal string
	winDivertSniffFatal = func(msg string) {
		gotFatal = msg
	}
	winDivertSniffDevMode = func() bool { return false }
	defer func() {
		openWinDivertSniffCall = oldOpen
		winDivertSniffFatal = oldFatal
		winDivertSniffDevMode = oldDevMode
	}()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("openWinDivertSniffHandle() did not panic after fatal hook")
		} else if msg := fmt.Sprintf("%v", r); !strings.Contains(msg, "Windows WinDivert 嗅探 (test") || !strings.Contains(msg, `filter="false"`) {
			t.Fatalf("panic = %q, want action context and filter", msg)
		}
		if gotFatal == "" {
			t.Fatal("fatal hook was not called")
		}
		if !strings.Contains(gotFatal, "Windows WinDivert 嗅探 (test") || !strings.Contains(gotFatal, `filter="false"`) {
			t.Fatalf("fatal message = %q, want action context and filter", gotFatal)
		}
	}()

	openWinDivertSniffHandle(context.Background(), "false", "test")
}
