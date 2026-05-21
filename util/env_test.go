package util

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEnvTrimmed(t *testing.T) {
	t.Setenv("TEST_TRIMMED_KEY", "  value  ")

	val, ok := GetEnvTrimmed("TEST_TRIMMED_KEY")
	assert.True(t, ok)
	assert.Equal(t, "value", val)

	_, ok = GetEnvTrimmed("TEST_TRIMMED_MISSING")
	assert.False(t, ok)
}

func TestGetEnvBool(t *testing.T) {
	t.Setenv("TEST_BOOL_TRUE", "1")
	assert.True(t, GetEnvBool("TEST_BOOL_TRUE", false))

	t.Setenv("TEST_BOOL_FALSE", "0")
	assert.False(t, GetEnvBool("TEST_BOOL_FALSE", true))

	t.Setenv("TEST_BOOL_INVALID", "maybe")
	assert.True(t, GetEnvBool("TEST_BOOL_INVALID", true))
}

func TestGetEnvDefault(t *testing.T) {
	t.Setenv("TEST_DEFAULT_KEY", " custom ")
	assert.Equal(t, "custom", GetEnvDefault("TEST_DEFAULT_KEY", "fallback"))

	assert.Equal(t, "fallback", GetEnvDefault("TEST_DEFAULT_MISSING", "fallback"))
}

func TestGetSecretEnvDefaultDoesNotPrintValueInDebugMode(t *testing.T) {
	t.Setenv("NEXTTRACE_DEBUG", "1")
	t.Setenv("TEST_SECRET_KEY", " secret-token ")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	got := GetSecretEnvDefault("TEST_SECRET_KEY", "")
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Equal(t, "secret-token", got)
	assert.Contains(t, string(out), "TEST_SECRET_KEY")
	assert.NotContains(t, string(out), "secret-token")
	assert.False(t, strings.Contains(string(out), " secret-token "))
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("TEST_INT_VALID", " 42 ")
	assert.Equal(t, 42, GetEnvInt("TEST_INT_VALID", 7))

	t.Setenv("TEST_INT_INVALID", "NaN")
	assert.Equal(t, 5, GetEnvInt("TEST_INT_INVALID", 5))

	assert.Equal(t, 9, GetEnvInt("TEST_INT_MISSING", 9))
}

func TestAllowCrossOriginBrowserAccess(t *testing.T) {
	t.Setenv(EnvAllowCrossOriginKey, "1")
	assert.True(t, AllowCrossOriginBrowserAccess())

	t.Setenv(EnvAllowCrossOriginKey, "0")
	assert.False(t, AllowCrossOriginBrowserAccess())
}

func TestGetNextTraceAPIV4TokenPrefersEnvOverSessionFile(t *testing.T) {
	tokenPath := overrideNextTraceAPIV4SessionTokenPath(t)
	require.NoError(t, os.WriteFile(tokenPath, []byte("file-token\n"), 0o600))
	t.Setenv(EnvNextTraceAPIV4TokenKey, " env-token ")

	assert.Equal(t, "env-token", GetNextTraceAPIV4Token())
}

func TestGetNextTraceAPIV4TokenLoadsSessionFileIntoEnv(t *testing.T) {
	tokenPath := overrideNextTraceAPIV4SessionTokenPath(t)
	require.NoError(t, os.WriteFile(tokenPath, []byte(" file-token \n"), 0o600))
	t.Setenv(EnvNextTraceAPIV4TokenKey, "")

	assert.Equal(t, "file-token", GetNextTraceAPIV4Token())
	assert.Equal(t, "file-token", os.Getenv(EnvNextTraceAPIV4TokenKey))
}

func TestWriteNextTraceAPIV4SessionTokenWritesTempFile(t *testing.T) {
	tokenPath := overrideNextTraceAPIV4SessionTokenPath(t)

	path, err := WriteNextTraceAPIV4SessionToken(" file-token ")
	require.NoError(t, err)
	assert.Equal(t, tokenPath, path)

	body, err := os.ReadFile(tokenPath)
	require.NoError(t, err)
	assert.Equal(t, "file-token\n", string(body))
}

func overrideNextTraceAPIV4SessionTokenPath(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + string(os.PathSeparator) + "nexttrace-api-v4-token"
	oldPathFunc := nextTraceAPIV4SessionTokenPath
	nextTraceAPIV4SessionTokenPath = func() string { return path }
	t.Cleanup(func() { nextTraceAPIV4SessionTokenPath = oldPathFunc })
	return path
}
