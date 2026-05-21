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
	apiV4ShellPOSIX      = "posix"
	apiV4ShellPowerShell = "powershell"
	apiV4ShellCMD        = "cmd"
)

type apiV4TokenSetupOptions struct {
	stdout      io.Writer
	stderr      io.Writer
	stdoutIsTTY bool
	shell       string
}

func resolveAPIV4SetupShell(requested string) string {
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case apiV4ShellPOSIX, apiV4ShellPowerShell, apiV4ShellCMD:
		return strings.ToLower(strings.TrimSpace(requested))
	}
	if runtime.GOOS == "windows" {
		return apiV4ShellPowerShell
	}
	return apiV4ShellPOSIX
}

func runAPIV4TokenSetup(opts apiV4TokenSetupOptions) error {
	stdout := opts.stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	shell := resolveAPIV4SetupShell(opts.shell)

	printAPIV4TokenSetupIntro(stderr, shell)
	if opts.stdoutIsTTY {
		fmt.Fprintln(stderr, "Run the command below; it will prompt for the token and set the environment in the current shell.")
		fmt.Fprintf(stderr, "Use: %s\n", apiV4SetupEvalCommand(shell))
		return nil
	}

	_, err := fmt.Fprint(stdout, formatAPIV4TokenSetupScript(shell))
	return err
}

func printAPIV4TokenSetupIntro(stderr io.Writer, shell string) {
	fmt.Fprintf(stderr, "Open token page: GET %s\n", ipgeo.APIV4TokenPageURL)
	fmt.Fprintf(stderr, "This writes only a session-scoped %s command to stdout.\n", util.EnvAPIV4TokenKey)
	if cmd := apiV4SetupEvalCommand(shell); cmd != "" {
		fmt.Fprintf(stderr, "Recommended usage: %s\n", cmd)
	}
}

func apiV4SetupEvalCommand(shell string) string {
	switch resolveAPIV4SetupShell(shell) {
	case apiV4ShellPowerShell:
		return "iex (& nexttrace.exe -x)"
	case apiV4ShellCMD:
		return "for /f \"delims=\" %i in ('nexttrace.exe -x --setup-api-v4-shell=cmd') do %i"
	default:
		return `eval "$(nexttrace -x)"`
	}
}

func formatAPIV4TokenSetupScript(shell string) string {
	switch resolveAPIV4SetupShell(shell) {
	case apiV4ShellPowerShell:
		return formatAPIV4PowerShellSetupScript()
	case apiV4ShellCMD:
		return fmt.Sprintf("set /p %s=Paste v4 API token: \r\n", util.EnvAPIV4TokenKey)
	default:
		return formatAPIV4POSIXSetupScript()
	}
}

func formatAPIV4POSIXSetupScript() string {
	return fmt.Sprintf(`printf 'Paste v4 API token: ' >&2
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
`, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey)
}

func formatAPIV4PowerShellSetupScript() string {
	return fmt.Sprintf(`$__nexttraceApiV4Token = Read-Host -AsSecureString 'Paste v4 API token'
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
`, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey, util.EnvAPIV4TokenKey)
}
