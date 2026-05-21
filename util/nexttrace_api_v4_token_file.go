package util

import (
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

func WriteNextTraceAPIV4SessionToken(token string) (string, error) {
	path := NextTraceAPIV4SessionTokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return path, err
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(token)+"\n"), 0o600); err != nil {
		return path, err
	}
	latestPath := NextTraceAPIV4LatestTokenPath()
	if err := os.WriteFile(latestPath, []byte(strings.TrimSpace(token)+"\n"), 0o600); err != nil {
		return latestPath, err
	}
	return path, nil
}
