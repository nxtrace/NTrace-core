package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/util"
)

type nextTraceAPIV4TokenSetupOptions struct {
	stdin      *os.File
	stdout     io.Writer
	stderr     io.Writer
	readToken  func() (string, error)
	writeToken func(string) (string, error)
}

func runNextTraceAPIV4TokenSetup(opts nextTraceAPIV4TokenSetupOptions) error {
	stderr := opts.stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	readToken := opts.readToken
	if readToken == nil {
		readToken = func() (string, error) {
			return readNextTraceAPIV4Token(opts.stdin, stderr)
		}
	}
	writeToken := opts.writeToken
	if writeToken == nil {
		writeToken = util.WriteNextTraceAPIV4SessionToken
	}

	printNextTraceAPIV4TokenSetupIntro(stderr)
	token, err := readToken()
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		fmt.Fprintf(stderr, "No token entered; %s was not written.\n", util.EnvNextTraceAPIV4TokenKey)
		return nil
	}

	path, err := writeToken(token)
	if err != nil {
		return fmt.Errorf("write NextTrace API v4 session token: %w", err)
	}
	fmt.Fprintf(stderr, "Saved NextTrace API v4 token for this shell session: %s\n", path)
	fmt.Fprintf(stderr, "NextTrace will load it into %s for processes started from this shell.\n", util.EnvNextTraceAPIV4TokenKey)
	return nil
}

func printNextTraceAPIV4TokenSetupIntro(stderr io.Writer) {
	fmt.Fprintf(stderr, "Open token page: GET %s\n", ipgeo.NextTraceAPIV4TokenPageURL)
	fmt.Fprintf(stderr, "This stores a session-scoped %s token in a temporary file.\n", util.EnvNextTraceAPIV4TokenKey)
	fmt.Fprintf(stderr, "Session token file: %s\n", util.NextTraceAPIV4SessionTokenPath())
	fmt.Fprintf(stderr, "Fallback token file: %s\n", util.NextTraceAPIV4LatestTokenPath())
}

func readNextTraceAPIV4Token(stdin *os.File, stderr io.Writer) (string, error) {
	if stdin == nil {
		stdin = os.Stdin
	}
	fmt.Fprint(stderr, "Paste NextTrace API v4 token: ")
	if CheckTTY(int(stdin.Fd())) {
		tokenBytes, err := term.ReadPassword(int(stdin.Fd()))
		fmt.Fprintln(stderr)
		return string(tokenBytes), err
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return line, err
}
