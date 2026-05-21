package cmd

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/util"
)

const (
	nextTraceAPIV4ShellPOSIX      = "posix"
	nextTraceAPIV4ShellPowerShell = "powershell"
	nextTraceAPIV4ShellCMD        = "cmd"
)

type nextTraceAPIV4TokenSetupOptions struct {
	stdout      io.Writer
	stderr      io.Writer
	stdoutIsTTY bool
	shell       string
}

func resolveNextTraceAPIV4SetupShell(requested string) string {
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case nextTraceAPIV4ShellPOSIX, nextTraceAPIV4ShellPowerShell, nextTraceAPIV4ShellCMD:
		return strings.ToLower(strings.TrimSpace(requested))
	}
	if runtime.GOOS == "windows" {
		return nextTraceAPIV4ShellPowerShell
	}
	return nextTraceAPIV4ShellPOSIX
}

func runNextTraceAPIV4TokenSetup(opts nextTraceAPIV4TokenSetupOptions) error {
	stdout := opts.stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	shell := resolveNextTraceAPIV4SetupShell(opts.shell)

	printNextTraceAPIV4TokenSetupIntro(stderr, shell)
	if opts.stdoutIsTTY {
		fmt.Fprintln(stderr, "Run the command below; it will prompt for the token and set the environment in the current shell.")
		fmt.Fprintf(stderr, "Use: %s\n", nextTraceAPIV4SetupEvalCommand(shell))
		return nil
	}

	_, err := fmt.Fprint(stdout, formatNextTraceAPIV4TokenSetupScript(shell))
	return err
}

func printNextTraceAPIV4TokenSetupIntro(stderr io.Writer, shell string) {
	fmt.Fprintf(stderr, "Open token page: GET %s\n", ipgeo.NextTraceAPIV4TokenPageURL)
	fmt.Fprintf(stderr, "This writes only a session-scoped %s command to stdout.\n", util.EnvNextTraceAPIV4TokenKey)
	if cmd := nextTraceAPIV4SetupEvalCommand(shell); cmd != "" {
		fmt.Fprintf(stderr, "Recommended usage: %s\n", cmd)
	}
}

func nextTraceAPIV4SetupEvalCommand(shell string) string {
	switch resolveNextTraceAPIV4SetupShell(shell) {
	case nextTraceAPIV4ShellPowerShell:
		return "iex (& nexttrace.exe -x)"
	case nextTraceAPIV4ShellCMD:
		return "for /f \"delims=\" %i in ('nexttrace.exe -x --setup-api-v4-shell=cmd') do %i"
	default:
		return `eval "$(nexttrace -x)"`
	}
}

func formatNextTraceAPIV4TokenSetupScript(shell string) string {
	switch resolveNextTraceAPIV4SetupShell(shell) {
	case nextTraceAPIV4ShellPowerShell:
		return formatNextTraceAPIV4PowerShellSetupScript()
	case nextTraceAPIV4ShellCMD:
		return fmt.Sprintf("set /p %s=Paste NextTrace API v4 token: \r\n", util.EnvNextTraceAPIV4TokenKey)
	default:
		return formatNextTraceAPIV4POSIXSetupScript()
	}
}

func formatNextTraceAPIV4POSIXSetupScript() string {
	return fmt.Sprintf(`printf 'Paste NextTrace API v4 token: ' >&2
if [ -t 0 ]; then
  __nexttrace_api_v4_stty=$(stty -g 2>/dev/null || true)
  stty -echo 2>/dev/null || true
  IFS= read -r %s
  __nexttrace_api_v4_status=$?
  if [ -n "$__nexttrace_api_v4_stty" ]; then
    stty "$__nexttrace_api_v4_stty" 2>/dev/null || true
  fi
  printf '\n' >&2
else
  IFS= read -r %s
  __nexttrace_api_v4_status=$?
fi
if [ "$__nexttrace_api_v4_status" -ne 0 ] || [ -z "${%s}" ]; then
  unset %s
  printf 'No token entered; %s was not set.\n' >&2
else
  export %s
fi
unset __nexttrace_api_v4_stty __nexttrace_api_v4_status
`, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey)
}

func formatNextTraceAPIV4PowerShellSetupScript() string {
	return fmt.Sprintf(`$__nexttraceApiV4Token = Read-Host -AsSecureString 'Paste NextTrace API v4 token'
$__nexttraceApiV4BSTR = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($__nexttraceApiV4Token)
try {
    $env:%s = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($__nexttraceApiV4BSTR)
    if ([string]::IsNullOrWhiteSpace($env:%s)) {
        Remove-Item Env:%s -ErrorAction SilentlyContinue
        [Console]::Error.WriteLine('No token entered; %s was not set.')
    }
}
finally {
    if ($__nexttraceApiV4BSTR -ne [IntPtr]::Zero) {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($__nexttraceApiV4BSTR)
    }
    Remove-Variable __nexttraceApiV4Token -ErrorAction SilentlyContinue
    Remove-Variable __nexttraceApiV4BSTR -ErrorAction SilentlyContinue
}
`, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey, util.EnvNextTraceAPIV4TokenKey)
}
