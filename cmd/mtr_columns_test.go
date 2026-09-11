package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/akamensky/argparse"
)

func TestMTRColumnsModeContract(t *testing.T) {
	for _, tt := range []struct {
		name       string
		modes      effectiveMTRModes
		json       bool
		standalone string
		valid      bool
	}{
		{"traditional", effectiveMTRModes{}, false, "", false},
		{"tui", effectiveMTRModes{mtr: true}, false, "", true},
		{"report", effectiveMTRModes{mtr: true, report: true}, false, "", true},
		{"wide", effectiveMTRModes{mtr: true, report: true, wide: true}, false, "", true},
		{"raw", effectiveMTRModes{mtr: true, raw: true}, false, "", false},
		{"json", effectiveMTRModes{mtr: true}, true, "", false},
		{"standalone", effectiveMTRModes{mtr: true}, false, "--mtu", false},
		{"init", effectiveMTRModes{mtr: true}, false, "--init", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveMTRColumns("received", true, tt.modes, tt.json, tt.standalone)
			if (err == nil) != tt.valid {
				t.Fatalf("%v", err)
			}
			cols, err := resolveMTRColumns("", false, tt.modes, tt.json, tt.standalone)
			if err != nil || cols != nil {
				t.Fatal("implicit selection changed")
			}
		})
	}
	p := argparse.NewParser("test", "")
	f := registerMTRFlags(p)
	if err := p.Parse([]string{"test", "--mtr-columns", "received"}); err != nil {
		t.Fatal(err)
	}
	if *f.columns != "received" || *f.mtrMode {
		t.Fatal("column flag changed mode")
	}
}

func TestMTRColumnsCLIRejectsBeforeInitialization(t *testing.T) {
	cases := [][]string{
		{"-r", "--mtr-columns", "loss,LOSS"}, {"-r", "--mtr-columns", ""},
		{"-r", "--mtr-columns", "received,"}, {"-r", "--mtr-columns", "unknown"},
		{"-r", "--raw", "--mtr-columns", "loss"}, {"-r", "--json", "--mtr-columns", "loss"},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, []string{"-r", "--init", "--mtr-columns", "loss"})
		if defaultMTR {
			cases = append(cases, []string{"--init", "--mtr-columns", "loss"})
		} else {
			cases = append(cases, []string{"-t", "--init", "--mtr-columns", "loss"})
		}
	}
	if enableTraceroute {
		cases = append(cases, []string{"--mtr-columns", "loss"}, []string{"-k", "--mtr-columns", "loss"}, []string{"--mtu", "--mtr-columns", "loss"})
	}
	if appBinName == "nexttrace" {
		cases = append(cases, []string{"--dns", "--mtr-columns", "loss"})
	}
	if enableSpeed {
		cases = append(cases, []string{"--speed", "--mtr-columns", "loss"})
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			data, _ := json.Marshal(append(args, "invalid.example"))
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			process := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMTRColumnsCLIProcess$")
			process.Env = append(os.Environ(), "NTRACE_TEST_COLUMNS_ARGS="+string(data))
			var stdout, stderr bytes.Buffer
			process.Stdout = &stdout
			process.Stderr = &stderr
			err := process.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || stdout.Len() != 0 || !strings.Contains(strings.ToLower(stderr.String()), "column") {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestMTRColumnsCLIProcess(t *testing.T) {
	raw := os.Getenv("NTRACE_TEST_COLUMNS_ARGS")
	if raw == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	os.Args = append([]string{"nexttrace"}, args...)
	Execute()
}
