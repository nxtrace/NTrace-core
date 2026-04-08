//go:build windows

package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	wd "github.com/xjasonlyu/windivert-go"
	"golang.org/x/sys/windows"
)

const winDivertDLLName = "WinDivert.dll"

type winDivertErrorKind int

const (
	winDivertErrorUnknown winDivertErrorKind = iota
	winDivertErrorDLLMissing
	winDivertErrorDLLLoadFailed
	winDivertErrorDriverMissing
	winDivertErrorAdminRequired
	winDivertErrorDriverBlocked
	winDivertErrorDriverSignature
	winDivertErrorBaseFilteringEngineDisabled
	winDivertErrorDriverVersionConflict
)

type winDivertError struct {
	Kind  winDivertErrorKind
	Cause error
}

func (e *winDivertError) Error() string {
	if e == nil || e.Cause == nil {
		return "windivert unavailable"
	}
	return e.Cause.Error()
}

func (e *winDivertError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

var (
	resolveWinDivertExecutablePath = os.Executable
	loadWinDivertDLLEx             = func(path string, flags uintptr) (windows.Handle, error) { return windows.LoadLibraryEx(path, 0, flags) }
	preloadedWinDivertDLL          windows.Handle
	preloadWinDivertDLLMu          sync.Mutex
	checkWinDivertDLL              = func() error {
		preloadWinDivertDLLMu.Lock()
		defer preloadWinDivertDLLMu.Unlock()
		if preloadedWinDivertDLL != 0 {
			return nil
		}
		return preloadWinDivertDLLFromExecutableDir()
	}
	openWinDivertCall = func(filter string, flags uint64) (wd.Handle, error) {
		return wd.Open(filter, wd.LayerNetwork, 0, flags)
	}
)

func preloadWinDivertDLLFromExecutableDir() error {
	exePath, err := resolveWinDivertExecutablePath()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	dllPath := filepath.Join(filepath.Dir(exePath), winDivertDLLName)
	flags := uintptr(windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | windows.LOAD_LIBRARY_SEARCH_APPLICATION_DIR)
	handle, err := loadWinDivertDLLEx(dllPath, flags)
	if err != nil {
		return err
	}
	preloadedWinDivertDLL = handle
	return nil
}

func OpenWinDivertHandle(filter string, flags uint64) (handle wd.Handle, err error) {
	defer func() {
		if r := recover(); r != nil {
			handle = 0
			err = classifyWinDivertError(normalizeWinDivertPanic(r))
		}
	}()
	if err := checkWinDivertDLL(); err != nil {
		return 0, classifyWinDivertError(err)
	}
	handle, err = openWinDivertCall(filter, flags)
	if err != nil {
		return 0, classifyWinDivertError(err)
	}
	return handle, nil
}

func normalizeWinDivertPanic(v any) error {
	switch r := v.(type) {
	case nil:
		return nil
	case error:
		return r
	case string:
		return errors.New(r)
	default:
		return fmt.Errorf("unexpected WinDivert panic: %v", r)
	}
}

func classifyWinDivertError(err error) error {
	if err == nil {
		return nil
	}
	if existing := (*winDivertError)(nil); errors.As(err, &existing) {
		return err
	}
	return &winDivertError{
		Kind:  detectWinDivertErrorKind(err),
		Cause: err,
	}
}

func detectWinDivertErrorKind(err error) winDivertErrorKind {
	if err == nil {
		return winDivertErrorUnknown
	}
	var dllErr *windows.DLLError
	if errors.As(err, &dllErr) {
		objName := dllErr.ObjName
		if strings.EqualFold(objName, winDivertDLLName) || strings.EqualFold(filepath.Base(objName), winDivertDLLName) {
			if errors.Is(dllErr.Err, windows.ERROR_MOD_NOT_FOUND) || errors.Is(dllErr.Err, windows.ERROR_FILE_NOT_FOUND) {
				return winDivertErrorDLLMissing
			}
			return winDivertErrorDLLLoadFailed
		}
	}

	var wdErr wd.Error
	if errors.As(err, &wdErr) {
		switch wdErr {
		case wd.Error(windows.ERROR_FILE_NOT_FOUND):
			return winDivertErrorDriverMissing
		case wd.Error(windows.ERROR_ACCESS_DENIED):
			return winDivertErrorAdminRequired
		case wd.Error(windows.ERROR_DRIVER_BLOCKED):
			return winDivertErrorDriverBlocked
		case wd.Error(windows.ERROR_INVALID_IMAGE_HASH):
			return winDivertErrorDriverSignature
		case wd.Error(windows.EPT_S_NOT_REGISTERED):
			return winDivertErrorBaseFilteringEngineDisabled
		case wd.Error(windows.ERROR_DRIVER_FAILED_PRIOR_UNLOAD):
			return winDivertErrorDriverVersionConflict
		}
	}

	var errno windows.Errno
	if errors.As(err, &errno) {
		switch errno {
		case windows.ERROR_MOD_NOT_FOUND, windows.ERROR_FILE_NOT_FOUND:
			return winDivertErrorDLLMissing
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "Failed to load "+winDivertDLLName):
		return winDivertErrorDLLLoadFailed
	case strings.Contains(msg, "WinDivert32.sys") || strings.Contains(msg, "WinDivert64.sys"):
		return winDivertErrorDriverMissing
	default:
		return winDivertErrorUnknown
	}
}

func winDivertInitHint() string {
	return "请先执行当前程序的 `--init`（例如 `nexttrace --init`），将 WinDivert 运行时解压到可执行文件目录后重试。"
}

func formatWinDivertIssue(err error) string {
	wrapped := classifyWinDivertError(err)
	kind := winDivertErrorUnknown
	var winErr *winDivertError
	if errors.As(wrapped, &winErr) && winErr != nil {
		kind = winErr.Kind
		if kind == winDivertErrorUnknown && winErr.Cause != nil {
			kind = detectWinDivertErrorKind(winErr.Cause)
		}
	}
	if kind == winDivertErrorUnknown {
		kind = detectWinDivertErrorKind(wrapped)
	}
	switch kind {
	case winDivertErrorDLLMissing:
		return fmt.Sprintf("当前缺少 %s。%s", winDivertDLLName, winDivertInitHint())
	case winDivertErrorDLLLoadFailed:
		return fmt.Sprintf("当前无法加载 %s (%v)。%s", winDivertDLLName, wrapped, winDivertInitHint())
	case winDivertErrorDriverMissing:
		return fmt.Sprintf("当前缺少 WinDivert 驱动文件。%s", winDivertInitHint())
	case winDivertErrorAdminRequired:
		return "当前进程没有管理员权限。请使用“以管理员身份运行”的终端重试。"
	case winDivertErrorDriverBlocked:
		return fmt.Sprintf("当前驱动被系统、安全软件或虚拟化环境拦截 (%v)。%s", wrapped, winDivertInitHint())
	case winDivertErrorDriverSignature:
		return fmt.Sprintf("当前驱动签名无效 (%v)。%s", wrapped, winDivertInitHint())
	case winDivertErrorBaseFilteringEngineDisabled:
		return fmt.Sprintf("当前 Windows Base Filtering Engine 服务未启用 (%v)。请启用该服务后重试。", wrapped)
	case winDivertErrorDriverVersionConflict:
		return fmt.Sprintf("当前系统中已加载不兼容的 WinDivert 驱动版本 (%v)。请更新或卸载旧驱动后重试。", wrapped)
	default:
		return fmt.Sprintf("当前无法打开 WinDivert (%v)。若尚未初始化，%s", wrapped, winDivertInitHint())
	}
}

func formatWinDivertRequiredError(feature string, err error) string {
	return fmt.Sprintf("%s 依赖 WinDivert，但%s", feature, formatWinDivertIssue(err))
}

func formatWinDivertFallbackMessage(feature string, err error) string {
	return fmt.Sprintf("请求使用 %s，但%s已回退到 Socket 模式。", feature, trimTrailingPunctuation(formatWinDivertIssue(err))+"；")
}

func trimTrailingPunctuation(s string) string {
	return strings.TrimRight(s, "。.;；!！ ")
}
