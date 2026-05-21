package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/nxtrace/NTrace-core/util"
)

func TestFormatAPIV4TokenAssignmentQuotesShells(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		token string
		want  string
	}{
		{name: "posix", shell: apiV4ShellPOSIX, token: `abc'def`, want: `export NEXTTRACE_API_V4_TOKEN='abc'\''def'` + "\n"},
		{name: "powershell", shell: apiV4ShellPowerShell, token: `abc'def`, want: "$env:NEXTTRACE_API_V4_TOKEN = 'abc''def'\n"},
		{name: "cmd", shell: apiV4ShellCMD, token: `abc%def`, want: "set \"NEXTTRACE_API_V4_TOKEN=abc%%def\"\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatAPIV4TokenAssignment(tt.shell, tt.token); got != tt.want {
				t.Fatalf("assignment = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunAPIV4TokenSetupStdoutTTYDoesNotPrintOrReadToken(t *testing.T) {
	var stdout, stderr bytes.Buffer
	readCalled := false
	err := runAPIV4TokenSetup(apiV4TokenSetupOptions{
		stdout:      &stdout,
		stderr:      &stderr,
		stdoutIsTTY: true,
		shell:       apiV4ShellPOSIX,
		readToken: func() (string, error) {
			readCalled = true
			return "secret-token", nil
		},
	})
	if err != nil {
		t.Fatalf("runAPIV4TokenSetup() error = %v", err)
	}
	if readCalled {
		t.Fatal("readToken called even though stdout is a terminal")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), "secret-token") {
		t.Fatalf("stderr leaked token: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `eval "$(nexttrace -x)"`) {
		t.Fatalf("stderr missing session setup usage: %q", stderr.String())
	}
}

func TestRunAPIV4TokenSetupWritesOnlyAssignmentToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runAPIV4TokenSetup(apiV4TokenSetupOptions{
		stdout:      &stdout,
		stderr:      &stderr,
		stdoutIsTTY: false,
		shell:       apiV4ShellPowerShell,
		readToken: func() (string, error) {
			return " secret-token ", nil
		},
	})
	if err != nil {
		t.Fatalf("runAPIV4TokenSetup() error = %v", err)
	}
	if got := stdout.String(); got != "$env:NEXTTRACE_API_V4_TOKEN = 'secret-token'\n" {
		t.Fatalf("stdout = %q, want PowerShell assignment only", got)
	}
	if strings.Contains(stderr.String(), "secret-token") {
		t.Fatalf("stderr leaked token: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "GET https://api.nxtrace.org/v4/api-tokens") {
		t.Fatalf("stderr missing token page URL: %q", stderr.String())
	}
}

func TestRunAPIV4TokenSetupEmptyTokenWritesNoAssignmentAndDoesNotSetEnv(t *testing.T) {
	t.Setenv(util.EnvAPIV4TokenKey, "")
	var stdout, stderr bytes.Buffer
	err := runAPIV4TokenSetup(apiV4TokenSetupOptions{
		stdout:      &stdout,
		stderr:      &stderr,
		stdoutIsTTY: false,
		shell:       apiV4ShellPOSIX,
		readToken: func() (string, error) {
			return "   ", nil
		},
	})
	if err != nil {
		t.Fatalf("runAPIV4TokenSetup() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := os.Getenv(util.EnvAPIV4TokenKey); got != "" {
		t.Fatalf("%s = %q, want unchanged empty", util.EnvAPIV4TokenKey, got)
	}
}
