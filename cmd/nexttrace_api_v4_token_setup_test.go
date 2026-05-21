package cmd

import (
	"bytes"
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
