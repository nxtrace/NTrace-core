//go:build windows && amd64

package internal

import (
	"bytes"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"

	wd "github.com/xjasonlyu/windivert-go"
	"golang.org/x/sys/windows"
)

func resetWinDivertPreloadTestState() func() {
	oldResolve := resolveWinDivertExecutablePath
	oldLoad := loadWinDivertDLLEx
	oldHandle := preloadedWinDivertDLL
	preloadedWinDivertDLL = 0
	return func() {
		resolveWinDivertExecutablePath = oldResolve
		loadWinDivertDLLEx = oldLoad
		preloadedWinDivertDLL = oldHandle
	}
}

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

func TestCheckWinDivertDLLPreloadsExecutableDirectoryDLL(t *testing.T) {
	restore := resetWinDivertPreloadTestState()
	defer restore()
	resolveWinDivertExecutablePath = func() (string, error) {
		return `C:\nexttrace\bin\nexttrace.exe`, nil
	}
	var gotPath string
	var gotFlags uintptr
	loadWinDivertDLLEx = func(path string, flags uintptr) (windows.Handle, error) {
		gotPath = path
		gotFlags = flags
		return windows.Handle(1234), nil
	}

	if err := checkWinDivertDLL(); err != nil {
		t.Fatalf("checkWinDivertDLL() error = %v", err)
	}

	wantPath := filepath.Join(`C:\nexttrace\bin`, winDivertDLLName)
	if gotPath != wantPath {
		t.Fatalf("preload path = %q, want %q", gotPath, wantPath)
	}
	wantFlags := uintptr(windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | windows.LOAD_LIBRARY_SEARCH_APPLICATION_DIR)
	if gotFlags != wantFlags {
		t.Fatalf("preload flags = %#x, want %#x", gotFlags, wantFlags)
	}
	if preloadedWinDivertDLL != windows.Handle(1234) {
		t.Fatalf("preloadedWinDivertDLL = %v, want 1234", preloadedWinDivertDLL)
	}
}

func TestCheckWinDivertDLLRetriesAfterFailure(t *testing.T) {
	restore := resetWinDivertPreloadTestState()
	defer restore()
	resolveWinDivertExecutablePath = func() (string, error) {
		return `C:\nexttrace\bin\nexttrace.exe`, nil
	}
	attempts := 0
	loadWinDivertDLLEx = func(path string, flags uintptr) (windows.Handle, error) {
		attempts++
		if attempts == 1 {
			return 0, windows.ERROR_FILE_NOT_FOUND
		}
		return windows.Handle(5678), nil
	}

	if err := checkWinDivertDLL(); err == nil {
		t.Fatal("checkWinDivertDLL() error = nil, want preload failure")
	}
	if attempts != 1 {
		t.Fatalf("attempts after first check = %d, want 1", attempts)
	}
	if preloadedWinDivertDLL != 0 {
		t.Fatalf("preloadedWinDivertDLL = %v, want 0 after failed preload", preloadedWinDivertDLL)
	}

	if err := checkWinDivertDLL(); err != nil {
		t.Fatalf("checkWinDivertDLL() second error = %v, want success", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts after second check = %d, want 2", attempts)
	}
	if preloadedWinDivertDLL != windows.Handle(5678) {
		t.Fatalf("preloadedWinDivertDLL = %v, want 5678 after successful retry", preloadedWinDivertDLL)
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
		{
			name: "dll missing absolute path",
			err: &windows.DLLError{
				Err:     windows.ERROR_MOD_NOT_FOUND,
				ObjName: `C:\nexttrace\bin\WinDivert.dll`,
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

func TestFormatWinDivertIssuePrefersWrappedKind(t *testing.T) {
	msg := formatWinDivertIssue(&winDivertError{
		Kind:  winDivertErrorDriverBlocked,
		Cause: wd.Error(windows.ERROR_ACCESS_DENIED),
	})
	if !strings.Contains(msg, "被系统、安全软件或虚拟化环境拦截") {
		t.Fatalf("message = %q, want driver-blocked wording", msg)
	}
	if strings.Contains(msg, "管理员权限") {
		t.Fatalf("message = %q, should not fall back to access-denied classification", msg)
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
