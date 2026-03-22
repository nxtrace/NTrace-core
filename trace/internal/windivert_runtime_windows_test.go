//go:build windows && amd64

package internal

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	wd "github.com/xjasonlyu/windivert-go"
	"golang.org/x/sys/windows"
)

func TestOpenWinDivertHandle_CatchesDLLLoadPanic(t *testing.T) {
	oldCheck := checkWinDivertDLL
	oldOpen := openWinDivertCall
	checkWinDivertDLL = func() error { return nil }
	openWinDivertCall = func(string, uint64) (wd.Handle, error) {
		panic(&windows.DLLError{
			Err:     windows.ERROR_MOD_NOT_FOUND,
			ObjName: winDivertDLLName,
			Msg:     "Failed to load WinDivert.dll: The specified module could not be found.",
		})
	}
	defer func() {
		checkWinDivertDLL = oldCheck
		openWinDivertCall = oldOpen
	}()

	_, err := OpenWinDivertHandle("false", 0)
	if err == nil {
		t.Fatal("OpenWinDivertHandle() error = nil, want non-nil")
	}
	var wrapped *winDivertError
	if !errors.As(err, &wrapped) {
		t.Fatalf("OpenWinDivertHandle() error = %T, want *winDivertError", err)
	}
	if wrapped.Kind != winDivertErrorDLLMissing {
		t.Fatalf("wrapped.Kind = %v, want %v", wrapped.Kind, winDivertErrorDLLMissing)
	}
}

func TestDetectWinDivertErrorKind(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want winDivertErrorKind
	}{
		{
			name: "dll missing",
			err: &windows.DLLError{
				Err:     windows.ERROR_MOD_NOT_FOUND,
				ObjName: winDivertDLLName,
				Msg:     "Failed to load WinDivert.dll",
			},
			want: winDivertErrorDLLMissing,
		},
		{name: "driver missing", err: wd.Error(windows.ERROR_FILE_NOT_FOUND), want: winDivertErrorDriverMissing},
		{name: "admin required", err: wd.Error(windows.ERROR_ACCESS_DENIED), want: winDivertErrorAdminRequired},
		{name: "driver blocked", err: wd.Error(windows.ERROR_DRIVER_BLOCKED), want: winDivertErrorDriverBlocked},
		{name: "driver signature", err: wd.Error(windows.ERROR_INVALID_IMAGE_HASH), want: winDivertErrorDriverSignature},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectWinDivertErrorKind(tt.err); got != tt.want {
				t.Fatalf("detectWinDivertErrorKind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatWinDivertRequiredError_IncludesInitHint(t *testing.T) {
	msg := formatWinDivertRequiredError("Windows TCP 探测", wd.Error(windows.ERROR_FILE_NOT_FOUND))
	if !strings.Contains(msg, "--init") {
		t.Fatalf("message missing --init hint: %q", msg)
	}
	if !strings.Contains(msg, "可执行文件目录") {
		t.Fatalf("message missing executable-directory hint: %q", msg)
	}
}

func TestICMPResolveMode_AutoFallsBackToSocketWhenWinDivertUnavailable(t *testing.T) {
	oldAdmin := hasWindowsAdminPrivileges
	oldAvailable := detectWinDivertAvailability
	hasWindowsAdminPrivileges = func() bool { return true }
	detectWinDivertAvailability = func() (bool, error) {
		return false, wd.Error(windows.ERROR_FILE_NOT_FOUND)
	}
	defer func() {
		hasWindowsAdminPrivileges = oldAdmin
		detectWinDivertAvailability = oldAvailable
	}()

	spec := &ICMPSpec{ICMPMode: 0}
	if got := spec.resolveICMPMode(); got != 1 {
		t.Fatalf("resolveICMPMode() = %d, want 1", got)
	}
}

func TestICMPResolveMode_ForcedWinDivertLogsInitHintAndFallsBack(t *testing.T) {
	oldAdmin := hasWindowsAdminPrivileges
	oldAvailable := detectWinDivertAvailability
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	hasWindowsAdminPrivileges = func() bool { return true }
	detectWinDivertAvailability = func() (bool, error) {
		return false, wd.Error(windows.ERROR_FILE_NOT_FOUND)
	}
	defer func() {
		hasWindowsAdminPrivileges = oldAdmin
		detectWinDivertAvailability = oldAvailable
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}()

	spec := &ICMPSpec{ICMPMode: 2}
	if got := spec.resolveICMPMode(); got != 1 {
		t.Fatalf("resolveICMPMode() = %d, want 1", got)
	}
	if !strings.Contains(buf.String(), "--init") {
		t.Fatalf("forced fallback log missing --init hint: %q", buf.String())
	}
}
