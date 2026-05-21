package util

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

var nextTraceAPIV4SessionTokenPath = defaultNextTraceAPIV4SessionTokenPath

func NextTraceAPIV4SessionTokenPath() string {
	return nextTraceAPIV4SessionTokenPath()
}

func defaultNextTraceAPIV4SessionTokenPath() string {
	dir := filepath.Join(os.TempDir(), "nexttrace-"+nextTraceAPIV4TempUserKey())
	return filepath.Join(dir, "nexttrace-api-v4-token-"+strconv.Itoa(os.Getppid()))
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
	body, err := os.ReadFile(NextTraceAPIV4SessionTokenPath())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func WriteNextTraceAPIV4SessionToken(token string) (string, error) {
	path := NextTraceAPIV4SessionTokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return path, err
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(token)+"\n"), 0o600); err != nil {
		return path, err
	}
	return path, nil
}
