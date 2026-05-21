package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/util"
)

const (
	apiV4ShellPOSIX      = "posix"
	apiV4ShellPowerShell = "powershell"
	apiV4ShellCMD        = "cmd"
)

type apiV4TokenSetupOptions struct {
	stdin       *os.File
	stdout      io.Writer
	stderr      io.Writer
	stdoutIsTTY bool
	shell       string
	readToken   func() (string, error)
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
		fmt.Fprintln(stderr, "stdout is a terminal; not printing a token-bearing shell command.")
		fmt.Fprintf(stderr, "Use: %s\n", apiV4SetupEvalCommand(shell))
		return nil
	}

	readToken := opts.readToken
	if readToken == nil {
		readToken = func() (string, error) {
			return readAPIV4Token(opts.stdin, stderr)
		}
	}
	token, err := readToken()
	if err != nil && err != io.EOF {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		fmt.Fprintln(stderr, "No token entered; no environment command was generated.")
		return nil
	}

	_, err = fmt.Fprint(stdout, formatAPIV4TokenAssignment(shell, token))
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

func readAPIV4Token(stdin *os.File, stderr io.Writer) (string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	fmt.Fprint(stderr, "Paste v4 API token: ")
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

func formatAPIV4TokenAssignment(shell string, token string) string {
	token = strings.TrimSpace(token)
	switch resolveAPIV4SetupShell(shell) {
	case apiV4ShellPowerShell:
		return fmt.Sprintf("$env:%s = '%s'\n", util.EnvAPIV4TokenKey, quotePowerShellSingle(token))
	case apiV4ShellCMD:
		return fmt.Sprintf("set \"%s=%s\"\r\n", util.EnvAPIV4TokenKey, quoteCMDSetValue(token))
	default:
		return fmt.Sprintf("export %s='%s'\n", util.EnvAPIV4TokenKey, quotePOSIXSingle(token))
	}
}

func quotePOSIXSingle(value string) string {
	return strings.ReplaceAll(value, `'`, `'\''`)
}

func quotePowerShellSingle(value string) string {
	return strings.ReplaceAll(value, `'`, `''`)
}

func quoteCMDSetValue(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	replacer := strings.NewReplacer(
		`^`, `^^`,
		`%`, `%%`,
		`"`, `^"`,
	)
	return replacer.Replace(value)
}
