package windivert

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed x64/WinDivert.dll
var winDivertDLL64 []byte

//go:embed x64/WinDivert64.sys
var winDivertSYS64 []byte

//go:embed x86/WinDivert.dll
var winDivertDLL32 []byte

//go:embed x86/WinDivert32.sys
var winDivertSYS32 []byte

// PrepareWinDivertRuntime 将内嵌的 WinDivert DLL/驱动解压到可执行文件同目录
func PrepareWinDivertRuntime() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exeDir := filepath.Dir(exe)

	var dllBytes, sysBytes []byte
	var sysName string

	switch runtime.GOARCH {
	case "amd64", "arm64":
		dllBytes, sysBytes, sysName = winDivertDLL64, winDivertSYS64, "WinDivert64.sys"
	case "386", "arm":
		dllBytes, sysBytes, sysName = winDivertDLL32, winDivertSYS32, "WinDivert32.sys"
	default:
		return errors.New("unsupported GOARCH for WinDivert: " + runtime.GOARCH)
	}

	// DLL
	if err := writeIfChecksumDiff(filepath.Join(exeDir, "WinDivert.dll"), dllBytes); err != nil {
		return err
	}
	// SYS
	if err := writeIfChecksumDiff(filepath.Join(exeDir, sysName), sysBytes); err != nil {
		return err
	}
	return nil
}

// writeIfChecksumDiff 通过比较 SHA-256 来判断是否覆写目标文件
func writeIfChecksumDiff(dst string, data []byte) error {
	f, err := os.Open(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(dst, data, 0o644)
		}
		// 读失败，则尝试覆盖
		return os.WriteFile(dst, data, 0o644)
	}
	defer func() {
		_ = f.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		// 读失败，则尝试覆盖
		return os.WriteFile(dst, data, 0o644)
	}
	sumFile := h.Sum(nil)
	sumMem := sha256.Sum256(data)

	if bytes.Equal(sumFile, sumMem[:]) {
		return nil // 一致，跳过
	}
	// 不一致，则尝试覆盖
	return os.WriteFile(dst, data, 0o644)
}
