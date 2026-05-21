package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/nxtrace/NTrace-core/util"
)

func TestFormatAPIV4TokenSetupScriptPromptsInParentShell(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		want  []string
	}{
		{name: "posix", shell: apiV4ShellPOSIX, want: []string{"read -r NEXTTRACE_API_V4_TOKEN", "export NEXTTRACE_API_V4_TOKEN"}},
		{name: "powershell", shell: apiV4ShellPowerShell, want: []string{"Read-Host -AsSecureString", "$env:NEXTTRACE_API_V4_TOKEN"}},
		{name: "cmd", shell: apiV4ShellCMD, want: []string{"set /p NEXTTRACE_API_V4_TOKEN=Paste v4 API token:"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAPIV4TokenSetupScript(tt.shell)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("setup script = %q, want containing %q", got, want)
				}
			}
			if strings.Contains(got, "secret-token") {
				t.Fatalf("setup script leaked test token: %q", got)
			}
		})
	}
}

func TestRunAPIV4TokenSetupStdoutTTYDoesNotPrintOrReadToken(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runAPIV4TokenSetup(apiV4TokenSetupOptions{
		stdout:      &stdout,
		stderr:      &stderr,
		stdoutIsTTY: true,
		shell:       apiV4ShellPOSIX,
	})
	if err != nil {
		t.Fatalf("runAPIV4TokenSetup() error = %v", err)
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
	if !strings.Contains(stderr.String(), "will prompt for the token") {
		t.Fatalf("stderr missing direct-run guidance: %q", stderr.String())
	}
}

func TestRunAPIV4TokenSetupWritesOnlySetupScriptToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runAPIV4TokenSetup(apiV4TokenSetupOptions{
		stdout:      &stdout,
		stderr:      &stderr,
		stdoutIsTTY: false,
		shell:       apiV4ShellPowerShell,
	})
	if err != nil {
		t.Fatalf("runAPIV4TokenSetup() error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "Read-Host -AsSecureString") || !strings.Contains(got, "$env:NEXTTRACE_API_V4_TOKEN") {
		t.Fatalf("stdout = %q, want PowerShell setup script", got)
	}
	if strings.Contains(stdout.String(), "secret-token") {
		t.Fatalf("stdout leaked token: %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "secret-token") {
		t.Fatalf("stderr leaked token: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "GET https://api.nxtrace.org/v4/api-tokens") {
		t.Fatalf("stderr missing token page URL: %q", stderr.String())
	}
}

func TestRunAPIV4TokenSetupGenerationDoesNotSetEnv(t *testing.T) {
	t.Setenv(util.EnvAPIV4TokenKey, "")
	var stdout, stderr bytes.Buffer
	err := runAPIV4TokenSetup(apiV4TokenSetupOptions{
		stdout:      &stdout,
		stderr:      &stderr,
		stdoutIsTTY: false,
		shell:       apiV4ShellPOSIX,
	})
	if err != nil {
		t.Fatalf("runAPIV4TokenSetup() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "read -r NEXTTRACE_API_V4_TOKEN") {
		t.Fatalf("stdout = %q, want POSIX setup script", stdout.String())
	}
	if got := os.Getenv(util.EnvAPIV4TokenKey); got != "" {
		t.Fatalf("%s = %q, want unchanged empty", util.EnvAPIV4TokenKey, got)
	}
}
