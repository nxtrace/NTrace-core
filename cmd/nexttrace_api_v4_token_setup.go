package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"

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
	stdin       *os.File
	stdoutIsTTY bool
	shell       string
	readToken   func(*os.File, io.Writer) (string, error)
	startShell  func(nextTraceAPIV4TokenShellOptions) error
}

type nextTraceAPIV4TokenShellOptions struct {
	stdout io.Writer
	stderr io.Writer
	stdin  *os.File
	shell  string
	token  string
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
	stdin := opts.stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	shell := resolveNextTraceAPIV4SetupShell(opts.shell)

	printNextTraceAPIV4TokenSetupIntro(stderr, shell, !opts.stdoutIsTTY)
	if opts.stdoutIsTTY {
		readToken := opts.readToken
		if readToken == nil {
			readToken = readNextTraceAPIV4Token
		}
		token, err := readToken(stdin, stderr)
		if err != nil && err != io.EOF {
			return err
		}
		token = strings.TrimSpace(token)
		if token == "" {
			fmt.Fprintf(stderr, "No token entered; %s was not set.\n", util.EnvNextTraceAPIV4TokenKey)
			return nil
		}
		startShell := opts.startShell
		if startShell == nil {
			startShell = startNextTraceAPIV4TokenShell
		}
		fmt.Fprintf(stderr, "Starting a child shell with %s set. Run NextTrace commands there; type exit to return.\n", util.EnvNextTraceAPIV4TokenKey)
		return startShell(nextTraceAPIV4TokenShellOptions{
			stdout: stdout,
			stderr: stderr,
			stdin:  stdin,
			shell:  shell,
			token:  token,
		})
	}

	_, err := fmt.Fprint(stdout, formatNextTraceAPIV4TokenSetupScript(shell))
	return err
}

func printNextTraceAPIV4TokenSetupIntro(stderr io.Writer, shell string, includeScriptHint bool) {
	fmt.Fprintf(stderr, "Open token page: GET %s\n", ipgeo.NextTraceAPIV4TokenPageURL)
	if includeScriptHint {
		cmd := nextTraceAPIV4SetupEvalCommand(shell)
		fmt.Fprintf(stderr, "Non-interactive script mode: %s\n", cmd)
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

func readNextTraceAPIV4Token(stdin *os.File, stderr io.Writer) (string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	fmt.Fprint(stderr, "Paste NextTrace API v4 token: ")
	if CheckTTY(int(stdin.Fd())) {
		tokenBytes, err := term.ReadPassword(int(stdin.Fd()))
		fmt.Fprintln(stderr)
		return string(tokenBytes), err
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, err
}

func startNextTraceAPIV4TokenShell(opts nextTraceAPIV4TokenShellOptions) error {
	name, args := nextTraceAPIV4TokenShellCommand(opts.shell)
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), util.EnvNextTraceAPIV4TokenKey+"="+opts.token)
	if opts.stdin != nil {
		cmd.Stdin = opts.stdin
	} else {
		cmd.Stdin = os.Stdin
	}
	if opts.stdout != nil {
		cmd.Stdout = opts.stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if opts.stderr != nil {
		cmd.Stderr = opts.stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func nextTraceAPIV4TokenShellCommand(shell string) (string, []string) {
	switch resolveNextTraceAPIV4SetupShell(shell) {
	case nextTraceAPIV4ShellPowerShell:
		if runtime.GOOS == "windows" {
			return "powershell.exe", nil
		}
		return "pwsh", nil
	case nextTraceAPIV4ShellCMD:
		if runtime.GOOS == "windows" {
			return "cmd.exe", nil
		}
		return "cmd", nil
	default:
		if sh := strings.TrimSpace(os.Getenv("SHELL")); sh != "" {
			return sh, nil
		}
		return "/bin/sh", nil
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
