package util

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
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

	var got string
	stdout, stderr := captureStdoutStderr(t, func() {
		got = GetSecretEnvDefault("TEST_SECRET_KEY", "")
	})

	assert.Equal(t, "secret-token", got)
	assert.Equal(t, "", stdout)
	assert.Contains(t, stderr, "TEST_SECRET_KEY")
	assert.NotContains(t, stderr, "secret-token")
	assert.False(t, strings.Contains(stderr, " secret-token "))
}

func TestGetEnvTrimmedDebugPrintsToStderr(t *testing.T) {
	t.Setenv("NEXTTRACE_DEBUG", "1")
	t.Setenv("TEST_DEBUG_KEY", " debug-value ")

	var got string
	stdout, stderr := captureStdoutStderr(t, func() {
		var ok bool
		got, ok = GetEnvTrimmed("TEST_DEBUG_KEY")
		require.True(t, ok)
	})

	assert.Equal(t, "debug-value", got)
	assert.Equal(t, "", stdout)
	assert.Contains(t, stderr, "TEST_DEBUG_KEY")
	assert.Contains(t, stderr, "debug-value")
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

func TestGetNextTraceAPIV4TokenFallsBackToLatestFile(t *testing.T) {
	paths := overrideNextTraceAPIV4TokenPaths(t)
	require.NoError(t, os.WriteFile(paths.latest, []byte(" latest-token \n"), 0o600))
	t.Setenv(EnvNextTraceAPIV4TokenKey, "")

	assert.Equal(t, "latest-token", GetNextTraceAPIV4Token())
	assert.Equal(t, "latest-token", os.Getenv(EnvNextTraceAPIV4TokenKey))
}

func TestGetNextTraceAPIV4TokenDebugLogsReadErrorAndPaths(t *testing.T) {
	t.Setenv("NEXTTRACE_DEBUG", "1")
	t.Setenv(EnvNextTraceAPIV4TokenKey, "")
	parentPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(parentPath, []byte("not-a-secret-token\n"), 0o600))
	sessionPath := filepath.Join(parentPath, "session")
	latestPath := filepath.Join(parentPath, "latest")
	overrideNextTraceAPIV4TokenPathFuncs(t, sessionPath, latestPath)

	var token string
	stdout, stderr := captureStdoutStderr(t, func() {
		token = GetNextTraceAPIV4Token()
	})

	assert.Equal(t, "", token)
	assert.Equal(t, "", stdout)
	output := stderr
	assert.Contains(t, output, EnvNextTraceAPIV4TokenKey)
	assert.Contains(t, output, "session token file read failed")
	assert.Contains(t, output, sessionPath)
	assert.Contains(t, output, latestPath)
	assert.NotContains(t, output, "not-a-secret-token")
}

func TestGetNextTraceAPIV4TokenRejectsSymlinkSessionFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows can require extra privileges")
	}
	paths := overrideNextTraceAPIV4TokenPaths(t)
	outside := filepath.Join(t.TempDir(), "outside-token")
	require.NoError(t, os.WriteFile(outside, []byte("outside-token\n"), 0o600))
	require.NoError(t, os.Symlink(outside, paths.session))
	t.Setenv(EnvNextTraceAPIV4TokenKey, "")

	assert.Equal(t, "", GetNextTraceAPIV4Token())
	assert.Equal(t, "", os.Getenv(EnvNextTraceAPIV4TokenKey))
}

func TestGetNextTraceAPIV4TokenSkipsSymlinkSessionFileAndReadsLatest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows can require extra privileges")
	}
	paths := overrideNextTraceAPIV4TokenPaths(t)
	outside := filepath.Join(t.TempDir(), "outside-token")
	require.NoError(t, os.WriteFile(outside, []byte("outside-token\n"), 0o600))
	require.NoError(t, os.Symlink(outside, paths.session))
	require.NoError(t, os.WriteFile(paths.latest, []byte(" latest-token \n"), 0o600))
	t.Setenv(EnvNextTraceAPIV4TokenKey, "")

	assert.Equal(t, "latest-token", GetNextTraceAPIV4Token())
	assert.Equal(t, "latest-token", os.Getenv(EnvNextTraceAPIV4TokenKey))
}

func TestReadNextTraceAPIV4SessionTokenRejectsSymlinkDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows can require extra privileges")
	}
	parent := t.TempDir()
	realDir := filepath.Join(parent, "real")
	linkDir := filepath.Join(parent, "link")
	require.NoError(t, os.Mkdir(realDir, 0o700))
	require.NoError(t, os.Symlink(realDir, linkDir))
	overrideNextTraceAPIV4TokenPathFuncs(t, filepath.Join(linkDir, "session"), filepath.Join(linkDir, "latest"))

	token, err := ReadNextTraceAPIV4SessionToken()
	require.Error(t, err)
	assert.Equal(t, "", token)
	assert.Contains(t, err.Error(), "symlink")
}

func TestReadNextTraceAPIV4SessionTokenMissingDirectoryIsEmpty(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "missing")
	overrideNextTraceAPIV4TokenPathFuncs(t, filepath.Join(missingDir, "session"), filepath.Join(missingDir, "latest"))

	token, err := ReadNextTraceAPIV4SessionToken()
	require.NoError(t, err)
	assert.Equal(t, "", token)
	_, statErr := os.Stat(missingDir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestReadNextTraceAPIV4SessionTokenRejectsDirectoryTokenPath(t *testing.T) {
	paths := overrideNextTraceAPIV4TokenPaths(t)
	require.NoError(t, os.Mkdir(paths.session, 0o700))

	token, err := ReadNextTraceAPIV4SessionToken()
	require.Error(t, err)
	assert.Equal(t, "", token)
	assert.Contains(t, err.Error(), "directory")
}

func TestReadNextTraceAPIV4SessionTokenRejectsLooseDirectoryPerms(t *testing.T) {
	if !strictNextTraceAPIV4TokenPerms() {
		t.Skip("POSIX mode bits are not enforced on this platform")
	}
	paths := overrideNextTraceAPIV4TokenPaths(t)
	dir := filepath.Dir(paths.session)
	require.NoError(t, os.WriteFile(paths.session, []byte("file-token\n"), 0o600))
	require.NoError(t, os.Chmod(dir, 0o755))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	token, err := ReadNextTraceAPIV4SessionToken()
	require.Error(t, err)
	assert.Equal(t, "", token)
	assert.Contains(t, err.Error(), "permissions")
}

func TestWriteNextTraceAPIV4SessionTokenWritesTempFiles(t *testing.T) {
	paths := overrideNextTraceAPIV4TokenPaths(t)

	path, err := WriteNextTraceAPIV4SessionToken(" file-token ")
	require.NoError(t, err)
	assert.Equal(t, paths.session, path)

	for _, path := range []string{paths.session, paths.latest} {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "file-token\n", string(body))
		if strictNextTraceAPIV4TokenPerms() {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
	}
	if strictNextTraceAPIV4TokenPerms() {
		info, err := os.Stat(filepath.Dir(paths.session))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	}
}

func TestGetNextTraceAPIV4TokenDebugLoadedMessagePrintsToStderr(t *testing.T) {
	t.Setenv("NEXTTRACE_DEBUG", "1")
	paths := overrideNextTraceAPIV4TokenPaths(t)
	require.NoError(t, os.WriteFile(paths.session, []byte(" file-token \n"), 0o600))
	t.Setenv(EnvNextTraceAPIV4TokenKey, "")

	var token string
	stdout, stderr := captureStdoutStderr(t, func() {
		token = GetNextTraceAPIV4Token()
	})

	assert.Equal(t, "file-token", token)
	assert.Equal(t, "", stdout)
	assert.Contains(t, stderr, EnvNextTraceAPIV4TokenKey)
	assert.Contains(t, stderr, "loaded from session token file")
	assert.NotContains(t, stderr, "file-token")
}

func TestWriteNextTraceAPIV4SessionTokenRejectsSymlinkFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows can require extra privileges")
	}
	paths := overrideNextTraceAPIV4TokenPaths(t)
	outside := filepath.Join(t.TempDir(), "outside-token")
	require.NoError(t, os.WriteFile(outside, []byte("outside\n"), 0o600))
	require.NoError(t, os.Symlink(outside, paths.session))

	_, err := WriteNextTraceAPIV4SessionToken(" file-token ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
	body, readErr := os.ReadFile(outside)
	require.NoError(t, readErr)
	assert.Equal(t, "outside\n", string(body))
}

func TestWriteNextTraceAPIV4SessionTokenRejectsDirectorySessionPath(t *testing.T) {
	paths := overrideNextTraceAPIV4TokenPaths(t)
	require.NoError(t, os.Mkdir(paths.session, 0o700))

	_, err := WriteNextTraceAPIV4SessionToken(" file-token ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestWriteNextTraceAPIV4SessionTokenRejectsSymlinkDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows can require extra privileges")
	}
	parent := t.TempDir()
	realDir := filepath.Join(parent, "real")
	linkDir := filepath.Join(parent, "link")
	require.NoError(t, os.Mkdir(realDir, 0o700))
	require.NoError(t, os.Symlink(realDir, linkDir))

	oldPathFunc := nextTraceAPIV4SessionTokenPath
	oldLatestPathFunc := nextTraceAPIV4LatestTokenPath
	nextTraceAPIV4SessionTokenPath = func() string { return filepath.Join(linkDir, "session") }
	nextTraceAPIV4LatestTokenPath = func() string { return filepath.Join(linkDir, "latest") }
	t.Cleanup(func() {
		nextTraceAPIV4SessionTokenPath = oldPathFunc
		nextTraceAPIV4LatestTokenPath = oldLatestPathFunc
	})

	_, err := WriteNextTraceAPIV4SessionToken(" file-token ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

func TestWriteNextTraceAPIV4SessionTokenIgnoresLatestWriteFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows can require extra privileges")
	}
	paths := overrideNextTraceAPIV4TokenPaths(t)
	outside := filepath.Join(t.TempDir(), "outside-token")
	require.NoError(t, os.WriteFile(outside, []byte("outside\n"), 0o600))
	require.NoError(t, os.Symlink(outside, paths.latest))

	path, err := WriteNextTraceAPIV4SessionToken(" file-token ")
	require.NoError(t, err)
	assert.Equal(t, paths.session, path)

	body, err := os.ReadFile(paths.session)
	require.NoError(t, err)
	assert.Equal(t, "file-token\n", string(body))
	outsideBody, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "outside\n", string(outsideBody))
}

func TestWriteNextTraceAPIV4SessionTokenIgnoresLatestDirectoryFailure(t *testing.T) {
	paths := overrideNextTraceAPIV4TokenPaths(t)
	require.NoError(t, os.Mkdir(paths.latest, 0o700))

	path, err := WriteNextTraceAPIV4SessionToken(" file-token ")
	require.NoError(t, err)
	assert.Equal(t, paths.session, path)

	body, err := os.ReadFile(paths.session)
	require.NoError(t, err)
	assert.Equal(t, "file-token\n", string(body))
	info, err := os.Stat(paths.latest)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func overrideNextTraceAPIV4SessionTokenPath(t *testing.T) string {
	t.Helper()
	return overrideNextTraceAPIV4TokenPaths(t).session
}

type nextTraceAPIV4TokenTestPaths struct {
	session string
	latest  string
}

func overrideNextTraceAPIV4TokenPaths(t *testing.T) nextTraceAPIV4TokenTestPaths {
	t.Helper()
	dir := t.TempDir()
	if strictNextTraceAPIV4TokenPerms() {
		require.NoError(t, os.Chmod(dir, 0o700))
	}
	tokenPath := dir + string(os.PathSeparator) + "nexttrace-api-v4-token"
	latestPath := dir + string(os.PathSeparator) + "nexttrace-api-v4-token-latest"

	overrideNextTraceAPIV4TokenPathFuncs(t, tokenPath, latestPath)
	return nextTraceAPIV4TokenTestPaths{session: tokenPath, latest: latestPath}
}

func overrideNextTraceAPIV4TokenPathFuncs(t *testing.T, tokenPath, latestPath string) {
	t.Helper()
	oldPathFunc := nextTraceAPIV4SessionTokenPath
	oldLatestPathFunc := nextTraceAPIV4LatestTokenPath
	nextTraceAPIV4SessionTokenPath = func() string { return tokenPath }
	nextTraceAPIV4LatestTokenPath = func() string { return latestPath }
	t.Cleanup(func() {
		nextTraceAPIV4SessionTokenPath = oldPathFunc
		nextTraceAPIV4LatestTokenPath = oldLatestPathFunc
	})
}

func captureStdoutStderr(t *testing.T, fn func()) (string, string) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = stdoutR.Close() })
	t.Cleanup(func() { _ = stdoutW.Close() })
	t.Cleanup(func() { _ = stderrR.Close() })
	t.Cleanup(func() { _ = stderrW.Close() })
	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()
	require.NoError(t, stdoutW.Close())
	require.NoError(t, stderrW.Close())
	stdout, err := io.ReadAll(stdoutR)
	require.NoError(t, err)
	stderr, err := io.ReadAll(stderrR)
	require.NoError(t, err)
	return string(stdout), string(stderr)
}
