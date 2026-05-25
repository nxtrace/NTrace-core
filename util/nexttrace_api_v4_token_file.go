package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	nextTraceAPIV4SessionTokenPath = defaultNextTraceAPIV4SessionTokenPath
	nextTraceAPIV4LatestTokenPath  = defaultNextTraceAPIV4LatestTokenPath
)

func NextTraceAPIV4SessionTokenPath() string {
	return nextTraceAPIV4SessionTokenPath()
}

func NextTraceAPIV4LatestTokenPath() string {
	return nextTraceAPIV4LatestTokenPath()
}

func defaultNextTraceAPIV4SessionTokenPath() string {
	return filepath.Join(nextTraceAPIV4TokenDir(), "nexttrace-api-v4-token-"+strconv.Itoa(os.Getppid()))
}

func defaultNextTraceAPIV4LatestTokenPath() string {
	return filepath.Join(nextTraceAPIV4TokenDir(), "nexttrace-api-v4-token-latest")
}

func nextTraceAPIV4TokenDir() string {
	return filepath.Join(os.TempDir(), "nexttrace-"+nextTraceAPIV4TempUserKey())
}

func nextTraceAPIV4TempUserKey() string {
	if current, err := user.Current(); err == nil && strings.TrimSpace(current.Uid) != "" {
		return sanitizeNextTraceAPIV4PathPart(current.Uid)
	}
	for _, key := range []string{"USER", "USERNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return sanitizeNextTraceAPIV4PathPart(value)
		}
	}
	return "unknown"
}

func sanitizeNextTraceAPIV4PathPart(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func ReadNextTraceAPIV4SessionToken() (string, error) {
	var firstErr error
	for _, path := range []string{NextTraceAPIV4SessionTokenPath(), NextTraceAPIV4LatestTokenPath()} {
		if err := validateNextTraceAPIV4TokenReadPath(path); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if token := strings.TrimSpace(string(body)); token != "" {
			return token, nil
		}
	}
	return "", firstErr
}

func validateNextTraceAPIV4TokenReadPath(path string) error {
	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("NextTrace API v4 token directory is a symlink: %s", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("NextTrace API v4 token path is not a directory: %s", dir)
	}
	if err := checkNextTraceAPIV4TokenDirOwner(info); err != nil {
		return err
	}
	if strictNextTraceAPIV4TokenPerms() && info.Mode().Perm() != 0o700 {
		return fmt.Errorf("NextTrace API v4 token directory permissions are %04o, want 0700", info.Mode().Perm())
	}
	return rejectNextTraceAPIV4Symlink(path)
}

func WriteNextTraceAPIV4SessionToken(token string) (string, error) {
	path := NextTraceAPIV4SessionTokenPath()
	body := []byte(strings.TrimSpace(token) + "\n")
	if err := writeNextTraceAPIV4TokenFile(path, body); err != nil {
		return path, err
	}
	latestPath := NextTraceAPIV4LatestTokenPath()
	_ = writeNextTraceAPIV4TokenFile(latestPath, body)
	return path, nil
}

func writeNextTraceAPIV4TokenFile(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := ensureNextTraceAPIV4TokenDir(dir); err != nil {
		return err
	}
	if err := rejectNextTraceAPIV4Symlink(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".nexttrace-api-v4-token-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTmp := false
	defer func() {
		if !keepTmp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil && strictNextTraceAPIV4TokenPerms() {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := rejectNextTraceAPIV4Symlink(path); err != nil {
		return err
	}
	if err := replaceNextTraceAPIV4TokenFile(tmpPath, path); err != nil {
		return err
	}
	keepTmp = true
	return nil
}

func ensureNextTraceAPIV4TokenDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("NextTrace API v4 token directory is a symlink: %s", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("NextTrace API v4 token path is not a directory: %s", dir)
	}
	if err := checkNextTraceAPIV4TokenDirOwner(info); err != nil {
		return err
	}
	if strictNextTraceAPIV4TokenPerms() {
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(dir)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("NextTrace API v4 token directory is a symlink: %s", dir)
		}
		if !info.IsDir() {
			return fmt.Errorf("NextTrace API v4 token path is not a directory: %s", dir)
		}
		if info.Mode().Perm() != 0o700 {
			return fmt.Errorf("NextTrace API v4 token directory permissions are %04o, want 0700", info.Mode().Perm())
		}
	}
	return nil
}

func rejectNextTraceAPIV4Symlink(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("NextTrace API v4 token file is a symlink: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("NextTrace API v4 token path is a directory: %s", path)
	}
	return nil
}
