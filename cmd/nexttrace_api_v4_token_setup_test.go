package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nxtrace/NTrace-core/util"
)

func TestRunNextTraceAPIV4TokenSetupWritesSessionTokenFile(t *testing.T) {
	t.Setenv(util.EnvNextTraceAPIV4TokenKey, "")
	var stdout, stderr bytes.Buffer
	var wrote string
	err := runNextTraceAPIV4TokenSetup(nextTraceAPIV4TokenSetupOptions{
		stdout: &stdout,
		stderr: &stderr,
		readToken: func() (string, error) {
			return " secret-token ", nil
		},
		writeToken: func(token string) (string, error) {
			wrote = token
			return "/tmp/nexttrace-session-token", nil
		},
	})
	if err != nil {
		t.Fatalf("runNextTraceAPIV4TokenSetup() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if wrote != "secret-token" {
		t.Fatalf("written token = %q, want trimmed token", wrote)
	}
	if got := os.Getenv(util.EnvNextTraceAPIV4TokenKey); got != "" {
		t.Fatalf("%s = %q, want unchanged empty", util.EnvNextTraceAPIV4TokenKey, got)
	}
	if strings.Contains(stdout.String(), "secret-token") || strings.Contains(stderr.String(), "secret-token") {
		t.Fatalf("output leaked token: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "GET https://api.nxtrace.org/v4/api-tokens") {
		t.Fatalf("stderr missing token page URL: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "/tmp/nexttrace-session-token") {
		t.Fatalf("stderr missing session token path: %q", stderr.String())
	}
}

func TestRunNextTraceAPIV4TokenSetupEmptyTokenDoesNotWrite(t *testing.T) {
	var stdout, stderr bytes.Buffer
	wrote := false
	err := runNextTraceAPIV4TokenSetup(nextTraceAPIV4TokenSetupOptions{
		stdout: &stdout,
		stderr: &stderr,
		readToken: func() (string, error) {
			return "   ", nil
		},
		writeToken: func(token string) (string, error) {
			wrote = true
			return "", nil
		},
	})
	if err != nil {
		t.Fatalf("runNextTraceAPIV4TokenSetup() error = %v", err)
	}
	if wrote {
		t.Fatal("writeToken called for empty token")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "was not written") {
		t.Fatalf("stderr = %q, want empty-token message", stderr.String())
	}
}

func TestRunNextTraceAPIV4TokenSetupWrappedEOFDoesNotWrite(t *testing.T) {
	var stdout, stderr bytes.Buffer
	wrote := false
	err := runNextTraceAPIV4TokenSetup(nextTraceAPIV4TokenSetupOptions{
		stdout: &stdout,
		stderr: &stderr,
		readToken: func() (string, error) {
			return "", fmt.Errorf("wrapped: %w", io.EOF)
		},
		writeToken: func(token string) (string, error) {
			wrote = true
			return "", nil
		},
	})
	if err != nil {
		t.Fatalf("runNextTraceAPIV4TokenSetup() error = %v", err)
	}
	if wrote {
		t.Fatal("writeToken called for wrapped EOF")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "was not written") {
		t.Fatalf("stderr = %q, want empty-token message", stderr.String())
	}
}

func TestRunNextTraceAPIV4TokenSetupDoesNotLeakWriteErrorsToken(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runNextTraceAPIV4TokenSetup(nextTraceAPIV4TokenSetupOptions{
		stdout: &stdout,
		stderr: &stderr,
		readToken: func() (string, error) {
			return "secret-token", nil
		},
		writeToken: func(token string) (string, error) {
			return "/tmp/nexttrace-session-token", os.ErrPermission
		},
	})
	if err == nil {
		t.Fatal("runNextTraceAPIV4TokenSetup() error = nil, want error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked token: %q", err.Error())
	}
	if strings.Contains(stdout.String(), "secret-token") || strings.Contains(stderr.String(), "secret-token") {
		t.Fatalf("stderr leaked token: %q", stderr.String())
	}
}

func TestRunNextTraceAPIV4TokenSetupInterruptedDoesNotWrite(t *testing.T) {
	var stdout, stderr bytes.Buffer
	wrote := false
	err := runNextTraceAPIV4TokenSetup(nextTraceAPIV4TokenSetupOptions{
		stdout: &stdout,
		stderr: &stderr,
		readToken: func() (string, error) {
			return "", errNextTraceAPIV4TokenSetupInterrupted
		},
		writeToken: func(token string) (string, error) {
			wrote = true
			return "", nil
		},
	})
	if !errors.Is(err, errNextTraceAPIV4TokenSetupInterrupted) {
		t.Fatalf("runNextTraceAPIV4TokenSetup() error = %v, want interrupted", err)
	}
	if wrote {
		t.Fatal("writeToken called after interrupted token read")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestHandleNextTraceAPIV4TokenSetupInterruptedError(t *testing.T) {
	var stderr bytes.Buffer
	code := handleNextTraceAPIV4TokenSetupError(&stderr, errNextTraceAPIV4TokenSetupInterrupted)
	if code != 130 {
		t.Fatalf("exit code = %d, want 130", code)
	}
	if !strings.Contains(stderr.String(), "canceled") {
		t.Fatalf("stderr = %q, want canceled message", stderr.String())
	}
}

func TestNextTraceAPIV4TTYTokenErrorKeepsInterruptAndRestoreError(t *testing.T) {
	restoreErr := errors.New("restore failed")
	err := nextTraceAPIV4TTYTokenError(errNextTraceAPIV4TokenSetupInterrupted, restoreErr)
	if !errors.Is(err, errNextTraceAPIV4TokenSetupInterrupted) {
		t.Fatalf("error = %v, want interrupted sentinel", err)
	}
	if !errors.Is(err, restoreErr) {
		t.Fatalf("error = %v, want restore error", err)
	}
	if !strings.Contains(err.Error(), "restore terminal") {
		t.Fatalf("error = %q, want restore context", err.Error())
	}
}

func TestNextTraceAPIV4TTYTokenErrorDoesNotHideRestoreBehindEOF(t *testing.T) {
	restoreErr := errors.New("restore failed")
	err := nextTraceAPIV4TTYTokenError(io.EOF, restoreErr)
	if errors.Is(err, io.EOF) {
		t.Fatalf("error = %v, should not be treated as plain EOF", err)
	}
	if !errors.Is(err, restoreErr) {
		t.Fatalf("error = %v, want restore error", err)
	}
}

func TestReadNextTraceAPIV4HiddenTokenReadsLine(t *testing.T) {
	token, err := readNextTraceAPIV4HiddenToken(strings.NewReader("token\n"))
	if err != nil {
		t.Fatalf("readNextTraceAPIV4HiddenToken() error = %v", err)
	}
	if token != "token" {
		t.Fatalf("token = %q, want token", token)
	}
}

func TestReadNextTraceAPIV4HiddenTokenHandlesBackspace(t *testing.T) {
	token, err := readNextTraceAPIV4HiddenToken(strings.NewReader("abc\bd\n"))
	if err != nil {
		t.Fatalf("readNextTraceAPIV4HiddenToken() error = %v", err)
	}
	if token != "abd" {
		t.Fatalf("token = %q, want abd", token)
	}
}

func TestReadNextTraceAPIV4HiddenTokenInterruptsOnCtrlC(t *testing.T) {
	token, err := readNextTraceAPIV4HiddenToken(strings.NewReader("secret\x03"))
	if !errors.Is(err, errNextTraceAPIV4TokenSetupInterrupted) {
		t.Fatalf("readNextTraceAPIV4HiddenToken() error = %v, want interrupted", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}

func TestReadNextTraceAPIV4HiddenTokenReturnsEOFOnCtrlD(t *testing.T) {
	token, err := readNextTraceAPIV4HiddenToken(strings.NewReader("\x04"))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("readNextTraceAPIV4HiddenToken() error = %v, want EOF", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty", token)
	}
}
